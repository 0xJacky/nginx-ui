package blocklist

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
	internalblocklist "github.com/0xJacky/Nginx-UI/internal/security/blocklist"
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

const apiTestKind = "plugin:api-feed"

// apiFeed serves apiTestKind.
type apiFeed struct {
	mu      sync.Mutex
	entries []internalblocklist.Entry
}

func (f *apiFeed) Kinds() []internalblocklist.Kind {
	return []internalblocklist.Kind{{Kind: apiTestKind, Name: "API feed", PluginID: "io.github.example.feed", RefreshSeconds: 900,
		Fields: []internalblocklist.KindField{
			{Key: "api_key", DisplayName: "API key", Required: true, Secret: true},
		}}}
}

func (f *apiFeed) Fetch(_ context.Context, _ string, config map[string]string) (internalblocklist.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if config["api_key"] == "down" {
		return internalblocklist.Result{}, errors.New("feed is down")
	}
	return internalblocklist.Result{Entries: f.entries}, nil
}

var registerAPIFeed = sync.OnceValue(func() *apiFeed {
	feed := &apiFeed{entries: []internalblocklist.Entry{{CIDR: "203.0.113.0/24"}, {CIDR: "not an address"}}}
	internalblocklist.RegisterSource(feed)
	return feed
})

// stubNginx stands in for nginx -t and the reload.
type stubNginx struct {
	mu       sync.Mutex
	failTest bool
	reloads  int
}

func (s *stubNginx) writer() config.GeneratedWriter {
	return config.GeneratedWriter{
		Test: func() error {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.failTest {
				return errors.New("open() failed")
			}
			return nil
		},
		Reload: func() error {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.reloads++
			return nil
		},
	}
}

