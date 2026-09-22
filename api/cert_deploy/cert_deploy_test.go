package cert_deploy

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/api/certificate"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert/deploy"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
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

const apiTestKind = "plugin:api-cdn"

// apiSource serves apiTestKind and remembers the pushes.
type apiSource struct {
	mu        sync.Mutex
	validated int
	pushes    []bool
}

func (s *apiSource) Kinds() []deploy.TargetKind {
	return []deploy.TargetKind{{Kind: apiTestKind, Name: "API CDN", PluginID: "io.github.example.cdn", Fields: []deploy.KindField{
		{Key: "zone_id", DisplayName: "Zone ID", Required: true},
		{Key: "api_token", DisplayName: "API token", Secret: true},
	}}}
}

func (s *apiSource) Validate(_ context.Context, _ string, config map[string]string) error {
	s.mu.Lock()
	s.validated++
	s.mu.Unlock()
	if config["zone_id"] == "missing" {
		return cosy.WrapErrorWithParams(plugin.ErrDeployConfigInvalid, "zone_id", "no such zone")
	}
	return nil
}

func (s *apiSource) Push(_ context.Context, _ string, config map[string]string, cert deploy.Certificate, dryRun bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushes = append(s.pushes, dryRun)
	if config["zone_id"] == "down" {
		return "", errors.New("cdn is down")
	}
	if dryRun {
		return "would push " + cert.Name, nil
	}
	return "pushed " + cert.Name, nil
}

func (s *apiSource) validations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.validated
}

var registerAPISource = sync.OnceValue(func() *apiSource {
	source := &apiSource{}
	deploy.RegisterSource(source)
	return source
})

