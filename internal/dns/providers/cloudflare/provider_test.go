package cloudflare

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestProviderLifecycle(t *testing.T) {
	t.Parallel()

	type recordPayload struct {
		Type     string   `json:"type"`
		Name     string   `json:"name"`
		Content  string   `json:"content"`
		TTL      int      `json:"ttl"`
		Proxied  *bool    `json:"proxied"`
		Priority *float64 `json:"priority"`
		Comment  string   `json:"comment"`
	}

	var mu sync.Mutex
	var record *recordPayload
	zoneRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization")) {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/zones":
			mu.Lock()
			zoneRequests++
			mu.Unlock()
			assert.Equal(t, "example.com", r.URL.Query().Get("name"))
			writeCloudflareResponse(t, w, []map[string]any{{"id": "zone-1", "name": "example.com"}}, true)
		case r.Method == http.MethodPost && r.URL.Path == "/zones/zone-1/dns_records":
			var payload recordPayload
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			assert.Equal(t, "www.example.com", payload.Name)
			mu.Lock()
			record = &payload
			mu.Unlock()
			writeCloudflareResponse(t, w, cloudflareRecord("record-1", payload), false)
		case r.Method == http.MethodGet && r.URL.Path == "/zones/zone-1/dns_records":
			assert.Equal(t, "CNAME", r.URL.Query().Get("type"))
			assert.Equal(t, "www.example.com", r.URL.Query().Get("name.exact"))
			if r.URL.Query().Get("page") == "2" {
				writeCloudflareResponse(t, w, []map[string]any{}, true)
				return
			}
			mu.Lock()
			current := *record
			mu.Unlock()
			writeCloudflareResponse(t, w, []map[string]any{cloudflareRecord("record-1", current)}, true)
		case r.Method == http.MethodPut && r.URL.Path == "/zones/zone-1/dns_records/record-1":
			var payload recordPayload
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			mu.Lock()
			record = &payload
			mu.Unlock()
			writeCloudflareResponse(t, w, cloudflareRecord("record-1", payload), false)
		case r.Method == http.MethodDelete && r.URL.Path == "/zones/zone-1/dns_records/record-1":
			mu.Lock()
			record = nil
			mu.Unlock()
			writeCloudflareResponse(t, w, map[string]any{"id": "record-1"}, false)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	createdProvider, err := newProvider(&dns.Credential{
		Values: map[string]string{"CF_API_TOKEN": "test-token"},
		Additional: map[string]string{
			"CF_BASE_URL": server.URL,
		},
	})
	require.NoError(t, err)

	proxied := true
	created, err := createdProvider.CreateRecord(t.Context(), "example.com", dns.RecordInput{
		Type:    "cname",
		Name:    "www",
		Content: "origin.example.com",
		TTL:     300,
		Proxied: &proxied,
		Comment: "managed by nginx-ui",
	})
	require.NoError(t, err)
	require.Equal(t, "record-1", created.ID)
	require.Equal(t, "www", created.Name)
	require.Equal(t, "managed by nginx-ui", created.Comment)
	require.Equal(t, true, *created.Proxied)

	records, err := createdProvider.ListRecords(t.Context(), "example.com", dns.RecordFilter{
		Type: "cname",
		Name: "www",
	})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, created, records[0])

	proxied = false
	updated, err := createdProvider.UpdateRecord(t.Context(), "example.com", created.ID, dns.RecordInput{
		Type:    "CNAME",
		Name:    "www",
		Content: "new-origin.example.com",
		TTL:     600,
		Proxied: &proxied,
		Comment: "",
	})
	require.NoError(t, err)
	require.Equal(t, "new-origin.example.com", updated.Content)
	require.Empty(t, updated.Comment)
	require.False(t, *updated.Proxied)

	require.NoError(t, createdProvider.DeleteRecord(t.Context(), "example.com", created.ID))
	mu.Lock()
	require.Nil(t, record)
	require.Equal(t, 1, zoneRequests)
	mu.Unlock()
}

func TestProviderUsesLegacyAPIKeyHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Email") != "admin@example.com" {
			t.Errorf("unexpected API email header: %q", r.Header.Get("X-Auth-Email"))
		}
		if r.Header.Get("X-Auth-Key") != "test-api-key" {
			t.Errorf("unexpected API key header: %q", r.Header.Get("X-Auth-Key"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/zones":
			writeCloudflareResponse(t, w, []map[string]any{{"id": "zone-1", "name": "example.com"}}, true)
		case "/zones/zone-1/dns_records":
			writeCloudflareResponse(t, w, []map[string]any{}, true)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	createdProvider, err := newProvider(&dns.Credential{
		Values: map[string]string{
			"CF_API_EMAIL": "admin@example.com",
			"CF_API_KEY":   "test-api-key",
		},
		Additional: map[string]string{"CF_BASE_URL": server.URL},
	})
	require.NoError(t, err)

	records, err := createdProvider.ListRecords(t.Context(), "example.com", dns.RecordFilter{})
	require.NoError(t, err)
	require.Empty(t, records)
}

func cloudflareRecord(id string, payload struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Content  string   `json:"content"`
	TTL      int      `json:"ttl"`
	Proxied  *bool    `json:"proxied"`
	Priority *float64 `json:"priority"`
	Comment  string   `json:"comment"`
}) map[string]any {
	proxied := false
	if payload.Proxied != nil {
		proxied = *payload.Proxied
	}
	return map[string]any{
		"id": id, "type": strings.ToUpper(payload.Type), "name": payload.Name,
		"content": payload.Content, "ttl": payload.TTL, "proxied": proxied,
		"priority": payload.Priority, "comment": payload.Comment,
	}
}

func writeCloudflareResponse(t *testing.T, w http.ResponseWriter, result any, paginated bool) {
	t.Helper()
	response := map[string]any{
		"success":  true,
		"errors":   []any{},
		"messages": []any{},
		"result":   result,
	}
	if paginated {
		response["result_info"] = map[string]any{
			"page": 1, "per_page": 100, "count": 1, "total_count": 1, "total_pages": 1,
		}
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Errorf("encode Cloudflare response: %v", err)
	}
}
