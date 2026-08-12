package external_notify

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testNotifierType = "security-test"

func newExternalNotifySecurityRouter(
	serviceToken *internalmcp.ServiceTokenPrincipal,
	nodePrincipal *nodeauth.Principal,
	user *model.User,
) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if serviceToken != nil {
			c.Set(internalmcp.ServiceTokenPrincipalKey, serviceToken)
		}
		if nodePrincipal != nil {
			c.Set(nodeauth.GinPrincipalKey, nodePrincipal)
		}
		if user != nil {
			c.Set("user", user)
		}
		c.Next()
	})
	InitRouter(router.Group("/"))
	return router
}

func externalNotifyRequest(
	router http.Handler,
	method, path, body, secureSessionID string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		method,
		path,
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	if secureSessionID != "" {
		req.Header.Set("X-Secure-Session-ID", secureSessionID)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func externalNotifyTestRequest(router http.Handler, secureSessionID string) *httptest.ResponseRecorder {
	return externalNotifyRequest(
		router,
		http.MethodPost,
		"/external_notifies/test",
		`{"type":"`+testNotifierType+`","language":"en","config":{"url":"https://example.com"}}`,
		secureSessionID,
	)
}

func TestExternalNotifyTestRequiresInteractiveSecureSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	notification.RegisterExternalNotifier(testNotifierType, func(
		_ context.Context,
		_ *model.ExternalNotify,
		_ *notification.ExternalMessage,
	) error {
		return nil
	})

	otpUser := &model.User{
		Model:     model.Model{ID: 101},
		Name:      "otp-admin",
		Status:    true,
		OTPSecret: []byte("enabled"),
	}

	recorder := externalNotifyTestRequest(newExternalNotifySecurityRouter(nil, nil, otpUser), "")
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	secureSessionID := internaluser.SetSecureSessionID(otpUser.ID)
	recorder = externalNotifyTestRequest(newExternalNotifySecurityRouter(nil, nil, otpUser), secureSessionID)
	require.Equal(t, http.StatusOK, recorder.Code)

	serviceToken := &internalmcp.ServiceTokenPrincipal{
		PublicID: "security-test-token",
		Scopes:   []string{model.APITokenScopeWrite},
	}
	recorder = externalNotifyTestRequest(newExternalNotifySecurityRouter(serviceToken, nil, nil), "")
	require.Equal(t, http.StatusForbidden, recorder.Code)

	nodePrincipal := &nodeauth.Principal{AuthMethod: model.NodeAuthMethodPaired}
	recorder = externalNotifyTestRequest(newExternalNotifySecurityRouter(nil, nodePrincipal, nil), "")
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestExternalNotifyMutationsRequireInteractiveSecureSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	otpUser := &model.User{
		Model:     model.Model{ID: 102},
		Name:      "otp-admin",
		Status:    true,
		OTPSecret: []byte("enabled"),
	}

	recorder := externalNotifyRequest(
		newExternalNotifySecurityRouter(nil, nil, otpUser),
		http.MethodPost,
		"/external_notifies",
		`{"type":"wecom","language":"en","config":{"webhook_url":"http://127.0.0.1/internal"}}`,
		"",
	)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	serviceToken := &internalmcp.ServiceTokenPrincipal{
		PublicID: "security-test-token",
		Scopes:   []string{model.APITokenScopeWrite},
	}
	recorder = externalNotifyRequest(
		newExternalNotifySecurityRouter(serviceToken, nil, nil),
		http.MethodPost,
		"/external_notifies",
		`{"type":"wecom","language":"en","config":{"webhook_url":"http://127.0.0.1/internal"}}`,
		"",
	)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
