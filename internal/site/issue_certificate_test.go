package site

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issueCertificateStub replaces the issuance and node sync of IssueCertificate.
type issueCertificateStub struct {
	mu      sync.Mutex
	calls   []string
	synced  []uint64
	result  *model.Cert
	err     error
	gate    chan struct{}
	started chan string
}

func stubIssueCertificate(t *testing.T) *issueCertificateStub {
	t.Helper()
	stub := &issueCertificateStub{result: &model.Cert{Model: model.Model{ID: 42}}, started: make(chan string, 8)}
	origIssue, origSync, origLog := issueWithRecord, syncIssuedCertificate, newIssueCertificateLog
	issueWithRecord = func(name string, payload *cert.ConfigPayload, log *cert.Logger) (*model.Cert, error) {
		if log == nil {
			t.Error("issueWithRecord got no logger")
		}
		stub.mu.Lock()
		stub.calls = append(stub.calls, name)
		gate := stub.gate
		stub.mu.Unlock()
		stub.started <- name
		if gate != nil {
			<-gate
		}
		return stub.result, stub.err
	}
	syncIssuedCertificate = func(id uint64) {
		stub.mu.Lock()
		defer stub.mu.Unlock()
		stub.synced = append(stub.synced, id)
	}
	newIssueCertificateLog = cert.NewLogger
	t.Cleanup(func() {
		issueWithRecord, syncIssuedCertificate, newIssueCertificateLog = origIssue, origSync, origLog
	})
	return stub
}

func TestIssueCertificateNormalizesAndSyncs(t *testing.T) {
	stub := stubIssueCertificate(t)
	payload := &cert.ConfigPayload{
		ServerName: []string{" example.com ", "example.com", "www.example.com"},
		KeyType:    "P256",
	}

	got, err := IssueCertificate(context.Background(), "example.com", payload, nil)
	require.NoError(t, err)
	assert.Same(t, stub.result, got)
	assert.Equal(t, []string{"example.com"}, stub.calls)
	assert.Equal(t, []uint64{42}, stub.synced)
	assert.Equal(t, []string{"example.com", "www.example.com"}, payload.ServerName)
	assert.Equal(t, cert.HTTP01, payload.ChallengeMethod)
	assert.Equal(t, certcrypto.EC256, payload.KeyType)
	assert.Empty(t, payload.ConfigName, "the route check stays opt-in")
}

func TestIssueCertificateRejectsInvalidIdentifiersWithoutRecord(t *testing.T) {
	stub := stubIssueCertificate(t)
	payload := &cert.ConfigPayload{ServerName: []string{"192.0.2.1"}, ChallengeMethod: cert.DNS01}

	got, err := IssueCertificate(context.Background(), "ip.conf", payload, nil)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Empty(t, stub.calls)
	assert.Empty(t, stub.synced)
}

func TestIssueCertificateReturnsRecordWithFailure(t *testing.T) {
	stub := stubIssueCertificate(t)
	stub.err = errors.New("acme: unauthorized")

	got, err := IssueCertificate(context.Background(), "example.com",
		&cert.ConfigPayload{ServerName: []string{"example.com"}}, cert.NewLogger())
	require.ErrorIs(t, err, stub.err)
	assert.Same(t, stub.result, got)
	assert.Empty(t, stub.synced, "a failed issuance is not synced")
}

func TestIssueCertificateHonorsCanceledContext(t *testing.T) {
	stub := stubIssueCertificate(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := IssueCertificate(ctx, "example.com", &cert.ConfigPayload{ServerName: []string{"example.com"}}, nil)
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, got)
	assert.Empty(t, stub.calls)
}

func TestIssueCertificateSerializesSameName(t *testing.T) {
	stub := stubIssueCertificate(t)
	gate := make(chan struct{})
	stub.gate = gate

	issue := func(name string) <-chan error {
		done := make(chan error, 1)
		go func() {
			_, err := IssueCertificate(context.Background(), name,
				&cert.ConfigPayload{ServerName: []string{name}}, nil)
			done <- err
		}()
		return done
	}

	first := issue("a.example.com")
	require.Equal(t, "a.example.com", <-stub.started)

	// Another name is not held up by the per-name lock.
	other := issue("b.example.com")
	require.Equal(t, "b.example.com", <-stub.started)

	// The same name waits for the first run.
	second := issue("a.example.com")
	select {
	case name := <-stub.started:
		t.Fatalf("second issuance of %s started while the first one was running", name)
	case <-time.After(100 * time.Millisecond):
	}

	// A waiting run gives up when its context ends.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := IssueCertificate(ctx, "a.example.com", &cert.ConfigPayload{ServerName: []string{"a.example.com"}}, nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	close(gate)
	require.NoError(t, <-first)
	require.NoError(t, <-other)
	require.Equal(t, "a.example.com", <-stub.started)
	require.NoError(t, <-second)
	assert.Len(t, stub.calls, 3)
}