func setup(t *testing.T) (*stubNginx, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	registerAPIFeed()
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.User{}, model.BlocklistSource{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "blocklist.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)

	confDir := t.TempDir()
	stub := &stubNginx{}
	previous := internalblocklist.Default()
	internalblocklist.SetDefault(internalblocklist.NewRunner(context.Background(),
		internalblocklist.WithWriter(stub.writer()),
		internalblocklist.WithConfDir(func() string { return confDir }),
	))
	t.Cleanup(func() { internalblocklist.SetDefault(previous) })
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

func TestSourcesAreValidatedAndStoredEncrypted(t *testing.T) {
	setup(t)
	admin := &model.User{Model: model.Model{ID: 301}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	require.NoError(t, model.UseDB().Create(admin).Error)
	session := internaluser.SetSecureSessionID(admin.ID)
	router := newRouter(nil, admin)

	recorder := request(router, http.MethodGet, "/blocklist_sources/kinds", "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), apiTestKind)
	require.Contains(t, recorder.Body.String(), `"refresh_seconds":900`)

	// A required field left empty, a kind nobody serves and a short interval
	// are refused.
	for body, code := range map[string]string{
		`{"name":"feed","kind":"` + apiTestKind + `","config":{},"enabled":true}`:                                   `"code":55206`,
		`{"name":"feed","kind":"plugin:gone","config":{"api_key":"k"},"enabled":true}`:                              `"code":55205`,
		`{"name":"feed","kind":"` + apiTestKind + `","config":{"api_key":"k"},"refresh_seconds":30,"enabled":true}`: `"code":55209`,
	} {
		recorder = request(router, http.MethodPost, "/blocklist_sources", body, session)
		require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), code, body)
	}
	count, err := query.BlocklistSource.Count()
	require.NoError(t, err)
	require.Zero(t, count)

	// Changes need a secure session.
	recorder = request(router, http.MethodPost, "/blocklist_sources",
		`{"name":"feed","kind":"`+apiTestKind+`","config":{"api_key":"secret-key"},"enabled":true}`, "")
	require.NotEqual(t, http.StatusOK, recorder.Code)

	// The interval defaults to the one of the kind, the runner fields are
	// not writable.
	recorder = request(router, http.MethodPost, "/blocklist_sources",
		`{"name":"feed","kind":"`+apiTestKind+`","config":{"api_key":"secret-key"},"enabled":true,"last_status":"ok","entry_count":99}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created model.BlocklistSource
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &created))
	assert.Equal(t, 900, created.RefreshSeconds)
	assert.Empty(t, created.LastStatus)
	assert.Zero(t, created.EntryCount)
	assert.Nil(t, created.NextRunAt)

	var raw string
	require.NoError(t, model.UseDB().Raw("SELECT config FROM blocklist_sources WHERE id = ?", created.ID).Scan(&raw).Error)
	require.NotContains(t, raw, "secret-key", "the configuration is encrypted at rest")

	// The view carries the include line and the file.
	path := "/blocklist_sources/" + strconv.FormatUint(created.ID, 10)
	recorder = request(router, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"include":"include blocklists/`+strconv.FormatUint(created.ID, 10)+`.conf;"`)
	require.Contains(t, recorder.Body.String(), `"path":`)

	// An MCP service token reads sources without their values.
	tokenRouter := newRouter(&internalmcp.ServiceTokenPrincipal{PublicID: "t", Scopes: []string{"read"}}, nil)
	for _, readPath := range []string{"/blocklist_sources", path} {
		recorder = request(tokenRouter, http.MethodGet, readPath, "", "")
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.NotContains(t, recorder.Body.String(), "secret-key")
		require.NotContains(t, recorder.Body.String(), `"config"`)
	}
}

func TestRefreshAndDelete(t *testing.T) {
	stub, confDir := setup(t)
	admin := &model.User{Model: model.Model{ID: 302}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	require.NoError(t, model.UseDB().Create(admin).Error)
	session := internaluser.SetSecureSessionID(admin.ID)
	router := newRouter(nil, admin)

	source := &model.BlocklistSource{Name: "feed", Kind: apiTestKind, Config: map[string]string{"api_key": "k"}, RefreshSeconds: 900, Enabled: true}
	require.NoError(t, query.BlocklistSource.Create(source))
	id := strconv.FormatUint(source.ID, 10)

	recorder := request(router, http.MethodPost, "/blocklist_sources/"+id+"/refresh", "", "")
	require.NotEqual(t, http.StatusOK, recorder.Code, "refreshing needs a secure session")

	recorder = request(router, http.MethodPost, "/blocklist_sources/"+id+"/refresh", "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var refreshed struct {
		model.BlocklistSource
		Include string `json:"include"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &refreshed))
	assert.Equal(t, model.RefreshStatusOK, refreshed.LastStatus)
	assert.Equal(t, 1, refreshed.EntryCount)
	assert.Contains(t, refreshed.LastMessage, "1 invalid entries dropped")
	assert.Equal(t, "include blocklists/"+id+".conf;", refreshed.Include)
	file := filepath.Join(confDir, "blocklists", id+".conf")
	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(content), "deny 203.0.113.0/24;")

	// Changing the configuration makes the source due again.
	recorder = request(router, http.MethodPost, "/blocklist_sources/"+id, `{"config":{"api_key":"other"}}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	stored, err := query.BlocklistSource.Where(query.BlocklistSource.ID.Eq(source.ID)).First()
	require.NoError(t, err)
	assert.Nil(t, stored.NextRunAt)
	assert.Equal(t, "other", stored.Config["api_key"])

	// A failed fetch is recorded, not returned as an error.
	recorder = request(router, http.MethodPost, "/blocklist_sources/"+id, `{"config":{"api_key":"down"}}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	recorder = request(router, http.MethodPost, "/blocklist_sources/"+id+"/refresh", "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), `"last_status":"failed"`)
	assert.Contains(t, recorder.Body.String(), "feed is down")
	assert.FileExists(t, file)

	// Deleting is refused while nginx still includes the file.
	stub.failTest = true
	recorder = request(router, http.MethodDelete, "/blocklist_sources/"+id, "", session)
	require.NotEqual(t, http.StatusNoContent, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":55213`)
	assert.FileExists(t, file)

	stub.failTest = false
	recorder = request(router, http.MethodDelete, "/blocklist_sources/"+id, "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	assert.NoFileExists(t, file)

	// A recovered source is due at once, which writes its file back.
	recorder = request(router, http.MethodPatch, "/blocklist_sources/"+id, "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	stored, err = query.BlocklistSource.Where(query.BlocklistSource.ID.Eq(source.ID)).First()
	require.NoError(t, err)
	assert.Nil(t, stored.NextRunAt)
}
