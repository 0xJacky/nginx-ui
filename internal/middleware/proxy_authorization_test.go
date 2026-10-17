package middleware

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

type proxyAuthorizationTestState struct {
	writeToken, readToken, jwt, plainJWT, secureSession string
	nodeID                                              uint64
}

func setupProxyAuthorizationTest(t *testing.T) proxyAuthorizationTestState {
	t.Helper()
	logger.Init("debug")
	cache.InitInMemoryCache()
	readToken, writeToken, _ := setupAPIServiceTokenTest(t)
	db := model.UseDB()
	require.NoError(t, db.AutoMigrate(&model.Node{}, &model.User{}, &model.Passkey{}, &model.AuthToken{}))
	query.SetDefault(db)
	oldSecret := settings.NodeSettings.Secret
	oldJWT := cosysettings.AppSettings.JwtSecret
	settings.NodeSettings.Secret = "proxy-authorization-child-secret"
	cosysettings.AppSettings.JwtSecret = "proxy-authorization-jwt-secret"
	t.Cleanup(func() {
		settings.NodeSettings.Secret = oldSecret
		cosysettings.AppSettings.JwtSecret = oldJWT
		internaluser.InvalidateUserCache(1)
		internaluser.InvalidateUserCache(2)
	})
	admin := &model.User{Name: "proxy-mfa-admin", OTPSecret: []byte("enrolled-mfa"), Status: true}
	require.NoError(t, db.Create(admin).Error)
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	plain := &model.User{Name: "proxy-plain-admin", Status: true}
	require.NoError(t, db.Create(plain).Error)
	plainPayload, err := internaluser.GenerateJWT(plain)
	require.NoError(t, err)
	return proxyAuthorizationTestState{writeToken: writeToken, readToken: readToken, jwt: payload.Token, plainJWT: plainPayload.Token, secureSession: internaluser.SetSecureSessionID(admin.ID)}
}

func registerProxyTestNode(t *testing.T, endpoint string) uint64 {
	t.Helper()
	db := model.UseDB()
	node := &model.Node{Name: "proxy-child", URL: endpoint, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, db.Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte(settings.NodeSettings.Secret))
	require.NoError(t, err)
	require.NoError(t, db.Model(node).Update("encrypted_legacy_secret", encrypted).Error)
	return node.ID
}

