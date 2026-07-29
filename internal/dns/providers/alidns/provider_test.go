package alidns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestListRecordsMapsFieldsAndPaginates(t *testing.T) {
	t.Parallel()

	firstPage := make([]domainRecord, maxRecordsPageSize)
	for i := range firstPage {
		firstPage[i] = domainRecord{RecordID: strconv.Itoa(i + 1), Type: "A", RR: "www", Value: "192.0.2.1", TTL: 600}
	}
	weight := 20
	priority := 10
	client := &fakeAliDNSClient{handler: func(action string, query map[string]any, result any) error {
		require.Equal(t, "DescribeDomainRecords", action)
		response := describeDomainRecordsResponse{TotalCount: maxRecordsPageSize + 1}
		if query["PageNumber"] == 1 {
			response.DomainRecords.Records = firstPage
		} else {
			response.DomainRecords.Records = []domainRecord{{
				RecordID: "last", Type: "MX", RR: "mail", Value: "mail.example.com",
				TTL: 300, Line: "telecom", Weight: &weight, Priority: &priority,
			}}
		}
		return copyJSON(result, response)
	}}

	records, err := (&provider{client: client}).ListRecords(t.Context(), "example.com", dns.RecordFilter{
		Name: " mail ",
		Type: " mx ",
	})
	require.NoError(t, err)
	require.Len(t, records, maxRecordsPageSize+1)
	require.Equal(t, dns.Record{
		ID: "last", Type: "MX", Name: "mail", Content: "mail.example.com", TTL: 300,
		Line: "telecom", Weight: &weight, Priority: &priority,
	}, records[len(records)-1])
	require.Len(t, client.calls, 2)
	require.Equal(t, "mail", client.calls[0].query["RRKeyWord"])
	require.Equal(t, "MX", client.calls[0].query["TypeKeyWord"])
	require.Equal(t, 2, client.calls[1].query["PageNumber"])
}

func TestAliDNSEndpointPreservesRegionalResolution(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://alidns.cn-hangzhou.aliyuncs.com", aliDNSEndpoint("cn-hangzhou", ""))
	require.Equal(t, publicEndpoint, aliDNSEndpoint("public", ""))
	require.Equal(t, "http://127.0.0.1:8080", aliDNSEndpoint("cn-hangzhou", " http://127.0.0.1:8080 "))
}

func TestListRecordLinesMapsSupportedLines(t *testing.T) {
	t.Parallel()

	client := &fakeAliDNSClient{handler: func(_ string, _ map[string]any, result any) error {
		response := describeSupportLinesResponse{}
		response.RecordLines.Lines = append(response.RecordLines.Lines,
			supportLine{Code: "default", Name: "Default", DisplayName: "Default line"},
			supportLine{Code: "cn_telecom_zhejiang", Name: "Zhejiang Telecom", FatherCode: "telecom"},
		)
		return copyJSON(result, response)
	}}

	lines, err := (&provider{client: client}).ListRecordLines(t.Context(), "example.com")
	require.NoError(t, err)
	require.Equal(t, []dns.RecordLine{
		{Code: "default", Name: "Default", DisplayName: "Default line"},
		{Code: "cn_telecom_zhejiang", Name: "Zhejiang Telecom", FatherCode: "telecom"},
	}, lines)
	require.Equal(t, "example.com", client.calls[0].query["DomainName"])
}

func TestUpdateRecordPreservesLineWhenOmitted(t *testing.T) {
	t.Parallel()

	describeCalls := 0
	client := &fakeAliDNSClient{handler: func(action string, query map[string]any, result any) error {
		switch action {
		case "DescribeDomainRecordInfo":
			describeCalls++
			return copyJSON(result, domainRecord{
				RecordID: "record-1", Type: "A", RR: "www", Value: "192.0.2.2", TTL: 600, Line: "telecom",
			})
		case "UpdateDomainRecord":
			require.Equal(t, "telecom", query["Line"])
			return nil
		default:
			return fmt.Errorf("unexpected action %s", action)
		}
	}}

	record, err := (&provider{client: client}).UpdateRecord(t.Context(), "example.com", "record-1", dns.RecordInput{
		Type: "A", Name: "www", Content: "192.0.2.2", TTL: 600,
	})
	require.NoError(t, err)
	require.Equal(t, "telecom", record.Line)
	require.Equal(t, 2, describeCalls)
}

