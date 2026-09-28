package nodeauth

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

type recordingRoundTripper struct {
	bodies []string
}

func (recorder *recordingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	body := ""
	if request.Body != nil {
		content, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		_ = request.Body.Close()
		body = string(content)
	}
	recorder.bodies = append(recorder.bodies, body)
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: request}, nil
}

func newGuardedTestTransport(limit int) (*replicationTransport, *recordingRoundTripper) {
	recorder := &recordingRoundTripper{}
	return &replicationTransport{next: recorder, nodeID: 7, guard: newLoopGuard(time.Minute, limit)}, recorder
}

func sendTestRequest(t *testing.T, transport http.RoundTripper, method, body string) error {
	t.Helper()
	request, err := http.NewRequest(method, "http://node.example/api/sites/loop.example.com", strings.NewReader(body))
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	return err
}

func TestLoopGuardDropsRepeatedIdenticalWrites(t *testing.T) {
	transport, recorder := newGuardedTestTransport(3)

	for range 3 {
		require.NoError(t, sendTestRequest(t, transport, http.MethodPost, `{"content":"a"}`))
	}
	err := sendTestRequest(t, transport, http.MethodPost, `{"content":"a"}`)
	require.True(t, errors.Is(err, ErrReplicationLoop), "got %v", err)
	require.Len(t, recorder.bodies, 3)
}

func TestLoopGuardSeparatesDistinctWrites(t *testing.T) {
	transport, recorder := newGuardedTestTransport(1)

	require.NoError(t, sendTestRequest(t, transport, http.MethodPost, `{"content":"a"}`))
	require.NoError(t, sendTestRequest(t, transport, http.MethodPost, `{"content":"b"}`))
	require.NoError(t, sendTestRequest(t, transport, http.MethodPut, `{"content":"a"}`))

	other := &replicationTransport{next: recorder, nodeID: 8, guard: transport.guard}
	require.NoError(t, sendTestRequest(t, other, http.MethodPost, `{"content":"a"}`))
	require.Len(t, recorder.bodies, 4)
}

func TestLoopGuardIgnoresReads(t *testing.T) {
	transport, recorder := newGuardedTestTransport(1)

	for range 5 {
		require.NoError(t, sendTestRequest(t, transport, http.MethodGet, ""))
	}
	require.Len(t, recorder.bodies, 5)
}

func TestLoopGuardKeepsBodyForNextTransport(t *testing.T) {
	transport, recorder := newGuardedTestTransport(3)

	// A reader without GetBody is staged before hashing.
	request, err := http.NewRequest(http.MethodPost, "http://node.example/api/configs", io.NopCloser(bytes.NewBufferString("payload")))
	require.NoError(t, err)
	require.Nil(t, request.GetBody)
	_, err = transport.RoundTrip(request)
	require.NoError(t, err)

	require.NoError(t, sendTestRequest(t, transport, http.MethodPost, "resty payload"))
	require.Equal(t, []string{"payload", "resty payload"}, recorder.bodies)
}

func TestLoopGuardReleasesKeyAfterWindow(t *testing.T) {
	guard := newLoopGuard(time.Minute, 2)
	start := time.Now()

	require.True(t, guard.allow("key", start))
	require.True(t, guard.allow("key", start.Add(time.Second)))
	require.False(t, guard.allow("key", start.Add(2*time.Second)))
	// Rejected sends are not recorded, so the key opens again one window after
	// the last accepted send.
	require.True(t, guard.allow("key", start.Add(time.Minute+2*time.Second)))
}

func TestLoopGuardSweepsStaleKeys(t *testing.T) {
	guard := newLoopGuard(time.Minute, 1)
	start := time.Now()

	for index := range loopGuardSweepSize + 1 {
		require.True(t, guard.allow(string(rune('a'+index%26))+strings.Repeat("x", index), start))
	}
	require.True(t, guard.allow("fresh", start.Add(2*time.Minute)))
	require.Len(t, guard.history, 1)
}

// Two nodes that sync to each other resend the same save on every lap; the
// clients used for node synchronization must stop that, while the user-facing
// proxy transport keeps forwarding repeated clicks.
func TestReplicationClientsStopRepeatedWrites(t *testing.T) {
	database, _, _ := setupSignatureTest(t)
	require.NoError(t, database.AutoMigrate(&model.Node{}))

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		received.Add(1)
		response.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	encryptedSecret, err := EncryptPrivateCredential(LegacyCredentialPurpose(1), []byte("loop-guard-secret"))
	require.NoError(t, err)
	node := &model.Node{
		Model:                 model.Model{ID: 1},
		URL:                   server.URL,
		Enabled:               true,
		AuthMethod:            model.NodeAuthMethodLegacy,
		EncryptedLegacySecret: encryptedSecret,
	}
	require.NoError(t, database.Create(node).Error)

	client := NewRestyClient(node)
	client.SetBaseURL(node.URL)
	body := map[string]any{"content": "server {}", "overwrite": true}

	var loopErr error
	for range loopGuardThreshold + 3 {
		if _, err := client.R().SetBody(body).Post("/api/sites/loop.example.com"); err != nil {
			loopErr = err
		}
	}
	require.ErrorIs(t, loopErr, ErrReplicationLoop)
	require.EqualValues(t, loopGuardThreshold, received.Load())

	proxyClient := &http.Client{Transport: NewTransport(node, http.DefaultTransport)}
	for range 2 {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/api/sites/loop.example.com", strings.NewReader(`{"content":"server {}"}`))
		require.NoError(t, err)
		response, err := proxyClient.Do(request)
		require.NoError(t, err)
		_ = response.Body.Close()
	}
	require.EqualValues(t, loopGuardThreshold+2, received.Load())
}
