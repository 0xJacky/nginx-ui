package tencentcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestListRecordsMapsFieldsAndPaginates(t *testing.T) {
	t.Parallel()

	firstPage := make([]recordListItem, maxRecordsPageSize)
	for i := range firstPage {
		firstPage[i] = recordListItem{RecordID: uint64(i + 1), Type: "A", Name: "www", Value: "192.0.2.1", TTL: 600}
	}
	weight := uint64(20)
	mx := uint64(10)
	client := &fakeTencentDNSClient{handler: func(action string, requestData, result any) error {
		require.Equal(t, "DescribeRecordList", action)
		request := requestData.(recordListRequest)
		response := listRecordsResponse{}
		response.Response.RecordCountInfo.TotalCount = maxRecordsPageSize + 1
		if request.Offset == 0 {
			response.Response.RecordList = firstPage
		} else {
			response.Response.RecordList = []recordListItem{{
				RecordID: 4000, Type: "MX", Name: "mail", Value: "mail.example.com", TTL: 300,
				Line: "电信", Weight: &weight, MX: &mx,
			}}
		}
		return copyJSON(result, response)
	}}

	records, err := (&provider{client: client}).ListRecords(t.Context(), "example.com", dns.RecordFilter{
		Name: " mail ", Type: " mx ",
	})
	require.NoError(t, err)
	require.Len(t, records, maxRecordsPageSize+1)
	require.Equal(t, dns.Record{
		ID: "4000", Type: "MX", Name: "mail", Content: "mail.example.com", TTL: 300,
		Line: "电信", Weight: intPointer(20), Priority: intPointer(10),
	}, records[len(records)-1])
	require.Len(t, client.calls, 2)
	firstRequest := client.calls[0].request.(recordListRequest)
	require.Equal(t, "mail", firstRequest.Subdomain)
	require.Equal(t, "MX", firstRequest.RecordType)
	require.Equal(t, uint64(maxRecordsPageSize), client.calls[1].request.(recordListRequest).Offset)
}

func TestProviderLifecycleUsesSignedCommonClientRequests(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	record := recordInfo{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "TC3-HMAC-SHA256 Credential=test-secret-id/") {
			t.Errorf("missing TC3 authorization: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-TC-Token") != "test-session-token" {
			t.Errorf("missing session token")
		}
		if r.Header.Get("X-TC-Version") != tencentAPIVersion {
			t.Errorf("unexpected API version: %q", r.Header.Get("X-TC-Version"))
		}

		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		action := r.Header.Get("X-TC-Action")

		mu.Lock()
		defer mu.Unlock()
		switch action {
		case "CreateRecord":
			record = recordInfo{
				ID: 123, Subdomain: stringField(request, "SubDomain"), RecordType: stringField(request, "RecordType"),
				RecordLine: stringField(request, "RecordLine"), Value: stringField(request, "Value"),
				TTL: uint64Field(request, "TTL"), Weight: optionalUint64Field(request, "Weight"), MX: optionalUint64Field(request, "MX"),
			}
			writeTencentResponse(t, w, map[string]any{"RecordId": record.ID})
		case "DescribeRecord":
			writeTencentResponse(t, w, map[string]any{"RecordInfo": record})
		case "DescribeRecordList":
			item := recordListItem{
				RecordID: record.ID, Type: record.RecordType, Name: record.Subdomain, Value: record.Value,
				TTL: record.TTL, Line: record.RecordLine, Weight: record.Weight, MX: record.MX,
			}
			writeTencentResponse(t, w, map[string]any{
				"RecordCountInfo": map[string]any{"TotalCount": 1}, "RecordList": []recordListItem{item},
			})
		case "ModifyRecord":
			record.Subdomain = stringField(request, "SubDomain")
			record.RecordType = stringField(request, "RecordType")
			record.RecordLine = stringField(request, "RecordLine")
			record.Value = stringField(request, "Value")
			record.TTL = uint64Field(request, "TTL")
			record.Weight = optionalUint64Field(request, "Weight")
			record.MX = optionalUint64Field(request, "MX")
			writeTencentResponse(t, w, map[string]any{})
		case "DeleteRecord":
			record = recordInfo{}
			writeTencentResponse(t, w, map[string]any{})
		default:
			http.Error(w, "unexpected action", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	createdProvider, err := newProvider(&dns.Credential{
		Values: map[string]string{
			"TENCENTCLOUD_SECRET_ID":     "test-secret-id",
			"TENCENTCLOUD_SECRET_KEY":    "test-secret-key",
			"TENCENTCLOUD_SESSION_TOKEN": "test-session-token",
		},
		Additional: map[string]string{"TENCENTCLOUD_ENDPOINT": server.URL},
	})
	require.NoError(t, err)

	line := "电信"
	weight := 20
	priority := 10
	created, err := createdProvider.CreateRecord(t.Context(), "example.com", dns.RecordInput{
		Type: "mx", Name: "mail", Content: "mail.example.com", TTL: 600,
		Line: &line, Weight: &weight, Priority: &priority,
	})
	require.NoError(t, err)
	require.Equal(t, dns.Record{
		ID: "123", Type: "MX", Name: "mail", Content: "mail.example.com", TTL: 600,
		Line: line, Weight: &weight, Priority: &priority,
	}, created)

	records, err := createdProvider.ListRecords(t.Context(), "example.com", dns.RecordFilter{Type: "MX", Name: "mail"})
	require.NoError(t, err)
	require.Equal(t, []dns.Record{created}, records)

	updated, err := createdProvider.UpdateRecord(t.Context(), "example.com", created.ID, dns.RecordInput{
		Type: "MX", Name: "mail", Content: "new-mail.example.com", TTL: 300,
		Line: &line, Weight: &weight, Priority: &priority,
	})
	require.NoError(t, err)
	require.Equal(t, "new-mail.example.com", updated.Content)

	require.NoError(t, createdProvider.DeleteRecord(t.Context(), "example.com", created.ID))
	mu.Lock()
	require.Zero(t, record.ID)
	mu.Unlock()
}

type tencentCall struct {
	action  string
	request any
}

type fakeTencentDNSClient struct {
	calls   []tencentCall
	handler func(action string, request, response any) error
}

func (c *fakeTencentDNSClient) Call(_ context.Context, action string, request, response any) error {
	c.calls = append(c.calls, tencentCall{action: action, request: request})
	return c.handler(action, request, response)
}

func copyJSON(target, source any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func stringField(fields map[string]any, key string) string {
	return fmt.Sprint(fields[key])
}

func uint64Field(fields map[string]any, key string) uint64 {
	value, _ := fields[key].(float64)
	return uint64(value)
}

func optionalUint64Field(fields map[string]any, key string) *uint64 {
	if _, ok := fields[key]; !ok {
		return nil
	}
	value := uint64Field(fields, key)
	return &value
}

func writeTencentResponse(t *testing.T, w http.ResponseWriter, response any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"Response": response}); err != nil {
		t.Errorf("encode Tencent response: %v", err)
	}
}

func intPointer(value int) *int {
	return &value
}
