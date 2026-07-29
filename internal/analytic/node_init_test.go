package analytic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

func TestInitNodeAuthenticatesPreV250Node(t *testing.T) {
	const secret = "legacy-node-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Node-Secret") != secret {
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		if err := json.NewEncoder(writer).Encode(NodeInfo{Version: "2.4.3"}); err != nil {
			t.Errorf("encode node info: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	node := &model.Node{
		Model: model.Model{ID: 47},
		URL:   server.URL,
	}
	setupLegacyNodeAuthForTest(t, node, secret)

	remote, err := InitNode(context.Background(), node)
	require.NoError(t, err)
	require.Equal(t, "2.4.3", remote.Version)
}

func TestInitNodeStopsWhenContextIsCancelled(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	node := &model.Node{
		Model: model.Model{ID: 46},
		URL:   server.URL,
	}
	setupLegacyNodeAuthForTest(t, node, "test-secret")
	result := make(chan error, 1)
	go func() {
		_, err := InitNode(ctx, node)
		result <- err
	}()

	select {
	case <-requestStarted:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("node metadata request did not start")
	}

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected cancellation to stop node metadata request")
		}
	case <-time.After(time.Second):
		t.Fatal("node metadata request ignored context cancellation")
	}
}
