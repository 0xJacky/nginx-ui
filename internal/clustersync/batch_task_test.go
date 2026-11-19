package clustersync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAcceptedBatchMustFinishBeforeDependentSite(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusInternalServerError, http.StatusNotFound} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var posts, polls atomic.Int32
			id := uuid.NewString()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/api/config_sync_batch":
					posts.Add(1)
					var payload configBatchPayload
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || !payload.Async {
						t.Error("missing asynchronous batch request")
					}
					w.WriteHeader(http.StatusAccepted)
					fmt.Fprintf(w, `{"id":%q}`, id)
				case r.Method == http.MethodGet && r.URL.Path == "/api/config_sync_batch/"+id:
					if polls.Add(1) == 1 {
						w.WriteHeader(http.StatusAccepted)
						return
					}
					w.WriteHeader(code)
					fmt.Fprint(w, `{"written":1,"skipped":1,"skipped_paths":["other.conf"],"failures":[]}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			called := false
			items := []item{
				managedConfigBatchItem("files", nil, true),
				{kind: KindSite, name: "site", push: func(context.Context, nodeRef) error {
					called = true
					return nil
				}},
			}
			summary := run(context.Background(), []nodeRef{{client: resty.New().SetBaseURL(server.URL)}}, items)
			require.EqualValues(t, 1, posts.Load())
			require.EqualValues(t, 2, polls.Load())
			require.Equal(t, code == http.StatusOK, called)
			if called {
				require.Equal(t, 0, summary.Failed)
				require.Equal(t, 1, summary.Results[0].SkippedExisting)
			} else {
				require.Equal(t, 2, summary.Failed)
			}
		})
	}
}
