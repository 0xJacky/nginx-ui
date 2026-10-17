package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/ingress"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAPI(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	confDir := t.TempDir()
	for _, dir := range []string{"conf.d", "sites-available", "sites-enabled"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}
	original := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.TestConfigCmd = "true"
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))
	originalAllowed := settings.IngressSettings.AllowedBackends
	settings.IngressSettings.AllowedBackends = []string{"127.0.0.1/32"}
	t.Cleanup(func() {
		*settings.NginxSettings = original
		settings.IngressSettings.AllowedBackends = originalAllowed
	})

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Ingress{}, &model.Site{}, &model.Namespace{}, &model.Node{},
		&model.Config{}, &model.ConfigBackup{}, &model.Cert{}, &model.Stream{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	r := gin.New()
	g := r.Group("/api")
	g.GET("/ingresses", ListIngresses)
	g.GET("/ingresses/:name", GetIngress)
	g.PUT("/ingresses", SyncIngresses)
	g.PUT("/ingresses/:name", PutIngress)
	g.DELETE("/ingresses/:name", DeleteIngress)
	g.PATCH("/ingresses/:name/backends/:backend/servers", PatchIngressServer)
	g.POST("/ingresses/:name/reconcile", ReconcileIngress)
	return r
}

func spec(host string) model.IngressSpec {
	return model.IngressSpec{
		Hosts:    []string{host},
		Backends: []model.IngressBackend{{Name: "app", Servers: []model.IngressServer{{Address: "127.0.0.1:24001"}}}},
		Routes:   []model.IngressRoute{{PathPrefix: "/", Backend: "app"}},
		TLS:      model.IngressTLS{Mode: model.IngressTLSModeNone},
	}
}