func TestProxyAuthorizesOriginalCallerBeforeNodeAuthentication(t *testing.T) {
	state := setupProxyAuthorizationTest(t)
	sentinel := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	register := func(group *gin.RouterGroup) {
		group.POST("settings", RequireSecureSession(), sentinel)
		group.POST("settings/nginx/control", RequireSecureSession(), RequireVerifiedTwoFactorOrProxy("verified session required"), sentinel)
		group.GET("host/setup/publickey", RequireSecureSession(), func(c *gin.Context) {
			path := c.Query("private_key_path")
			if path != "" && path != settings.DefaultHostPrivateKeyPath && path != "/configured/custom-key" && !VerifiedTwoFactorOrProxy(c, "verified session required") {
				return
			}
			sentinel(c)
		})
		group.GET("settings/protected", RequireInteractiveUser(), RequireSecureSession(), sentinel)
		group.POST("nodes", RequireInteractiveUser(), RequireSecureSession(), sentinel)
		group.POST("namespace/sync", RequireSecureSession(), sentinel)
		group.POST("nginx_log/analytics", sentinel)
	}
	child := gin.New()
	register(child.Group("/api", AuthRequired()))
	childServer := httptest.NewServer(child)
	defer childServer.Close()
	state.nodeID = registerProxyTestNode(t, childServer.URL)
	controller := gin.New()
	register(controller.Group("/api", AuthRequired(), Proxy()))
	tests := []struct {
		name, method, path, token string
		verified, remote          bool
		want                      int
	}{
		{"MFA local mutation", "POST", "settings", state.jwt, false, false, 401},
		{"MFA remote mutation", "POST", "settings", state.jwt, false, true, 401},
		{"verified remote mutation", "POST", "settings", state.jwt, true, true, 204},
		{"non-MFA remote mutation", "POST", "settings", state.plainJWT, false, true, 204},
		{"write PAT remote mutation", "POST", "settings", state.writeToken, false, true, 204},
		{"read PAT remote mutation", "POST", "settings", state.readToken, false, true, 403},
		{"PAT remote interactive endpoint", "POST", "nodes", state.writeToken, false, true, 403},
		{"PAT remote verified endpoint", "POST", "settings/nginx/control", state.writeToken, false, true, 401},
		{"non-MFA remote verified endpoint", "POST", "settings/nginx/control", state.plainJWT, false, true, 401},
		{"verified remote verified endpoint", "POST", "settings/nginx/control", state.jwt, true, true, 204},
		{"MFA read-only POST", "POST", "nginx_log/analytics", state.jwt, false, true, 204},
		{"PAT read-only POST", "POST", "nginx_log/analytics", state.writeToken, false, true, 204},
		{"PAT remote default public key", "GET", "host/setup/publickey", state.writeToken, false, true, 204},
		{"PAT remote explicit default key", "GET", "host/setup/publickey?private_key_path=" + url.QueryEscape(settings.DefaultHostPrivateKeyPath), state.writeToken, false, true, 204},
		{"PAT remote configured custom key", "GET", "host/setup/publickey?private_key_path=/configured/custom-key", state.writeToken, false, true, 204},
		{"plain admin remote configured custom key", "GET", "host/setup/publickey?private_key_path=/configured/custom-key", state.plainJWT, false, true, 204},
		{"spoofed proxy proof", "GET", "host/setup/publickey?private_key_path=/tmp/other-key&" + proxyVerifiedTwoFactorQuery + "=true", state.writeToken, false, true, 401},
		{"duplicate spoofed proxy proof", "GET", "host/setup/publickey?private_key_path=/tmp/other-key&" + proxyVerifiedTwoFactorQuery + "=true&" + proxyVerifiedTwoFactorQuery + "=true", state.writeToken, false, true, 401},
		{"PAT remote arbitrary key", "GET", "host/setup/publickey?private_key_path=/tmp/other-key", state.writeToken, false, true, 401},
		{"verified remote arbitrary key", "GET", "host/setup/publickey?private_key_path=/tmp/other-key", state.jwt, true, true, 204},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := httptest.NewRequest(tc.method, "/api/"+tc.path, nil).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			if tc.remote {
				req.Header.Set("X-Node-ID", strconv.FormatUint(state.nodeID, 10))
			}
			if tc.verified {
				req.Header.Set("X-Secure-Session-ID", state.secureSession)
			}
			res := httptest.NewRecorder()
			controller.ServeHTTP(res, req)
			require.Equal(t, tc.want, res.Code, res.Body.String())
		})
	}
	// Node-originated synchronization keeps its existing exemption, but the
	// same principal still cannot enter an interactive-only route.
	for _, tc := range []struct {
		path string
		want int
	}{{"namespace/sync", 204}, {"nodes", 403}} {
		req := httptest.NewRequest("POST", "/api/"+tc.path, nil)
		req.Header.Set("X-Node-Secret", settings.NodeSettings.Secret)
		res := httptest.NewRecorder()
		controller.ServeHTTP(res, req)
		require.Equal(t, tc.want, res.Code, res.Body.String())
	}
}

