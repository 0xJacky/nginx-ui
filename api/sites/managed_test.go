package sites

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const managedSiteAPIContent = "server {\n    listen 80;\n    server_name managed.example.com;\n}\n"

// setupManagedSiteAPITest prepares the nginx tree and database for the site
// handlers, with the site file and row of name. Nginx test and reload are
// no-ops.
func setupManagedSiteAPITest(t *testing.T, name, managedBy string, enabled bool) *gorm.DB {
	t.Helper()

	db := setupSiteBatchUpdateTest(t)
	require.NoError(t, db.AutoMigrate(&model.Cert{}, &model.Node{}, &model.ConfigBackup{}, &model.LLMSession{}))

	confDir := settings.NginxSettings.ConfigDir
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}

	original := *settings.NginxSettings
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	settings.NginxSettings.TestConfigCmd = "true"
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))
	t.Cleanup(func() {
		settings.NginxSettings.PIDPath = original.PIDPath
		settings.NginxSettings.ReloadCmd = original.ReloadCmd
		settings.NginxSettings.RestartCmd = original.RestartCmd
		settings.NginxSettings.TestConfigCmd = original.TestConfigCmd
	})
	// Runs before the restores above: replications read the settings.
	t.Cleanup(site.WaitForSync)

	availablePath := sitePath(t, name)
	require.NoError(t, os.WriteFile(availablePath, []byte(managedSiteAPIContent), 0o644))
	require.NoError(t, db.Create(&model.Site{Path: availablePath, ManagedBy: managedBy}).Error)
	// The batch namespace endpoint moves sites into this one (id 1).
	require.NoError(t, db.Create(&model.Namespace{Name: "production"}).Error)
	if enabled {
		enabledPath, err := site.ResolveEnabledPath(name)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(availablePath, enabledPath))
	}
	return db
}