func do(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type errorBody struct {
	Scope string `json:"scope"`
	Code  int32  `json:"code"`
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body
}

func TestStatusOf(t *testing.T) {
	tests := map[int32]int{
		40001: 400, 40009: 400, 40401: 404, 40801: 408, 40802: 408, 40901: 409, 40904: 409, 40905: 409, 50001: 500, 50002: 500,
		404: 404, 40000: 400, 10001: 500, 0: 500, 60001: 500,
	}
	for code, want := range tests {
		assert.Equal(t, want, statusOf(code), code)
	}
}

func TestPutGetListAndErrors(t *testing.T) {
	r := setupAPI(t)

	w := do(t, r, http.MethodPut, "/api/ingresses/web", ingress.UpsertRequest{Owner: "ctl", Spec: spec("web.example.com")})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var ing model.Ingress
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ing))
	assert.Equal(t, "web", ing.Name)
	assert.Equal(t, int64(1), ing.Generation)

	w = do(t, r, http.MethodGet, "/api/ingresses/web", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	w = do(t, r, http.MethodGet, "/api/ingresses?owner=ctl", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Data []model.Ingress `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Data, 1)
	w = do(t, r, http.MethodGet, "/api/ingresses?owner=nobody", nil)
	assert.JSONEq(t, `{"data":[]}`, w.Body.String())

	w = do(t, r, http.MethodGet, "/api/ingresses/missing", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, errorBody{Scope: "ingress", Code: 40401}, decodeError(t, w))

	w = do(t, r, http.MethodPut, "/api/ingresses/web", ingress.UpsertRequest{Owner: "other", Spec: spec("web.example.com")})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, int32(40901), decodeError(t, w).Code)

	w = do(t, r, http.MethodPut, "/api/ingresses/api", ingress.UpsertRequest{Owner: "other", Spec: spec("web.example.com")})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, int32(40902), decodeError(t, w).Code)

	bad := spec("bad host.example.com")
	w = do(t, r, http.MethodPut, "/api/ingresses/bad", ingress.UpsertRequest{Owner: "ctl", Spec: bad})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, int32(40003), decodeError(t, w).Code)

	outside := spec("out.example.com")
	outside.Backends[0].Servers[0].Address = "10.0.0.1:80"
	w = do(t, r, http.MethodPut, "/api/ingresses/out", ingress.UpsertRequest{Owner: "ctl", Spec: outside})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, int32(40005), decodeError(t, w).Code)

	w = do(t, r, http.MethodPut, "/api/ingresses/web?wait=soon", ingress.UpsertRequest{Owner: "ctl", Spec: spec("web.example.com")})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, int32(40000), decodeError(t, w).Code)

	w = do(t, r, http.MethodPut, "/api/ingresses/web", map[string]any{"spec": spec("web.example.com")})
	assert.Equal(t, http.StatusBadRequest, w.Code, "owner is required")

	w = do(t, r, http.MethodPatch, "/api/ingresses/web/backends/app/servers",
		ingress.ServerPatch{Owner: "ctl", Address: "127.0.0.1:1"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, int32(40009), decodeError(t, w).Code)

	w = do(t, r, http.MethodDelete, "/api/ingresses/web", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "delete needs the owner or force")
	w = do(t, r, http.MethodDelete, "/api/ingresses/web?owner=other", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestPutWaitTimesOutWithoutWorker(t *testing.T) {
	r := setupAPI(t)
	w := do(t, r, http.MethodPut, "/api/ingresses/slow?wait=50ms", ingress.UpsertRequest{Owner: "ctl", Spec: spec("slow.example.com")})
	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, errorBody{Scope: "ingress", Code: 40801}, decodeError(t, w))
}

// runWorker runs the apply worker until the test ends.
func runWorker(t *testing.T) {
	t.Helper()
	originalCheck := settings.UpstreamCheckSettings.Enabled
	settings.UpstreamCheckSettings.Enabled = false
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ingress.RunWorker(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		settings.UpstreamCheckSettings.Enabled = originalCheck
	})
}

func TestPutWaitReturnsTheAppliedIngress(t *testing.T) {
	r := setupAPI(t)
	runWorker(t)

	w := do(t, r, http.MethodPut, "/api/ingresses/waited?wait=10s", ingress.UpsertRequest{Owner: "ctl", Spec: spec("waited.example.com")})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var ing model.Ingress
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ing))
	assert.Equal(t, ing.Generation, ing.Status.ObservedGeneration)
	assert.Equal(t, model.IngressPhaseReady, ing.Status.Phase)
}

// A retry with an unchanged spec keeps the generation. ?wait must still wait
// for the apply this request queued instead of returning the status of the
// previous, failed apply.
func TestPutWaitWaitsForTheApplyOfThisRequest(t *testing.T) {
	r := setupAPI(t)
	settings.NginxSettings.TestConfigCmd = "false"
	runWorker(t)

	w := do(t, r, http.MethodPut, "/api/ingresses/retry?wait=10s", ingress.UpsertRequest{Owner: "ctl", Spec: spec("retry.example.com")})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var ing model.Ingress
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ing))
	require.Equal(t, model.IngressPhaseError, ing.Status.Phase)

	settings.NginxSettings.TestConfigCmd = "true"
	w = do(t, r, http.MethodPut, "/api/ingresses/retry?wait=10s", ingress.UpsertRequest{Owner: "ctl", Spec: spec("retry.example.com")})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ing))
	assert.Equal(t, int64(1), ing.Generation)
	assert.Equal(t, model.IngressPhaseReady, ing.Status.Phase, "the retry's own apply was awaited")

	w = do(t, r, http.MethodPost, "/api/ingresses/retry/reconcile?wait=10s", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ing))
	assert.Equal(t, model.IngressPhaseReady, ing.Status.Phase)
}

// A sync with prune and wait returns once the pruned ingresses are gone.
func TestSyncWaitCoversPrunedIngresses(t *testing.T) {
	r := setupAPI(t)
	runWorker(t)

	w := do(t, r, http.MethodPut, "/api/ingresses?owner=ctl&wait=10s", ingress.SyncRequest{Items: []ingress.SyncItem{
		{Name: "one", Spec: spec("one.example.com")},
		{Name: "two", Spec: spec("two.example.com")},
	}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = do(t, r, http.MethodPut, "/api/ingresses?owner=ctl&prune=1&wait=10s", ingress.SyncRequest{Items: []ingress.SyncItem{
		{Name: "one", Spec: spec("one.example.com")},
	}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = do(t, r, http.MethodGet, "/api/ingresses/two", nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "the pruned ingress is gone when the sync returns")
	_, err := os.Stat(filepath.Join(settings.NginxSettings.ConfigDir, "sites-available", "ingress-two"))
	assert.True(t, os.IsNotExist(err))
}

func TestPutDryRunReturnsApplyResult(t *testing.T) {
	r := setupAPI(t)
	w := do(t, r, http.MethodPut, "/api/ingresses/dry?dry_run=1", ingress.UpsertRequest{Owner: "ctl", Spec: spec("dry.example.com")})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var result ingress.ApplyResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "dry", result.Name)
	assert.Empty(t, result.Error)
	require.NotNil(t, result.Rendered)
	assert.Contains(t, result.Rendered.Upstreams, "ig-dry-app")

	w = do(t, r, http.MethodGet, "/api/ingresses/dry", nil)
	assert.Equal(t, http.StatusNotFound, w.Code, "a dry run stores nothing")
}

func TestSyncAndReconcile(t *testing.T) {
	r := setupAPI(t)
	w := do(t, r, http.MethodPut, "/api/ingresses?owner=ctl", ingress.SyncRequest{Items: []ingress.SyncItem{
		{Name: "one", Spec: spec("one.example.com")},
		{Name: "two", Spec: spec("two.example.com")},
	}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list struct {
		Data []model.Ingress `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list.Data, 2)

	w = do(t, r, http.MethodPut, "/api/ingresses?owner=bad%20owner", ingress.SyncRequest{})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = do(t, r, http.MethodPost, "/api/ingresses/one/reconcile", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	w = do(t, r, http.MethodPost, "/api/ingresses/none/reconcile", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
