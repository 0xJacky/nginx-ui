package certificate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
)

// setupDnsCredentialRecoverDB stores DNS credentials and the certificates
// that reference them in a fresh database.
func setupDnsCredentialRecoverDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.User{}, model.DnsCredential{}, model.Cert{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "dns-credential-recover.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)
}

// newDnsCredentialRecoverRouter mounts the DNS credential router for an
// administrator with two-factor authentication, who needs a secure session
// to change credentials, and returns that session.
func newDnsCredentialRecoverRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	setupDnsCredentialRecoverDB(t)

	admin := &model.User{Model: model.Model{ID: 501}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	require.NoError(t, model.UseDB().Create(admin).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})
	InitDNSCredentialRouter(router.Group("/api"))
	return router, internaluser.SetSecureSessionID(admin.ID)
}

// newDnsCredentialServiceTokenRouter mounts the DNS credential router for a
// service token holding scopes.
func newDnsCredentialServiceTokenRouter(t *testing.T, scopes ...string) *gin.Engine {
	t.Helper()
	setupDnsCredentialRecoverDB(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{
			PublicID: "token",
			Name:     "token",
			Scopes:   scopes,
		})
		c.Next()
	})
	InitDNSCredentialRouter(router.Group("/api"))
	return router
}

func dnsCredentialRequest(router http.Handler, method, path, body, session string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		req.Header.Set("X-Secure-Session-ID", session)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func dnsCredentialPath(id uint64) string {
	return "/api/dns_credentials/" + strconv.FormatUint(id, 10)
}

func createRecoverableDnsCredential(t *testing.T, name string) *model.DnsCredential {
	t.Helper()
	credential := &model.DnsCredential{
		Name:         name,
		Provider:     "Cloudflare",
		ProviderCode: "cloudflare",
		Config: &dns.Config{
			Name: "Cloudflare",
			Code: "cloudflare",
			Configuration: &dns.Configuration{
				Credentials: map[string]string{"CF_DNS_API_TOKEN": "token-value"},
			},
		},
	}
	require.NoError(t, model.UseDB().Create(credential).Error)
	return credential
}

// listedDnsCredentialIDs returns the ids the list endpoint shows, from the
// trash when trash is true.
func listedDnsCredentialIDs(t *testing.T, router http.Handler, trash bool) []uint64 {
	t.Helper()

	path := "/api/dns_credentials"
	if trash {
		path += "?trash=true"
	}
	recorder := dnsCredentialRequest(router, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var list struct {
		Data []model.DnsCredential `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &list), recorder.Body.String())
	ids := make([]uint64, 0, len(list.Data))
	for _, item := range list.Data {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestDnsCredentialTrashRecoversWithPatch(t *testing.T) {
	router, session := newDnsCredentialRecoverRouter(t)
	credential := createRecoverableDnsCredential(t, "cloudflare account")
	path := dnsCredentialPath(credential.ID)

	recorder := dnsCredentialRequest(router, http.MethodDelete, path, "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	require.NotContains(t, listedDnsCredentialIDs(t, router, false), credential.ID)
	require.Contains(t, listedDnsCredentialIDs(t, router, true), credential.ID)

	// The trash view recovers through useCurdApi's restoreItem, which sends
	// PATCH with an empty body.
	recorder = dnsCredentialRequest(router, http.MethodPatch, path, "{}", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())

	require.Contains(t, listedDnsCredentialIDs(t, router, false), credential.ID)
	require.NotContains(t, listedDnsCredentialIDs(t, router, true), credential.ID)

	// The stored provider settings come back intact.
	recorder = dnsCredentialRequest(router, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	recovered, err := query.DnsCredential.FirstByID(credential.ID)
	require.NoError(t, err)
	require.Equal(t, credential.Name, recovered.Name)
	require.Equal(t, "token-value", recovered.Config.Configuration.Credentials["CF_DNS_API_TOKEN"])
}

// Deleting a credential leaves the certificates that use it alone, so
// recovering it makes their reference resolve again.
func TestDnsCredentialRecoverRestoresCertificateReference(t *testing.T) {
	router, session := newDnsCredentialRecoverRouter(t)
	credential := createRecoverableDnsCredential(t, "cloudflare account")
	certificate := &model.Cert{Name: "example.com", DnsCredentialID: credential.ID}
	require.NoError(t, model.UseDB().Create(certificate).Error)

	recorder := dnsCredentialRequest(router, http.MethodDelete, dnsCredentialPath(credential.ID), "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	var stored model.Cert
	require.NoError(t, model.UseDB().Preload("DnsCredential").First(&stored, certificate.ID).Error)
	require.Equal(t, credential.ID, stored.DnsCredentialID)
	require.Nil(t, stored.DnsCredential)

	recorder = dnsCredentialRequest(router, http.MethodPatch, dnsCredentialPath(credential.ID), "{}", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	stored = model.Cert{}
	require.NoError(t, model.UseDB().Preload("DnsCredential").First(&stored, certificate.ID).Error)
	require.NotNil(t, stored.DnsCredential)
	require.Equal(t, credential.ID, stored.DnsCredential.ID)
}

func TestDnsCredentialRecoverUnknownIDIsNotFound(t *testing.T) {
	router, session := newDnsCredentialRecoverRouter(t)
	credential := createRecoverableDnsCredential(t, "cloudflare account")

	recorder := dnsCredentialRequest(router, http.MethodPatch, dnsCredentialPath(credential.ID+1000), "{}", session)
	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Contains(t, listedDnsCredentialIDs(t, router, false), credential.ID)
}

func TestDnsCredentialRecoverKeepsDeleteGuards(t *testing.T) {
	t.Run("read scope service token is refused", func(t *testing.T) {
		router := newDnsCredentialServiceTokenRouter(t, model.APITokenScopeRead)
		credential := createRecoverableDnsCredential(t, "cloudflare account")
		require.NoError(t, model.UseDB().Delete(credential).Error)

		recorder := dnsCredentialRequest(router, http.MethodPatch, dnsCredentialPath(credential.ID), "{}", "")
		require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedDnsCredentialIDs(t, router, false), credential.ID)
	})

	t.Run("write scope service token recovers", func(t *testing.T) {
		router := newDnsCredentialServiceTokenRouter(t, model.APITokenScopeRead, model.APITokenScopeWrite)
		credential := createRecoverableDnsCredential(t, "cloudflare account")
		require.NoError(t, model.UseDB().Delete(credential).Error)

		recorder := dnsCredentialRequest(router, http.MethodPatch, dnsCredentialPath(credential.ID), "{}", "")
		require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		require.Contains(t, listedDnsCredentialIDs(t, router, false), credential.ID)
	})

	t.Run("two-factor user without a secure session is refused", func(t *testing.T) {
		router, _ := newDnsCredentialRecoverRouter(t)
		credential := createRecoverableDnsCredential(t, "cloudflare account")
		require.NoError(t, model.UseDB().Delete(credential).Error)

		recorder := dnsCredentialRequest(router, http.MethodPatch, dnsCredentialPath(credential.ID), "{}", "")
		require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedDnsCredentialIDs(t, router, false), credential.ID)
	})
}