func setupDB(t *testing.T) {
	t.Helper()
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Cert{}, model.CertDeployTarget{}, model.CertDeployment{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "cert-deploy.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)
}

// createCert stores a certificate whose files nginx could serve.
func createCert(t *testing.T, name string) *model.Cert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certPath := filepath.Join(dir, "fullchain.cer")
	keyPath := filepath.Join(dir, "private.key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))

	certModel := &model.Cert{Name: name, SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath}
	require.NoError(t, query.Cert.Create(certModel))
	return certModel
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
	g := router.Group("/")
	// Registered next to the certificate routes, as in the real router, so
	// a conflicting path panics here.
	certificate.InitCertificateRouter(g)
	InitRouter(g)
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

func TestTargetsAreValidatedAndStoredEncrypted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := registerAPISource()
	setupDB(t)
	certModel := createCert(t, "example.com")

	admin := &model.User{Model: model.Model{ID: 201}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	session := internaluser.SetSecureSessionID(admin.ID)
	router := newRouter(nil, admin)

	recorder := request(router, http.MethodGet, "/cert_deploy_targets/kinds", "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), apiTestKind)

	// A required field left empty is rejected before the plugin is asked.
	before := source.validations()
	recorder = request(router, http.MethodPost, "/cert_deploy_targets",
		`{"name":"cdn","kind":"`+apiTestKind+`","config":{"api_token":"t"},"enabled":true}`, session)
	require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"code":55204`)
	require.Equal(t, before, source.validations())

	// The plugin rejects a value.
	recorder = request(router, http.MethodPost, "/cert_deploy_targets",
		`{"name":"cdn","kind":"`+apiTestKind+`","config":{"zone_id":"missing"},"enabled":true}`, session)
	require.Contains(t, recorder.Body.String(), `"code":55204`)

	// A kind nobody serves and a certificate that does not exist are refused.
	recorder = request(router, http.MethodPost, "/cert_deploy_targets",
		`{"name":"cdn","kind":"plugin:gone","config":{"zone_id":"z"},"enabled":true}`, session)
	require.Contains(t, recorder.Body.String(), `"code":55203`)
	recorder = request(router, http.MethodPost, "/cert_deploy_targets",
		`{"name":"cdn","kind":"`+apiTestKind+`","config":{"zone_id":"z"},"cert_id":999,"enabled":true}`, session)
	require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
	count, err := query.CertDeployTarget.Count()
	require.NoError(t, err)
	require.Zero(t, count)

	recorder = request(router, http.MethodPost, "/cert_deploy_targets",
		`{"name":"cdn","kind":"`+apiTestKind+`","config":{"zone_id":"z","api_token":"secret-token"},"cert_id":`+
			strconv.FormatUint(certModel.ID, 10)+`,"enabled":true}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created model.CertDeployTarget
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &created))
	require.True(t, created.Enabled)

	var raw string
	require.NoError(t, model.UseDB().Raw("SELECT config FROM cert_deploy_targets WHERE id = ?", created.ID).Scan(&raw).Error)
	require.NotContains(t, raw, "secret-token", "the configuration is encrypted at rest")

	// Toggling the switch is not validated again; a disabled target stays so.
	validated := source.validations()
	path := "/cert_deploy_targets/" + strconv.FormatUint(created.ID, 10)
	recorder = request(router, http.MethodPost, path, `{"enabled":false}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, validated, source.validations())
	stored, err := query.CertDeployTarget.Where(query.CertDeployTarget.ID.Eq(created.ID)).First()
	require.NoError(t, err)
	require.False(t, stored.Enabled)
	require.Equal(t, "secret-token", stored.Config["api_token"])

	// Changes need a secure session.
	recorder = request(router, http.MethodPost, path, `{"enabled":true}`, "")
	require.NotEqual(t, http.StatusOK, recorder.Code)

	// An MCP service token reads targets without their values.
	tokenRouter := newRouter(&internalmcp.ServiceTokenPrincipal{PublicID: "t", Scopes: []string{"read"}}, nil)
	for _, readPath := range []string{"/cert_deploy_targets", path} {
		recorder = request(tokenRouter, http.MethodGet, readPath, "", "")
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.NotContains(t, recorder.Body.String(), "secret-token")
		require.NotContains(t, recorder.Body.String(), `"config"`)
	}
}

func TestDeployEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	source := registerAPISource()
	setupDB(t)
	certModel := createCert(t, "example.com")
	certID := strconv.FormatUint(certModel.ID, 10)

	up := &model.CertDeployTarget{Name: "up", Kind: apiTestKind, Config: map[string]string{"zone_id": "z"}, Enabled: true}
	down := &model.CertDeployTarget{Name: "down", Kind: apiTestKind, Config: map[string]string{"zone_id": "down"}, CertID: certModel.ID, Enabled: true}
	require.NoError(t, query.CertDeployTarget.Create(up, down))

	admin := &model.User{Model: model.Model{ID: 202}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	session := internaluser.SetSecureSessionID(admin.ID)
	router := newRouter(nil, admin)

	// Pushing needs a secure session.
	recorder := request(router, http.MethodPost, "/certs/"+certID+"/deploy", "", "")
	require.NotEqual(t, http.StatusOK, recorder.Code)

	recorder = request(router, http.MethodPost, "/certs/"+certID+"/deploy", "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var results struct {
		Data []deploy.Result `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &results))
	require.Len(t, results.Data, 2)
	assert.Equal(t, model.CertDeploymentOK, results.Data[0].Status)
	assert.Equal(t, "pushed example.com", results.Data[0].Message)
	assert.Equal(t, model.CertDeploymentFailed, results.Data[1].Status)
	assert.Contains(t, results.Data[1].Message, "cdn is down")

	recorder = request(router, http.MethodGet, "/certs/"+certID+"/deploy_targets", "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var targets struct {
		Data []certificateTarget `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &targets))
	require.Len(t, targets.Data, 2)
	assert.Equal(t, "API CDN", targets.Data[0].KindName)
	require.NotNil(t, targets.Data[1].Last)
	assert.Equal(t, model.CertDeploymentFailed, targets.Data[1].Last.Status)
	assert.NotContains(t, recorder.Body.String(), `"config"`)

	recorder = request(router, http.MethodPost, "/cert_deploy_targets/"+strconv.FormatUint(up.ID, 10)+"/deploy", "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	recorder = request(router, http.MethodGet, "/cert_deploy_targets/"+strconv.FormatUint(up.ID, 10)+"/deployments", "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var deployments struct {
		Data []model.CertDeployment `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &deployments))
	require.Len(t, deployments.Data, 2)

	// A dry run tests a configuration and records nothing.
	pushes := len(source.pushes)
	recorder = request(router, http.MethodPost, "/cert_deploy_targets/test",
		`{"kind":"`+apiTestKind+`","config":{"zone_id":"z"},"cert_id":`+certID+`}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "would push example.com")
	require.True(t, source.pushes[pushes])
	recorder = request(router, http.MethodPost, "/cert_deploy_targets/test",
		`{"kind":"`+apiTestKind+`","config":{}}`, session)
	require.Contains(t, recorder.Body.String(), `"code":55204`)
	count, err := query.CertDeployment.Count()
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}
