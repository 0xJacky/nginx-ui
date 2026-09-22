package upstream_discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/config"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/upstream/discovery"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
)

const apiTestKind = "plugin:api-registry"

// apiRegistry serves apiTestKind and resolves every service to one server.
type apiRegistry struct{}

func (apiRegistry) Providers() []discovery.Provider {
	return []discovery.Provider{{Kind: apiTestKind, Name: "API registry", PluginID: "io.github.example.registry",
		Fields: []discovery.ProviderField{
			{Key: "address", DisplayName: "Address", Required: true},
			{Key: "token", DisplayName: "Token", Secret: true},
		}}}
}

func (apiRegistry) Resolve(_ context.Context, _ string, config map[string]string, service string) (discovery.Result, error) {
	if config["address"] == "down" {
		return discovery.Result{}, errors.New("registry is down")
	}
	return discovery.Result{Targets: []discovery.Target{{Address: "10.0.0.5", Port: 8080, Weight: 2}}}, nil
}

var registerAPIRegistry = sync.OnceFunc(func() { discovery.RegisterSource(apiRegistry{}) })

// stubNginx stands in for nginx -t and the reload.
type stubNginx struct {
	mu       sync.Mutex
	failTest bool
}

func (s *stubNginx) writer() config.GeneratedWriter {
	return config.GeneratedWriter{
		Test: func() error {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.failTest {
				return errors.New(`no upstream "api"`)
			}
			return nil
		},
		Reload: func() error { return nil },
	}
}

func setup(t *testing.T) (*stubNginx, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	registerAPIRegistry()
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.UpstreamDiscovery{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "discovery.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)

	confDir := t.TempDir()
	stub := &stubNginx{}
	previous := discovery.Default()
	discovery.SetDefault(discovery.NewRunner(context.Background(),
		discovery.WithWriter(stub.writer()),
		discovery.WithConfDir(func() string { return confDir }),
	))
	t.Cleanup(func() { discovery.SetDefault(previous) })
	return stub, confDir
}

func newRouter(serviceToken *internalmcp.ServiceTokenPrincipal, user *model.User) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if serviceToken != nil {
			c.Set(internalmcp.ServiceTokenPrincipalKey, serviceToken)
		}
		if user != nil {
			c.Set("user", user)
		}
		c.Next()
	})
	InitRouter(router.Group("/"))
	return router
}

func request(router http.Handler, method, path, body, session string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		req.Header.Set("X-Secure-Session-ID", session)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func binding(name, extra string) string {
	return `{"upstream_name":"` + name + `","kind":"` + apiTestKind + `","config":{"address":"https://registry","token":"secret-token"},` +
		`"service":"api","extra_directives":"` + extra + `","enabled":true}`
}

func TestBindingsAreValidated(t *testing.T) {
	setup(t)
	admin := &model.User{Model: model.Model{ID: 401}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	session := internaluser.SetSecureSessionID(admin.ID)
	router := newRouter(nil, admin)

	recorder := request(router, http.MethodGet, "/upstream_discoveries/kinds", "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), apiTestKind)

	for body, code := range map[string]string{
		binding("bad name", ""): `"code":55210`,
		binding("../api", ""):   `"code":55210`,
		binding("api", "} x {"): `"code":55212`,
		`{"upstream_name":"api","kind":"plugin:gone","config":{"address":"a"},"service":"api"}`:                             `"code":55207`,
		`{"upstream_name":"api","kind":"` + apiTestKind + `","config":{},"service":"api"}`:                                  `"code":55208`,
		`{"upstream_name":"api","kind":"` + apiTestKind + `","config":{"address":"a"},"service":"api","refresh_seconds":5}`: `"code":55209`,
	} {
		recorder = request(router, http.MethodPost, "/upstream_discoveries", body, session)
		require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), code, body)
	}

	recorder = request(router, http.MethodPost, "/upstream_discoveries", binding("api", "keepalive 32;"), session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created model.UpstreamDiscovery
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &created))
	assert.Equal(t, discovery.DefaultRefreshSeconds, created.RefreshSeconds)

	var raw string
	require.NoError(t, model.UseDB().Raw("SELECT config FROM upstream_discoveries WHERE id = ?", created.ID).Scan(&raw).Error)
	require.NotContains(t, raw, "secret-token", "the configuration is encrypted at rest")

	// An upstream name is bound once.
	recorder = request(router, http.MethodPost, "/upstream_discoveries", binding("api", ""), session)
	require.Contains(t, recorder.Body.String(), `"code":55211`)

	path := "/upstream_discoveries/" + strconv.FormatUint(created.ID, 10)
	recorder = request(router, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"include":"include upstreams/api.conf;"`)

	tokenRouter := newRouter(&internalmcp.ServiceTokenPrincipal{PublicID: "t", Scopes: []string{"read"}}, nil)
	for _, readPath := range []string{"/upstream_discoveries", path} {
		recorder = request(tokenRouter, http.MethodGet, readPath, "", "")
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.NotContains(t, recorder.Body.String(), "secret-token")
		require.NotContains(t, recorder.Body.String(), `"config"`)
	}
}

func TestRefreshRenameAndDelete(t *testing.T) {
	stub, confDir := setup(t)
	admin := &model.User{Model: model.Model{ID: 402}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	session := internaluser.SetSecureSessionID(admin.ID)
	router := newRouter(nil, admin)

	recorder := request(router, http.MethodPost, "/upstream_discoveries", binding("api", "keepalive 32;"), session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created model.UpstreamDiscovery
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &created))
	id := strconv.FormatUint(created.ID, 10)

	recorder = request(router, http.MethodPost, "/upstream_discoveries/"+id+"/refresh", "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"last_status":"ok"`)
	assert.Contains(t, recorder.Body.String(), `"target_count":1`)
	oldFile := filepath.Join(confDir, "upstreams", "api.conf")
	content, err := os.ReadFile(oldFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "server 10.0.0.5:8080 weight=2;\n    keepalive 32;\n}")

	// Renaming removes the file of the old name and makes the binding due.
	recorder = request(router, http.MethodPost, "/upstream_discoveries/"+id, `{"upstream_name":"api_v2"}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.NoFileExists(t, oldFile)
	stored, err := query.UpstreamDiscovery.Where(query.UpstreamDiscovery.ID.Eq(created.ID)).First()
	require.NoError(t, err)
	assert.Nil(t, stored.NextRunAt)
	recorder = request(router, http.MethodPost, "/upstream_discoveries/"+id+"/refresh", "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	newFile := filepath.Join(confDir, "upstreams", "api_v2.conf")
	assert.FileExists(t, newFile)

	// Deleting is refused while nginx still needs the upstream.
	stub.failTest = true
	recorder = request(router, http.MethodDelete, "/upstream_discoveries/"+id, "", session)
	require.Contains(t, recorder.Body.String(), `"code":55213`)
	assert.FileExists(t, newFile)

	stub.failTest = false
	recorder = request(router, http.MethodDelete, "/upstream_discoveries/"+id, "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	assert.NoFileExists(t, newFile)

	// A binding whose name was taken meanwhile cannot be recovered.
	recorder = request(router, http.MethodPost, "/upstream_discoveries", binding("api_v2", ""), session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	recorder = request(router, http.MethodPatch, "/upstream_discoveries/"+id, "", session)
	require.Contains(t, recorder.Body.String(), `"code":55211`)
}
