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
	"github.com/0xJacky/Nginx-UI/internal/middleware"
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
			name:    "trust first scanned key",
			handler: TrustScannedHostKey,
			body: hostKeyTrustRequest{
				HostAddress: hostAddress,
				Algorithm:   oldKey.Type(),
				Fingerprint: oldFingerprint,
				PublicKey:   oldPublicKey,
				Confirmed:   true,
			},
			wantStatus: http.StatusOK,
			wantResets: 1,
		},
		{
			name:    "reject trust fingerprint mismatch",
			handler: TrustScannedHostKey,
			body: hostKeyTrustRequest{
				HostAddress: hostAddress,
				Algorithm:   oldKey.Type(),
				Fingerprint: newFingerprint,
				PublicKey:   oldPublicKey,
				Confirmed:   true,
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
			name:    "reject algorithm mismatch",
			handler: TrustScannedHostKey,
			body: hostKeyTrustRequest{
				HostAddress: hostAddress,
				Algorithm:   "ssh-rsa",
				Fingerprint: newFingerprint,
				PublicKey:   newPublicKey,
				Confirmed:   true,
			},
			wantStatus: http.StatusBadRequest,
			wantResets: 0,
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

func performGetPublicKey(t *testing.T, keyPath string, verified bool) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet,
		"/api/host/setup/publickey?private_key_path="+url.QueryEscape(keyPath), nil)
	if verified {
		context.Set(middleware.SecureSessionVerifiedKey, true)
	}
	GetPublicKey(context)
	return recorder
}

// The wizard's "existing key" flow reads a path the operator typed but has not
// saved yet. That must keep working, but only for a verified session: without
// it the endpoint would be a public key reader and file-existence oracle for
// every path inside the container.
func TestGetPublicKeyRequiresVerifiedSessionForUnmanagedPath(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "existing_key")
	if _, err := setup.GenerateKeypair(keyPath); err != nil {
		t.Fatal(err)
	}

	recorder := performGetPublicKey(t, keyPath, false)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unverified status = %d, want %d: %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}

	recorder = performGetPublicKey(t, keyPath, true)
	if recorder.Code != http.StatusOK {
		t.Fatalf("verified status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response keypairResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(response.PublicKey, "ssh-ed25519 ") {
		t.Fatalf("unexpected public key: %q", response.PublicKey)
	}
}

func TestGetPublicKeyReadsConfiguredPathWithoutVerification(t *testing.T) {
	originalPrivateKeyPath := settings.NginxSettings.HostPrivateKeyPath
	keyPath := filepath.Join(t.TempDir(), "configured_key")
	settings.NginxSettings.HostPrivateKeyPath = keyPath
	t.Cleanup(func() {
		settings.NginxSettings.HostPrivateKeyPath = originalPrivateKeyPath
	})
	if _, err := setup.GenerateKeypair(keyPath); err != nil {
		t.Fatal(err)
	}

	recorder := performGetPublicKey(t, keyPath, false)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
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
	if !strings.Contains(recorder.Body.String(), setup.ErrPrivateKeyPathNotAbsolute.Error()) {
		t.Fatalf("body = %s, want the scoped private key path error", recorder.Body.String())
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

// known_hosts treats "," as a host separator, "*" as a wildcard and a newline
// as the start of another entry, so an unvalidated host address could trust a
// key for every host. The handlers must refuse it before touching the file.
func TestHostKeyEndpointsRejectUnsafeHostAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalPath := settings.NginxSettings.HostKnownHostsPath
	originalReset := resetSSHClient
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	settings.NginxSettings.HostKnownHostsPath = knownHostsPath
	resetCount := 0
	resetSSHClient = func() { resetCount++ }
	t.Cleanup(func() {
		settings.NginxSettings.HostKnownHostsPath = originalPath
		resetSSHClient = originalReset
	})

	key := generatePublicHostKey(t)
	publicKey := string(gossh.MarshalAuthorizedKey(key))
	fingerprint := gossh.FingerprintSHA256(key)

	for _, address := range []string{"*", "a,*", "example.com:22 extra", "example.com\n*", " ", "example.com:22|1|x"} {
		handlers := []struct {
			name    string
			handler gin.HandlerFunc
			body    any
		}{
			{"scan", ScanHostKey, hostKeyScanRequest{HostAddress: address, KeyscanOutput: address + " " + publicKey}},
			{"trust", TrustScannedHostKey, hostKeyTrustRequest{HostAddress: address, Algorithm: key.Type(), Fingerprint: fingerprint, PublicKey: publicKey, Confirmed: true}},
			{"replace", ReplaceHostKey, hostKeyReplaceRequest{HostAddress: address, Algorithm: key.Type(), OldFingerprint: fingerprint, NewFingerprint: fingerprint, PublicKey: publicKey, Confirmed: true}},
			{"delete", DeleteHostKey, hostKeyDeleteRequest{HostAddress: address, Algorithm: key.Type(), Fingerprint: fingerprint, Confirmed: true}},
		}
		for _, tt := range handlers {
			t.Run(tt.name+" "+strings.NewReplacer("\n", "\\n").Replace(address), func(t *testing.T) {
				recorder := performHostKeyRequest(t, tt.handler, tt.body)
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
				}
			})
		}
	}
	if resetCount != 0 {
		t.Fatalf("resetSSHClient called %d times for rejected addresses", resetCount)
	}
	if _, err := os.Stat(knownHostsPath); !os.IsNotExist(err) {
		t.Fatalf("known_hosts was created for a rejected address: %v", err)
	}
}

