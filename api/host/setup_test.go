package host

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/host/setup"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
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

func TestConnectionRejectsMissingTarget(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/host/setup/connection", bytes.NewBufferString(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")

	TestConnection(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestGetPublicKeyUsesRequestedContainerPath(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "existing_key")
	if _, err := setup.GenerateKeypair(keyPath); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet,
		"/api/host/setup/publickey?private_key_path="+url.QueryEscape(keyPath), nil)

	GetPublicKey(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response keypairResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(response.PublicKey, "ssh-ed25519 ") {
		t.Fatalf("unexpected public key: %q", response.PublicKey)
	}
}

func TestGetPublicKeyRejectsRelativePath(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet,
		"/api/host/setup/publickey?private_key_path=relative/key", nil)

	GetPublicKey(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestGenerateKeypairRejectsUnconfiguredPath(t *testing.T) {
	originalPrivateKeyPath := settings.NginxSettings.HostPrivateKeyPath
	settings.NginxSettings.HostPrivateKeyPath = ""
	t.Cleanup(func() {
		settings.NginxSettings.HostPrivateKeyPath = originalPrivateKeyPath
	})

	recorder := performHostKeyRequest(t, GenerateKeypair, keyPathRequest{
		PrivateKeyPath: filepath.Join(t.TempDir(), "unconfigured_key"),
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestGenerateKeypairAllowsConfiguredPath(t *testing.T) {
	originalPrivateKeyPath := settings.NginxSettings.HostPrivateKeyPath
	privateKeyPath := filepath.Join(t.TempDir(), "configured_key")
	settings.NginxSettings.HostPrivateKeyPath = privateKeyPath
	t.Cleanup(func() {
		settings.NginxSettings.HostPrivateKeyPath = originalPrivateKeyPath
	})

	recorder := performHostKeyRequest(t, GenerateKeypair, keyPathRequest{
		PrivateKeyPath: privateKeyPath,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if _, err := os.Stat(privateKeyPath); err != nil {
		t.Fatalf("configured private key was not created: %v", err)
	}
}

// The wizard sits under middleware.Proxy(), so a controller configuring a
// child node arrives signed as that node. Adding RequireInteractiveUser() here
// would break that flow, and the proxy rewrites 403 into an opaque 503.
func TestSetupRoutesStayReachableForProxiedNodePrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{
			CredentialID: "credential",
			AuthMethod:   model.NodeAuthMethodPaired,
		})
		c.Next()
	})
	InitRouter(router.Group("/"))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/host/setup/keypair", nil))

	if recorder.Code == http.StatusForbidden {
		t.Fatalf("DELETE /host/setup/keypair = 403, want the request to reach the handler; "+
			"a middleware is rejecting the proxied node principal (body: %s)", recorder.Body.String())
	}
}

// The wizard reads operator supplied paths and opens SSH sessions, so the
// group must stay behind a verified session for an interactive user. Without
// this, removing the middleware leaves the package's tests green.
func TestSetupRoutesRequireASecureSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		// An interactive user with 2FA enabled and no secure session header.
		c.Set("user", &model.User{
			Model:     model.Model{ID: 1},
			OTPSecret: []byte("otp-enabled"),
		})
		c.Next()
	})
	InitRouter(router.Group("/"))

	for _, tt := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/host/setup/keypair"},
		{http.MethodDelete, "/host/setup/keypair"},
		{http.MethodPost, "/host/setup/host-key/replace"},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s = %d, want %d; the secure session guard is missing",
					tt.method, tt.path, recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}
