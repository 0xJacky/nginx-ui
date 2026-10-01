package clustersync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/go-resty/resty/v2"
)

func TestCollectConfigFilesSkipsTheReservedSnippetDirectory(t *testing.T) {
	confDir := withConfDir(t, map[string]string{
		"snippets/gzip.conf":            "gzip on;\n",
		"snippets/plugins/x/limit.conf": "limit_req zone=x;\n",
	})

	files, err := CollectConfigFiles(confDir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got := collectedPaths(files); !slices.Equal(got, []string{"snippets/gzip.conf"}) {
		t.Fatalf("snippets of other software must stay on their node, got %v", got)
	}
}

func TestIncludedSnippetsAreCreatedButNeverOverwritten(t *testing.T) {
	withConfDir(t, map[string]string{
		"snippets/cache.conf":   "expires 7d;\n",
		"snippets/headers.conf": "add_header X-Frame-Options DENY;\n",
	})

	names := includedSnippets([]ConfigFile{
		{Name: "a", Content: "server {\n    include snippets/cache.conf;\n}\n"},
		{Name: "b", Content: "server {\n    include snippets/cache.conf;\n    include snippets/headers.conf;\n    include snippets/missing.conf;\n}\n"},
	})
	if !slices.Equal(names, []string{"cache.conf", "headers.conf", "missing.conf"}) {
		t.Fatalf("included snippets = %v", names)
	}

	snippetsItem, ok := snippetItem(names)
	if !ok || !snippetsItem.blocking {
		t.Fatalf("expected a blocking snippet item, got %+v", snippetsItem)
	}

	var mutex sync.Mutex
	var received configBatchPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		if r.URL.Path == "/api/config_sync_batch" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode batch: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"written":2,"failures":[]}`))
	}))
	defer server.Close()

	client := resty.New()
	client.SetBaseURL(server.URL)
	summary := run(context.Background(), []nodeRef{{id: 1, name: "remote", client: client}}, []item{snippetsItem})
	if summary.Failed != 0 {
		t.Fatalf("sync failed: %+v", summary)
	}

	mutex.Lock()
	defer mutex.Unlock()
	if received.Overwrite {
		t.Fatal("a snippet that follows a site must not replace the copy of the node")
	}
	if got := collectedPaths(received.Files); !slices.Equal(got, []string{"snippets/cache.conf", "snippets/headers.conf"}) {
		t.Fatalf("pushed snippets = %v", got)
	}

	if _, ok := snippetItem([]string{"missing.conf"}); ok {
		t.Fatal("no readable snippet means no item")
	}
}