func setDemoMode(t *testing.T, enabled bool) {
	t.Helper()
	previous := settings.NodeSettings.Demo
	settings.NodeSettings.Demo = enabled
	t.Cleanup(func() {
		settings.NodeSettings.Demo = previous
	})
}

// newInteractiveUserRouter serves the setup routes as a logged-in user with no
// second factor, which is the weakest session that reaches the group.
func newInteractiveUserRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", &model.User{Model: model.Model{ID: 1}})
		c.Next()
	})
	InitRouter(router.Group("/"))
	return router
}

// The public demo accepts any login, so the wizard must not let a demo user
// mint or delete SSH keys, append to known_hosts, or probe and connect to
// arbitrary addresses from the demo container.
func TestSetupRoutesAreRejectedInDemoMode(t *testing.T) {
	setDemoMode(t, true)
	router := newInteractiveUserRouter()

	for _, tt := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/host/setup/preview"},
		{http.MethodGet, "/host/setup/publickey"},
		{http.MethodPost, "/host/setup/keypair"},
		{http.MethodDelete, "/host/setup/keypair"},
		{http.MethodGet, "/host/setup/ssh-targets"},
		{http.MethodPost, "/host/setup/connection"},
		{http.MethodPost, "/host/setup/host-key/scan"},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))
			if recorder.Code == http.StatusOK || recorder.Code == http.StatusNoContent || recorder.Code == http.StatusNotFound {
				t.Fatalf("%s %s = %d in demo mode, want the request to be rejected (body: %s)",
					tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "demo") {
				t.Fatalf("%s %s did not fail with the demo guard: %s", tt.method, tt.path, recorder.Body.String())
			}
		})
	}
}

// Everything that stores SSH material or opens an outbound connection shares
// the verified two-factor requirement of POST settings/nginx/control, where
// the wizard result is finally saved. A user without a second factor passes
// RequireSecureSession, so the group needs its own check.
func TestMutatingSetupRoutesRequireVerifiedTwoFactor(t *testing.T) {
	setDemoMode(t, false)
	router := newInteractiveUserRouter()

	for _, tt := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/host/setup/keypair"},
		{http.MethodDelete, "/host/setup/keypair"},
		{http.MethodGet, "/host/setup/ssh-targets"},
		{http.MethodPost, "/host/setup/connection"},
		{http.MethodPost, "/host/setup/discover"},
		{http.MethodPost, "/host/setup/diagnose"},
		{http.MethodPost, "/host/setup/verify"},
		{http.MethodPost, "/host/setup/host-key/scan"},
		{http.MethodPost, "/host/setup/host-key/trust"},
		{http.MethodPost, "/host/setup/host-key/replace"},
		{http.MethodDelete, "/host/setup/host-key"},
	} {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s = %d, want %d; the verified two-factor guard is missing (body: %s)",
					tt.method, tt.path, recorder.Code, http.StatusUnauthorized, recorder.Body.String())
			}
		})
	}

	// The read-only preview keeps the weaker guard so the wizard can render
	// snippets before the operator has completed a second factor.
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/host/setup/preview", nil))
	if recorder.Code == http.StatusUnauthorized {
		t.Fatalf("GET /host/setup/preview = 401, want the read-only preview to stay reachable: %s", recorder.Body.String())
	}
}