func TestProviderLifecycleUsesSignedOpenAPIRequests(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	record := domainRecord{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "ACS3-HMAC-SHA256 Credential=test-access-key,") {
			t.Errorf("missing ACS3 authorization: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Acs-Security-Token") != "test-session-token" {
			t.Errorf("missing security token")
		}
		if r.Header.Get("X-Acs-Version") != apiVersion {
			t.Errorf("unexpected API version: %q", r.Header.Get("X-Acs-Version"))
		}
		w.Header().Set("Content-Type", "application/json")
		action := r.Header.Get("X-Acs-Action")
		query := r.URL.Query()

		mu.Lock()
		defer mu.Unlock()
		switch action {
		case "DescribeDomainRecords":
			writeJSON(t, w, map[string]any{
				"TotalCount": 1, "PageNumber": 1, "PageSize": maxRecordsPageSize,
				"DomainRecords": map[string]any{"Record": []domainRecord{record}},
			})
		case "DescribeSupportLines":
			writeJSON(t, w, map[string]any{"RecordLines": map[string]any{"RecordLine": []map[string]any{{
				"LineCode": "default", "LineName": "Default", "LineDisplayName": "Default line",
			}}}})
		case "AddDomainRecord":
			record = domainRecord{
				RecordID: "record-1", Type: query.Get("Type"), RR: query.Get("RR"), Value: query.Get("Value"),
				TTL: queryInt(t, query.Get("TTL")), Line: query.Get("Line"),
			}
			writeJSON(t, w, recordIDResponse{RecordID: record.RecordID})
		case "DescribeDomainRecordInfo":
			writeJSON(t, w, record)
		case "UpdateDomainRecord":
			record.Type = query.Get("Type")
			record.RR = query.Get("RR")
			record.Value = query.Get("Value")
			record.TTL = queryInt(t, query.Get("TTL"))
			record.Line = query.Get("Line")
			writeJSON(t, w, map[string]any{"RecordId": record.RecordID})
		case "DeleteDomainRecord":
			record = domainRecord{}
			writeJSON(t, w, map[string]any{"RecordId": "record-1"})
		default:
			http.Error(w, "unexpected action", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	createdProvider, err := newProvider(&dns.Credential{
		Values: map[string]string{
			"ALICLOUD_ACCESS_KEY_ID":     "test-access-key",
			"ALICLOUD_ACCESS_KEY_SECRET": "test-secret-key",
			"ALICLOUD_SECURITY_TOKEN":    "test-session-token",
		},
		Additional: map[string]string{"ALICLOUD_ENDPOINT": server.URL},
	})
	require.NoError(t, err)

	line := "telecom"
	created, err := createdProvider.CreateRecord(t.Context(), "example.com", dns.RecordInput{
		Type: "a", Name: "www", Content: "192.0.2.1", TTL: 600, Line: &line,
	})
	require.NoError(t, err)
	require.Equal(t, "record-1", created.ID)
	require.Equal(t, "telecom", created.Line)

	records, err := createdProvider.ListRecords(t.Context(), "example.com", dns.RecordFilter{})
	require.NoError(t, err)
	require.Equal(t, []dns.Record{created}, records)

	updated, err := createdProvider.UpdateRecord(t.Context(), "example.com", created.ID, dns.RecordInput{
		Type: "A", Name: "www", Content: "192.0.2.2", TTL: 300,
	})
	require.NoError(t, err)
	require.Equal(t, "192.0.2.2", updated.Content)
	require.Equal(t, "telecom", updated.Line)

	require.NoError(t, createdProvider.DeleteRecord(t.Context(), "example.com", created.ID))
	mu.Lock()
	require.Empty(t, record.RecordID)
	mu.Unlock()
}

type aliCall struct {
	action string
	query  map[string]any
}

type fakeAliDNSClient struct {
	calls   []aliCall
	handler func(action string, query map[string]any, result any) error
}

func (c *fakeAliDNSClient) Call(_ context.Context, action string, query map[string]any, result any) error {
	queryCopy := make(map[string]any, len(query))
	for key, value := range query {
		queryCopy[key] = value
	}
	c.calls = append(c.calls, aliCall{action: action, query: queryCopy})
	return c.handler(action, query, result)
}

func copyJSON(target, source any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func queryInt(t *testing.T, value string) int {
	t.Helper()
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Errorf("parse query integer %q: %v", value, err)
		return 0
	}
	return parsed
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode AliDNS response: %v", err)
	}
}
