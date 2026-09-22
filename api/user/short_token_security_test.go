package user

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupShortTokenSecurityTest(t *testing.T) (*gin.Engine, *gorm.DB, *model.User, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	previousSecret := cSettings.AppSettings.JwtSecret
	cSettings.AppSettings.JwtSecret = "short-token-security-test-secret"
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s-short-token?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	u := &model.User{Model: model.Model{ID: 1}, Name: "short-token-user", Status: true}
	require.NoError(t, db.Create(u).Error)
	payload, err := internaluser.GenerateJWT(u)
	require.NoError(t, err)

	router := gin.New()
	public := router.Group("/api")
	InitAuthRouter(public)
	httpRoutes := router.Group("/api", middleware.AuthRequired())
	InitTokenRouter(httpRoutes)
	httpRoutes.GET("/nginx/status", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	httpRoutes.POST("/system/restart", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	wsRoutes := router.Group("/api", middleware.AuthRequiredWS())
	wsRoutes.GET("/nginx/detail_status/ws", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = previousSecret
	})
	return router, db, u, payload.Token
}

func shortTokenRequest(router http.Handler, method, path, authorization string, withCookie bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if withCookie {
		req.AddCookie(&http.Cookie{Name: middleware.SecureSessionCookieName, Value: "forged-nonempty-cookie"})
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func issueWebSocketToken(t *testing.T, router http.Handler, jwt string) string {
	t.Helper()
	response := shortTokenRequest(router, http.MethodPost, "/api/token/short", "Bearer "+jwt, true)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		ShortToken string `json:"short_token"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Len(t, body.ShortToken, 16)
	return body.ShortToken
}

func TestShortTokenOnlyAuthenticatesWebSocketRoutes(t *testing.T) {
	router, db, u, jwt := setupShortTokenSecurityTest(t)
	shortToken := issueWebSocketToken(t, router, jwt)

	require.Equal(t, http.StatusNoContent, shortTokenRequest(router, http.MethodGet, "/api/nginx/status", "Bearer "+jwt, false).Code)
	require.Equal(t, http.StatusForbidden, shortTokenRequest(router, http.MethodGet, "/api/nginx/status", "Bearer "+shortToken, false).Code)
	require.Equal(t, http.StatusForbidden, shortTokenRequest(router, http.MethodPost, "/api/system/restart", "Bearer "+shortToken, false).Code)
	require.Equal(t, http.StatusForbidden, shortTokenRequest(router, http.MethodPost, "/api/token/short", "Bearer "+shortToken, true).Code)
	require.Equal(t, http.StatusForbidden, shortTokenRequest(router, http.MethodPost, "/api/token/short", "Bearer "+shortToken, false).Code)
	require.Equal(t, http.StatusNoContent, shortTokenRequest(router, http.MethodGet, "/api/nginx/detail_status/ws?token="+shortToken, "", false).Code)
	wsJWT := base64.RawURLEncoding.EncodeToString([]byte(jwt))
	require.Equal(t, http.StatusNoContent, shortTokenRequest(router, http.MethodGet, "/api/nginx/detail_status/ws?token="+wsJWT, "", false).Code)

	loaded, ok := internaluser.GetTokenUserByShortToken(shortToken)
	require.True(t, ok)
	require.Equal(t, u.ID, loaded.ID)

	legacyShortToken := "legacyshorttoken"
	legacy := &model.AuthToken{UserID: u.ID, ShortToken: legacyShortToken, ExpiredAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, db.Create(legacy).Error)
	require.Equal(t, http.StatusForbidden, shortTokenRequest(router, http.MethodGet, "/api/nginx/status", legacyShortToken, false).Code)
	require.Equal(t, http.StatusForbidden, shortTokenRequest(router, http.MethodGet, "/api/nginx/detail_status/ws?token="+legacyShortToken, "", false).Code)
	var legacyCount int64
	require.NoError(t, db.Model(&model.AuthToken{}).Where("short_token = ?", legacyShortToken).Count(&legacyCount).Error)
	require.Zero(t, legacyCount)
}

func TestLogoutRevokesOnlyIssuingSessionAndItsShortTokens(t *testing.T) {
	router, db, u, firstJWT := setupShortTokenSecurityTest(t)
	var firstRow model.AuthToken
	require.NoError(t, db.Where("token = ?", firstJWT).Take(&firstRow).Error)
	firstMinted := issueWebSocketToken(t, router, firstJWT)
	var firstMintedRow model.AuthToken
	require.NoError(t, db.Where("short_token = ?", firstMinted).Take(&firstMintedRow).Error)

	secondPayload, err := internaluser.GenerateJWT(u)
	require.NoError(t, err)
	secondJWT := secondPayload.Token
	require.NotEqual(t, firstJWT, secondJWT)
	secondMinted := issueWebSocketToken(t, router, secondJWT)
	var secondRow model.AuthToken
	require.NoError(t, db.Where("token = ?", secondJWT).Take(&secondRow).Error)
	require.Equal(t, http.StatusNoContent, shortTokenRequest(router, http.MethodGet, "/api/nginx/detail_status/ws?token="+firstMinted, "", false).Code)
	_, ok := internaluser.GetTokenUserByShortToken(firstRow.ShortToken)
	require.True(t, ok)
	_, ok = internaluser.GetTokenUserByShortToken(secondMinted)
	require.True(t, ok)
	cache.Del("auth_token:" + firstJWT)
	_, cached := internaluser.GetCachedShortTokenData(firstRow.ShortToken)
	require.True(t, cached)

	response := shortTokenRequest(router, http.MethodDelete, "/api/logout", "Bearer "+firstJWT, false)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	_, ok = internaluser.GetTokenUser(firstJWT)
	require.False(t, ok)
	_, ok = internaluser.GetTokenUserByShortToken(firstRow.ShortToken)
	require.False(t, ok)
	_, ok = internaluser.GetTokenUserByShortToken(firstMinted)
	require.False(t, ok)
	_, cached = internaluser.GetCachedShortTokenData(firstRow.ShortToken)
	require.False(t, cached)
	// A second instance may still hold these cache entries after DB revocation.
	internaluser.CacheToken(&firstRow)
	internaluser.CacheToken(&firstMintedRow)
	_, ok = internaluser.GetTokenUser(firstJWT)
	require.False(t, ok)
	_, ok = internaluser.GetTokenUserByShortToken(firstMinted)
	require.False(t, ok)
	_, ok = internaluser.GetTokenUser(secondJWT)
	require.True(t, ok)
	_, ok = internaluser.GetTokenUserByShortToken(secondMinted)
	require.True(t, ok)
	var secondMintedRow model.AuthToken
	require.NoError(t, db.Where("short_token = ?", secondMinted).Take(&secondMintedRow).Error)
	require.NoError(t, internaluser.RevokeShortToken(secondMinted))
	internaluser.CacheToken(&secondMintedRow)
	_, ok = internaluser.GetTokenUserByShortToken(secondMinted)
	require.False(t, ok)
	_, ok = internaluser.GetTokenUser(secondJWT)
	require.True(t, ok)

	response = shortTokenRequest(router, http.MethodDelete, "/api/logout", secondJWT, false)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	_, ok = internaluser.GetTokenUserByShortToken(secondRow.ShortToken)
	require.False(t, ok)
}
