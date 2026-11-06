package config

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalconfig "github.com/0xJacky/Nginx-UI/internal/config"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

// restrictedScriptingConfig loads njs into the Nginx worker, which is what
// the two-factor step-up exists to gate.
const restrictedScriptingConfig = "server {\n    js_import main from conf.d/main.js;\n}\n"

// newPrincipalConfigRouter runs the configuration mutations behind the real
// secure-session middleware with a principal the test injects, which is how a
// cluster node and a service token reach these endpoints.
func newPrincipalConfigRouter(prepare gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	g := r.Group("/", prepare, middleware.RequireSecureSession())
	InitRouter(g)
	return r
}

func newServiceTokenConfigRouter() *gin.Engine {
	return newPrincipalConfigRouter(func(c *gin.Context) {
		c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{
			PublicID:  "token-1",
			Name:      "ci",
			Scopes:    []string{model.APITokenScopeWrite},
			CreatorID: 2,
		})
		c.Next()
	})
}

func newNodeConfigRouter() *gin.Engine {
	return newPrincipalConfigRouter(func(c *gin.Context) {
		c.Set("user", &model.User{
			Model:     model.Model{ID: 1},
			Name:      "node-sync",
			Status:    true,
			OTPSecret: []byte("otp-enabled"),
		})
		c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{AuthMethod: model.NodeAuthMethodLegacy})
		c.Next()
	})
}

// otpSecureSessionHeaders authenticates the fixture's two-factor user with a
// verified secure session, the state an administrator is in after passing the
// 2FA prompt.
func otpSecureSessionHeaders(auth configAuthFixture) map[string]string {
	return map[string]string{
		"Authorization":       auth.otpToken,
		"X-Secure-Session-ID": internaluser.SetSecureSessionID(3),
	}
}

func assertConfigAbsent(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be absent, stat error: %v", path, err)
	}
}

// An administrator with a verified two-factor session who explicitly asks for
// the restricted directives gets the save.
func TestAddConfigAllowsRestrictedDirectivesAfterTwoFactorOptIn(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":                        "app.conf",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, otpSecureSessionHeaders(auth))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	written, err := os.ReadFile(filepath.Join(confDir, "app.conf"))
	if err != nil {
		t.Fatalf("expected the configuration to be written: %v", err)
	}
	if !strings.Contains(string(written), "js_import") {
		t.Fatalf("expected js_import to be saved, got %q", written)
	}
}

func TestEditConfigAllowsRestrictedDirectivesAfterTwoFactorOptIn(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	path := filepath.Join(confDir, "app.conf")
	if err := os.WriteFile(path, []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/config", gin.H{
		"path":                        "app.conf",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, otpSecureSessionHeaders(auth))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if !strings.Contains(string(written), "js_import") {
		t.Fatalf("expected js_import to be saved, got %q", written)
	}
}

// Without the opt-in the same administrator is asked to confirm, with the
// directives and the file the UI shows in the dialog.
func TestAddConfigAsksForConfirmationWithoutTheOptIn(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigRestrictedDirectiveConfirmRequired)

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":    "app.conf",
		"content": restrictedScriptingConfig,
	}, otpSecureSessionHeaders(auth))

	path := filepath.Join(confDir, "app.conf")
	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "js_import", path)
	assertConfigAbsent(t, path)
}

// The opt-in is only a request. An account with no second factor cannot
// confirm anything, and is told so instead of being let through.
func TestAddConfigRefusesTheOptInWithoutASecondFactor(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigRestrictedDirectiveNeedsTwoFactor)

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":                        "app.conf",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, map[string]string{
		"Authorization": auth.plainToken,
	})

	path := filepath.Join(confDir, "app.conf")
	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "js_import", path)
	assertConfigAbsent(t, path)
}

// A two-factor account whose session is not verified for this request cannot
// opt in either: RequireSecureSession stops it before the handler runs.
func TestAddConfigRefusesTheOptInWithoutAVerifiedSession(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":                        "app.conf",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, map[string]string{
		"Authorization": auth.otpToken,
	})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", recorder.Code, recorder.Body.String())
	}
	assertConfigAbsent(t, filepath.Join(confDir, "app.conf"))
}

// A write-scoped service token is exactly what the denylist protects against,
// so the opt-in does nothing for it and the plain rejection stands.
func TestServiceTokenCannotOptInToRestrictedDirectives(t *testing.T) {
	confDir, _ := setupConfigSecurityTest(t)
	router := newServiceTokenConfigRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigDirectiveNotAllowed)

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":                        "app.conf",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, nil)

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "js_import")
	assertConfigAbsent(t, filepath.Join(confDir, "app.conf"))
}

