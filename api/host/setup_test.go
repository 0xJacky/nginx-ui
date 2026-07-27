package host

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	gossh "golang.org/x/crypto/ssh"
)

func generatePublicHostKey(t *testing.T) gossh.PublicKey {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := gossh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func performHostKeyRequest(t *testing.T, handler gin.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/host/host-key", bytes.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	handler(context)
	return recorder
}

func TestHostKeyChangesResetSSHClientOnlyAfterSuccessfulWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalPath := settings.NginxSettings.HostKnownHostsPath
	originalReset := resetSSHClient
	settings.NginxSettings.HostKnownHostsPath = filepath.Join(t.TempDir(), "known_hosts")
	resetCount := 0
	resetSSHClient = func() { resetCount++ }
	t.Cleanup(func() {
		settings.NginxSettings.HostKnownHostsPath = originalPath
		resetSSHClient = originalReset
	})

	hostAddress := "example.com:22"
	oldKey := generatePublicHostKey(t)
	newKey := generatePublicHostKey(t)
	oldPublicKey := string(gossh.MarshalAuthorizedKey(oldKey))
	newPublicKey := string(gossh.MarshalAuthorizedKey(newKey))
	oldFingerprint := gossh.FingerprintSHA256(oldKey)
	newFingerprint := gossh.FingerprintSHA256(newKey)

	tests := []struct {
		name       string
		handler    gin.HandlerFunc
		body       any
		wantStatus int
		wantResets int
	}{
		{
			name:    "trust legacy endpoint",
			handler: TrustHostKey,
			body: knownHostRequest{
				HostAddress: hostAddress,
				Fingerprint: oldFingerprint,
				PublicKey:   oldPublicKey,
			},
			wantStatus: http.StatusOK,
			wantResets: 1,
		},
		{
			name:    "reject trust fingerprint mismatch",
			handler: TrustHostKey,
			body: knownHostRequest{
				HostAddress: hostAddress,
				Fingerprint: newFingerprint,
				PublicKey:   oldPublicKey,
			},
			wantStatus: http.StatusInternalServerError,
			wantResets: 0,
		},
		{
			name:    "trust scanned key",
			handler: TrustScannedHostKey,
			body: hostKeyTrustRequest{
				HostAddress: hostAddress,
				Algorithm:   newKey.Type(),
				Fingerprint: newFingerprint,
				PublicKey:   newPublicKey,
				Confirmed:   true,
			},
			wantStatus: http.StatusOK,
			wantResets: 1,
		},
		{
			name:    "reject unconfirmed scanned key",
			handler: TrustScannedHostKey,
			body: hostKeyTrustRequest{
				HostAddress: hostAddress,
				Algorithm:   newKey.Type(),
				Fingerprint: newFingerprint,
				PublicKey:   newPublicKey,
			},
			wantStatus: http.StatusInternalServerError,
			wantResets: 0,
		},
		{
			name:    "replace key",
			handler: ReplaceHostKey,
			body: hostKeyReplaceRequest{
				HostAddress:    hostAddress,
				Algorithm:      oldKey.Type(),
				OldFingerprint: oldFingerprint,
				NewFingerprint: newFingerprint,
				PublicKey:      newPublicKey,
				Confirmed:      true,
			},
			wantStatus: http.StatusOK,
			wantResets: 1,
		},
		{
			name:    "reject replacement for missing key",
			handler: ReplaceHostKey,
			body: hostKeyReplaceRequest{
				HostAddress:    hostAddress,
				Algorithm:      newKey.Type(),
				OldFingerprint: oldFingerprint,
				NewFingerprint: newFingerprint,
				PublicKey:      newPublicKey,
				Confirmed:      true,
			},
			wantStatus: http.StatusInternalServerError,
			wantResets: 0,
		},
		{
			name:    "delete key",
			handler: DeleteHostKey,
			body: hostKeyDeleteRequest{
				HostAddress: hostAddress,
				Algorithm:   newKey.Type(),
				Fingerprint: newFingerprint,
				Confirmed:   true,
			},
			wantStatus: http.StatusOK,
			wantResets: 1,
		},
		{
			name:    "reject deletion for missing key",
			handler: DeleteHostKey,
			body: hostKeyDeleteRequest{
				HostAddress: hostAddress,
				Algorithm:   newKey.Type(),
				Fingerprint: newFingerprint,
				Confirmed:   true,
			},
			wantStatus: http.StatusInternalServerError,
			wantResets: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetCount = 0
			recorder := performHostKeyRequest(t, test.handler, test.body)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if resetCount != test.wantResets {
				t.Fatalf("reset count = %d, want %d", resetCount, test.wantResets)
			}
		})
	}
}
