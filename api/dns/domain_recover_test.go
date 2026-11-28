package dns

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
)

// setupDomainRecoverDB stores DNS domains and their credentials in a fresh
// database.
func setupDomainRecoverDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.User{}, model.DnsCredential{}, model.DnsDomain{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "dns-domain-recover.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)
}

// newDomainRecoverRouter mounts the DNS router for an administrator with
// two-factor authentication, who needs a secure session to change domains,
// and returns that session.
func newDomainRecoverRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	setupDomainRecoverDB(t)

	admin := &model.User{Model: model.Model{ID: 601}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	require.NoError(t, model.UseDB().Create(admin).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})
	InitRouter(router.Group("/api"))
	return router, internaluser.SetSecureSessionID(admin.ID)
}

// newDomainServiceTokenRouter mounts the DNS router for a service token
// holding scopes.
func newDomainServiceTokenRouter(t *testing.T, scopes ...string) *gin.Engine {
	t.Helper()
	setupDomainRecoverDB(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{
			PublicID: "token",
			Name:     "token",
			Scopes:   scopes,
		})
		c.Next()
	})
	InitRouter(router.Group("/api"))
	return router
}

func domainRecoverRequest(router http.Handler, method, path, body, session string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		req.Header.Set("X-Secure-Session-ID", session)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func domainPath(id uint64) string {
	return "/api/dns/domains/" + strconv.FormatUint(id, 10)
}

func createRecoverableDomain(t *testing.T, name string) *model.DnsDomain {
	t.Helper()
	credential := &model.DnsCredential{Name: "cloudflare account", Provider: "Cloudflare", ProviderCode: "cloudflare"}
	require.NoError(t, model.UseDB().Create(credential).Error)
	domain := &model.DnsDomain{Domain: name, DnsCredentialID: credential.ID}
	require.NoError(t, model.UseDB().Create(domain).Error)
	return domain
}

// listedDomainIDs returns the ids the list endpoint shows, from the trash
// when trash is true.
func listedDomainIDs(t *testing.T, router http.Handler, trash bool) []uint64 {
	t.Helper()

	path := "/api/dns/domains"
	if trash {
		path += "?trash=true"
	}
	recorder := domainRecoverRequest(router, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var list struct {
		Data []model.DnsDomain `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &list), recorder.Body.String())
	ids := make([]uint64, 0, len(list.Data))
	for _, item := range list.Data {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestDomainTrashRecoversWithPatch(t *testing.T) {
	router, session := newDomainRecoverRouter(t)
	domain := createRecoverableDomain(t, "example.com")
	path := domainPath(domain.ID)

	recorder := domainRecoverRequest(router, http.MethodDelete, path, "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	require.NotContains(t, listedDomainIDs(t, router, false), domain.ID)
	require.Contains(t, listedDomainIDs(t, router, true), domain.ID)

	// The trash view recovers through useCurdApi's restoreItem, which sends
	// PATCH with an empty body.
	recorder = domainRecoverRequest(router, http.MethodPatch, path, "{}", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())

	require.Contains(t, listedDomainIDs(t, router, false), domain.ID)
	require.NotContains(t, listedDomainIDs(t, router, true), domain.ID)

	var recovered model.DnsDomain
	require.NoError(t, model.UseDB().First(&recovered, domain.ID).Error)
	require.Equal(t, domain.Domain, recovered.Domain)
	require.Equal(t, domain.DnsCredentialID, recovered.DnsCredentialID)
}

func TestDomainRecoverUnknownIDIsNotFound(t *testing.T) {
	router, session := newDomainRecoverRouter(t)
	domain := createRecoverableDomain(t, "example.com")

	recorder := domainRecoverRequest(router, http.MethodPatch, domainPath(domain.ID+1000), "{}", session)
	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Contains(t, listedDomainIDs(t, router, false), domain.ID)
}

func TestDomainRecoverKeepsDeleteGuards(t *testing.T) {
	t.Run("read scope service token is refused", func(t *testing.T) {
		router := newDomainServiceTokenRouter(t, model.APITokenScopeRead)
		domain := createRecoverableDomain(t, "example.com")
		require.NoError(t, model.UseDB().Delete(domain).Error)

		recorder := domainRecoverRequest(router, http.MethodPatch, domainPath(domain.ID), "{}", "")
		require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedDomainIDs(t, router, false), domain.ID)
	})

	t.Run("write scope service token recovers", func(t *testing.T) {
		router := newDomainServiceTokenRouter(t, model.APITokenScopeRead, model.APITokenScopeWrite)
		domain := createRecoverableDomain(t, "example.com")
		require.NoError(t, model.UseDB().Delete(domain).Error)

		recorder := domainRecoverRequest(router, http.MethodPatch, domainPath(domain.ID), "{}", "")
		require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		require.Contains(t, listedDomainIDs(t, router, false), domain.ID)
	})

	t.Run("two-factor user without a secure session is refused", func(t *testing.T) {
		router, _ := newDomainRecoverRouter(t)
		domain := createRecoverableDomain(t, "example.com")
		require.NoError(t, model.UseDB().Delete(domain).Error)

		recorder := domainRecoverRequest(router, http.MethodPatch, domainPath(domain.ID), "{}", "")
		require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedDomainIDs(t, router, false), domain.ID)
	})

	t.Run("demo mode is refused", func(t *testing.T) {
		router, session := newDomainRecoverRouter(t)
		domain := createRecoverableDomain(t, "example.com")
		require.NoError(t, model.UseDB().Delete(domain).Error)

		previous := settings.NodeSettings.Demo
		settings.NodeSettings.Demo = true
		t.Cleanup(func() { settings.NodeSettings.Demo = previous })

		recorder := domainRecoverRequest(router, http.MethodPatch, domainPath(domain.ID), "{}", session)
		require.NotEqual(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), "demo")
		require.NotContains(t, listedDomainIDs(t, router, false), domain.ID)
	})
}
