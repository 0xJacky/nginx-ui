package analytic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestInitNodeStopsWhenContextIsCancelled(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := InitNode(ctx, &model.Node{URL: server.URL})
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