func serveManagedSiteRequest(t *testing.T, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	group := router.Group("/api")
	group.GET("sites", GetSiteList)
	group.GET("sites/:name", GetSite)
	group.PUT("sites", BatchUpdateSites)
	group.POST("sites/batch/enable", BatchEnableSites)
	group.POST("sites/batch/disable", BatchDisableSites)
	group.POST("sites/batch/maintenance", BatchEnableMaintenanceSites)
	group.POST("sites/:name/rename", RenameSite)
	group.POST("sites/:name/enable", EnableSite)
	group.POST("sites/:name/disable", DisableSite)
	group.POST("sites/:name", SaveSite)
	group.DELETE("sites/:name", DeleteSite)
	group.POST("sites/:name/maintenance", EnableMaintenanceSite)

	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

type managedSiteRequest struct {
	name    string
	enabled bool
	method  string
	path    string
	body    func(force bool) any
}

func managedSiteRequests() []managedSiteRequest {
	names := func(bool) any { return gin.H{"names": []string{"ingress-web"}} }
	return []managedSiteRequest{
		{name: "save", method: http.MethodPost, path: "/api/sites/ingress-web", body: func(force bool) any {
			return gin.H{"content": "server {\n    listen 8080;\n}\n", "overwrite": true, "force": force}
		}},
		{name: "rename", method: http.MethodPost, path: "/api/sites/ingress-web/rename", body: func(bool) any {
			return gin.H{"new_name": "ingress-web-renamed"}
		}},
		{name: "enable", method: http.MethodPost, path: "/api/sites/ingress-web/enable"},
		{name: "disable", enabled: true, method: http.MethodPost, path: "/api/sites/ingress-web/disable"},
		{name: "delete", method: http.MethodDelete, path: "/api/sites/ingress-web"},
		{name: "maintenance", enabled: true, method: http.MethodPost, path: "/api/sites/ingress-web/maintenance"},
		{name: "batch enable", method: http.MethodPost, path: "/api/sites/batch/enable", body: names},
		{name: "batch disable", enabled: true, method: http.MethodPost, path: "/api/sites/batch/disable", body: names},
		{name: "batch maintenance", enabled: true, method: http.MethodPost, path: "/api/sites/batch/maintenance", body: names},
		{name: "batch namespace", method: http.MethodPut, path: "/api/sites", body: func(bool) any {
			return gin.H{"ids": []string{"ingress-web"}, "data": gin.H{"namespace_id": 1}}
		}},
	}
}

func (r managedSiteRequest) send(t *testing.T, force bool) *httptest.ResponseRecorder {
	target := r.path
	// Save takes force from the body, every other endpoint from the query.
	if force && r.name != "save" {
		target += "?force=1"
	}
	var body any
	if r.body != nil {
		body = r.body(force)
	}
	return serveManagedSiteRequest(t, r.method, target, body)
}

func TestManagedSiteEndpointsAreRefusedUnlessForced(t *testing.T) {
	for _, req := range managedSiteRequests() {
		t.Run(req.name+"/refused", func(t *testing.T) {
			setupManagedSiteAPITest(t, "ingress-web", "ingress:web", req.enabled)
			before, err := os.ReadFile(sitePath(t, "ingress-web"))
			require.NoError(t, err)

			response := req.send(t, false)

			require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
			var body struct {
				Scope  string   `json:"scope"`
				Code   int32    `json:"code"`
				Params []string `json:"params"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, "site", body.Scope)
			assert.Equal(t, int32(40901), body.Code)
			assert.Equal(t, []string{"web"}, body.Params)

			after, err := os.ReadFile(sitePath(t, "ingress-web"))
			require.NoError(t, err, "a refused request must leave the site file in place")
			assert.Equal(t, before, after)
		})

		t.Run(req.name+"/forced", func(t *testing.T) {
			setupManagedSiteAPITest(t, "ingress-web", "ingress:web", req.enabled)

			response := req.send(t, true)

			assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
		})

		t.Run(req.name+"/unmanaged", func(t *testing.T) {
			setupManagedSiteAPITest(t, "ingress-web", "", req.enabled)

			response := req.send(t, false)

			assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
		})
	}
}

func TestBatchSiteRequestRefusesWholeBatchWithManagedSite(t *testing.T) {
	setupManagedSiteAPITest(t, "ingress-web", "ingress:web", false)
	plainPath := sitePath(t, "plain.example.com")
	require.NoError(t, os.WriteFile(plainPath, []byte(managedSiteAPIContent), 0o644))

	response := serveManagedSiteRequest(t, http.MethodPost, "/api/sites/batch/enable",
		gin.H{"names": []string{"plain.example.com", "ingress-web"}})

	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	enabledPath, err := site.ResolveEnabledPath("plain.example.com")
	require.NoError(t, err)
	_, statErr := os.Lstat(enabledPath)
	assert.True(t, os.IsNotExist(statErr), "no site of a refused batch may be enabled")
}

func TestSiteResponsesIncludeManagedBy(t *testing.T) {
	setupManagedSiteAPITest(t, "ingress-web", "ingress:web", true)

	response := serveManagedSiteRequest(t, http.MethodGet, "/api/sites/ingress-web", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var detail map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &detail))
	assert.Equal(t, "ingress:web", detail["managed_by"])

	response = serveManagedSiteRequest(t, http.MethodGet, "/api/sites", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var list struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &list))
	require.Len(t, list.Data, 1)
	assert.Equal(t, "ingress:web", list.Data[0]["managed_by"])
}

// The certificate of an ingress site belongs to the ingress: the auto renew
// switch of the site refuses it unless forced.
func TestAutoCertRefusesManagedSiteUnlessForced(t *testing.T) {
	db := setupManagedSiteAPITest(t, "ingress-web", "ingress:web", true)
	serve := func(method, target string, body any) *httptest.ResponseRecorder {
		router := gin.New()
		router.POST("/api/auto_cert/:name", AddDomainToAutoCert)
		router.DELETE("/api/auto_cert/:name", RemoveDomainFromAutoCert)
		var reader *bytes.Reader
		if body != nil {
			payload, err := json.Marshal(body)
			require.NoError(t, err)
			reader = bytes.NewReader(payload)
		} else {
			reader = bytes.NewReader(nil)
		}
		request := httptest.NewRequest(method, target, reader)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	require.NoError(t, db.Create(&model.Cert{Filename: "ingress-web", AutoCert: model.AutoCertEnabled}).Error)
	body := gin.H{"domains": []string{"web.example.com"}, "challenge_method": "http01"}

	response := serve(http.MethodPost, "/api/auto_cert/ingress-web", body)
	assert.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	response = serve(http.MethodDelete, "/api/auto_cert/ingress-web", nil)
	assert.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	var row model.Cert
	require.NoError(t, db.Where("filename = ?", "ingress-web").First(&row).Error)
	assert.Equal(t, model.AutoCertEnabled, row.AutoCert)

	response = serve(http.MethodDelete, "/api/auto_cert/ingress-web?force=1", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NoError(t, db.Where("filename = ?", "ingress-web").First(&row).Error)
	assert.Equal(t, model.AutoCertDisabled, row.AutoCert)
}