// A cluster node is refused too, which is what keeps a configuration with
// restricted directives from replicating across the cluster.
func TestClusterNodeCannotOptInToRestrictedDirectives(t *testing.T) {
	confDir, _ := setupConfigSecurityTest(t)
	router := newNodeConfigRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigDirectiveNotAllowed)

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":                        "app.conf",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, nil)

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "js_import")
	assertConfigAbsent(t, filepath.Join(confDir, "app.conf"))
}

// The public demo leaves local configuration CRUD open to visitors because a
// configuration write cannot execute code there. The hatch would change that,
// and the demo password is published, so it stays shut however the visitor
// authenticates — with the plain rejection, since there is no confirmation
// they could satisfy.
func TestDemoModeNeverUnlocksRestrictedDirectives(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigDirectiveNotAllowed)

	originalDemo := appsettings.NodeSettings.Demo
	appsettings.NodeSettings.Demo = true
	t.Cleanup(func() { appsettings.NodeSettings.Demo = originalDemo })

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":                        "app.conf",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, otpSecureSessionHeaders(auth))

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "js_import")
	assertConfigAbsent(t, filepath.Join(confDir, "app.conf"))
}

// The step-up unlocks the code an administrator deploys, not the account the
// workers run as.
func TestOptInNeverUnlocksRootWorkers(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigDirectiveNotAllowed)

	if err := os.WriteFile(filepath.Join(confDir, "nginx.conf"), []byte("user nginx;\nevents {}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/config", gin.H{
		"path":                        "nginx.conf",
		"content":                     "user root;\nevents {}\n",
		"allow_restricted_directives": true,
	}, otpSecureSessionHeaders(auth))

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "user root")

	content, err := os.ReadFile(filepath.Join(confDir, "nginx.conf"))
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if strings.Contains(string(content), "user root") {
		t.Fatalf("expected user root to be rejected, got %q", content)
	}
}

// The cluster sync endpoint applies the denylist on the receiving node and is
// not part of the step-up, so a restricted file is reported as a failure
// rather than written.
func TestConfigSyncBatchKeepsRejectingRestrictedDirectives(t *testing.T) {
	confDir, _ := setupConfigSecurityTest(t)
	router := newNodeConfigRouter()

	recorder := performJSONRequest(t, router, http.MethodPost, "/config_sync_batch", gin.H{
		"overwrite": true,
		"files": []gin.H{
			{"name": "app.conf", "base_dir": "", "content": restrictedScriptingConfig},
		},
		"allow_restricted_directives": true,
	}, nil)

	if recorder.Code == http.StatusOK && !strings.Contains(recorder.Body.String(), "restricted") {
		t.Fatalf("expected the restricted file to be reported as a failure, got %s", recorder.Body.String())
	}
	assertConfigAbsent(t, filepath.Join(confDir, "app.conf"))
}

// A rejected save must leave the configuration record alone as well, not
// only the file: the request carries sync targets, and storing those for a
// configuration that was never written would replicate it on the next
// successful save.
func TestRejectedEditConfigLeavesTheConfigRecordUntouched(t *testing.T) {
	confDir, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()

	if err := os.WriteFile(filepath.Join(confDir, "app.conf"), []byte("server {\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	recorder := performJSONRequest(t, router, http.MethodPost, "/config", gin.H{
		"path":           "app.conf",
		"content":        restrictedScriptingConfig,
		"sync_node_ids":  []uint64{7},
		"sync_overwrite": true,
	}, otpSecureSessionHeaders(auth))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected the save to be refused, got %d: %s", recorder.Code, recorder.Body.String())
	}

	q := query.Config
	if _, err := q.Where(q.Filepath.Eq(filepath.Join(confDir, "app.conf"))).First(); err == nil {
		t.Fatal("expected no configuration record for a refused save")
	}
}

// A verified administrator must still not be able to save a file the
// filename policy rejects, whatever the opt-in says.
func TestOptInDoesNotLiftTheFilenamePolicy(t *testing.T) {
	_, auth := setupConfigSecurityTest(t)
	router := newConfigMutationRouter()
	scope, code := mustCosyErrorMeta(t, internalconfig.ErrConfigFilenameNotAllowed)

	recorder := performJSONRequest(t, router, http.MethodPost, "/configs", gin.H{
		"name":                        "evil.so",
		"content":                     restrictedScriptingConfig,
		"allow_restricted_directives": true,
	}, otpSecureSessionHeaders(auth))

	assertCosyErrorResponse(t, recorder, http.StatusInternalServerError, scope, code, "evil.so")
}
