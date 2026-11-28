package cluster

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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
)

// setupNamespaceRecoverDB stores namespaces in a fresh database.
func setupNamespaceRecoverDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.User{}, model.Node{}, model.Namespace{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "namespace-recover.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)
}

// newNamespaceRecoverRouter mounts the cluster router for an administrator
// with two-factor authentication, who needs a secure session to change
// namespaces, and returns that session.
func newNamespaceRecoverRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	setupNamespaceRecoverDB(t)

	admin := &model.User{Model: model.Model{ID: 401}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	require.NoError(t, model.UseDB().Create(admin).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})
	InitRouter(router.Group("/api"))
	return router, internaluser.SetSecureSessionID(admin.ID)
}

// newNamespaceServiceTokenRouter mounts the cluster router for a service
// token holding scopes.
func newNamespaceServiceTokenRouter(t *testing.T, scopes ...string) *gin.Engine {
	t.Helper()
	setupNamespaceRecoverDB(t)

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

func namespaceRequest(router http.Handler, method, path, body, session string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		req.Header.Set("X-Secure-Session-ID", session)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func namespacePath(id uint64) string {
	return "/api/namespaces/" + strconv.FormatUint(id, 10)
}

func createRecoverableNamespace(t *testing.T, name string) *model.Namespace {
	t.Helper()
	namespace := &model.Namespace{Name: name}
	require.NoError(t, model.UseDB().Create(namespace).Error)
	return namespace
}

// listedNamespaceIDs returns the ids the list endpoint shows, from the trash
// when trash is true.
func listedNamespaceIDs(t *testing.T, router http.Handler, trash bool) []uint64 {
	t.Helper()

	path := "/api/namespaces"
	if trash {
		path += "?trash=true"
	}
	recorder := namespaceRequest(router, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var list struct {
		Data []model.Namespace `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &list), recorder.Body.String())
	ids := make([]uint64, 0, len(list.Data))
	for _, item := range list.Data {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestNamespaceTrashRecoversWithPatch(t *testing.T) {
	router, session := newNamespaceRecoverRouter(t)
	namespace := createRecoverableNamespace(t, "production")
	path := namespacePath(namespace.ID)

	recorder := namespaceRequest(router, http.MethodDelete, path, "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	require.NotContains(t, listedNamespaceIDs(t, router, false), namespace.ID)
	require.Contains(t, listedNamespaceIDs(t, router, true), namespace.ID)

	// The trash view recovers through useCurdApi's restoreItem, which sends
	// PATCH with an empty body.
	recorder = namespaceRequest(router, http.MethodPatch, path, "{}", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())

	require.Contains(t, listedNamespaceIDs(t, router, false), namespace.ID)
	require.NotContains(t, listedNamespaceIDs(t, router, true), namespace.ID)

	var recovered model.Namespace
	require.NoError(t, model.UseDB().First(&recovered, namespace.ID).Error)
	require.Equal(t, namespace.Name, recovered.Name)
}

func TestNamespaceRecoverKeepsPostRoute(t *testing.T) {
	router, session := newNamespaceRecoverRouter(t)
	namespace := createRecoverableNamespace(t, "staging")
	require.NoError(t, model.UseDB().Delete(namespace).Error)

	recorder := namespaceRequest(router, http.MethodPost, namespacePath(namespace.ID)+"/recover", "", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	require.Contains(t, listedNamespaceIDs(t, router, false), namespace.ID)
}

func TestNamespaceRecoverUnknownIDIsNotFound(t *testing.T) {
	router, session := newNamespaceRecoverRouter(t)
	namespace := createRecoverableNamespace(t, "production")

	recorder := namespaceRequest(router, http.MethodPatch, namespacePath(namespace.ID+1000), "{}", session)
	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Contains(t, listedNamespaceIDs(t, router, false), namespace.ID)
}

func TestNamespaceRecoverKeepsDeleteGuards(t *testing.T) {
	t.Run("read scope service token is refused", func(t *testing.T) {
		router := newNamespaceServiceTokenRouter(t, model.APITokenScopeRead)
		namespace := createRecoverableNamespace(t, "production")
		require.NoError(t, model.UseDB().Delete(namespace).Error)

		recorder := namespaceRequest(router, http.MethodPatch, namespacePath(namespace.ID), "{}", "")
		require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedNamespaceIDs(t, router, false), namespace.ID)
	})

	t.Run("write scope service token recovers", func(t *testing.T) {
		router := newNamespaceServiceTokenRouter(t, model.APITokenScopeRead, model.APITokenScopeWrite)
		namespace := createRecoverableNamespace(t, "production")
		require.NoError(t, model.UseDB().Delete(namespace).Error)

		recorder := namespaceRequest(router, http.MethodPatch, namespacePath(namespace.ID), "{}", "")
		require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		require.Contains(t, listedNamespaceIDs(t, router, false), namespace.ID)
	})

	t.Run("two-factor user without a secure session is refused", func(t *testing.T) {
		router, _ := newNamespaceRecoverRouter(t)
		namespace := createRecoverableNamespace(t, "production")
		require.NoError(t, model.UseDB().Delete(namespace).Error)

		recorder := namespaceRequest(router, http.MethodPatch, namespacePath(namespace.ID), "{}", "")
		require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedNamespaceIDs(t, router, false), namespace.ID)
	})
}