func TestWebSocketProxyAuthorizesOriginalCaller(t *testing.T) {
	state := setupProxyAuthorizationTest(t)
	upgrade := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	child := gin.New()
	child.GET("/api/sites/:name/https", AuthRequiredWS(), RequireSecureSession(), func(c *gin.Context) {
		conn, err := upgrade.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte("child reached")); err != nil {
			t.Errorf("write child response: %v", err)
		}
	})
	childServer := httptest.NewServer(child)
	defer childServer.Close()
	nodeID := registerProxyTestNode(t, childServer.URL)
	controller := gin.New()
	controller.GET("/api/sites/:name/https", AuthRequiredWS(), ProxyWs(), RequireSecureSession(), func(c *gin.Context) { t.Error("local handler must not run") })
	server := httptest.NewServer(controller)
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/sites/test/https?x_node_id=" + strconv.FormatUint(nodeID, 10) + "&token=" + url.QueryEscape(base64.RawURLEncoding.EncodeToString([]byte(state.jwt)))
	conn, response, err := websocket.DefaultDialer.Dial(endpoint, nil)
	require.Error(t, err)
	require.Nil(t, conn)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	response.Body.Close()
	endpoint += "&X-Secure-Session-ID=" + url.QueryEscape(state.secureSession)
	conn, response, err = websocket.DefaultDialer.Dial(endpoint, nil)
	require.NoError(t, err)
	defer conn.Close()
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "child reached", string(message))
	for _, tc := range []struct {
		name, token string
		want        int
	}{
		{"write PAT", state.writeToken, http.StatusSwitchingProtocols},
		{"read PAT", state.readToken, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := url.Values{"x_node_id": {strconv.FormatUint(nodeID, 10)}, "token": {base64.RawURLEncoding.EncodeToString([]byte(tc.token))}}
			patEndpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/sites/test/https?" + values.Encode()
			connection, response, err := websocket.DefaultDialer.Dial(patEndpoint, nil)
			if tc.want == http.StatusSwitchingProtocols {
				require.NoError(t, err)
				defer connection.Close()
				_, message, err := connection.ReadMessage()
				require.NoError(t, err)
				require.Equal(t, "child reached", string(message))
			} else {
				require.Error(t, err)
				require.Nil(t, connection)
				defer response.Body.Close()
			}
			require.Equal(t, tc.want, response.StatusCode)
		})
	}
}

func TestNodeProxyTwoFactorProofRejectsInvalidAndDuplicateValues(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"", true},
		{proxyVerifiedTwoFactorQuery + "=true", true},
		{proxyVerifiedTwoFactorQuery + "=false", false},
		{proxyVerifiedTwoFactorQuery + "=TRUE", false},
		{proxyVerifiedTwoFactorQuery + "=", false},
		{proxyVerifiedTwoFactorQuery + "=true&" + proxyVerifiedTwoFactorQuery + "=true", false},
		{proxyVerifiedTwoFactorQuery + "=true&bad=%xx", false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			c := newTestGinContext(t, http.MethodGet, "/api/host/setup/publickey?"+tc.query, nil)
			c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{})
			require.Equal(t, tc.want, VerifiedTwoFactorOrProxy(c, "verified proof required"))
		})
	}
}

func TestProxyTwoFactorProofIsBoundToPairedNodeSignature(t *testing.T) {
	setupProxyAuthorizationTest(t)
	db := model.UseDB()
	require.NoError(t, db.AutoMigrate(&model.NodeControllerCredential{}))
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	credentialID := "22222222-2222-4222-8222-222222222222"
	require.NoError(t, db.Create(&model.NodeControllerCredential{CredentialID: credentialID, ControllerInstanceID: "33333333-3333-4333-8333-333333333333", PublicKey: publicKey, Status: model.NodeCredentialStatusActive}).Error)
	for _, tamper := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodGet, "http://child/api/host/setup/publickey?"+proxyVerifiedTwoFactorQuery+"=false", nil)
		require.NoError(t, nodeauth.SignRequestWithKey(req, credentialID, settings.NodeSettings.InstanceID, privateKey, time.Now()))
		if tamper {
			values := req.URL.Query()
			values.Set(proxyVerifiedTwoFactorQuery, "true")
			req.URL.RawQuery = values.Encode()
		}
		_, err := nodeauth.VerifyRequest(req)
		if tamper {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		nodeauth.CloseStagedBody(req)
	}
}
