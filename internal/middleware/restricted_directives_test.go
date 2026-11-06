package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type restrictedDirectiveProbe struct {
	allowed bool
	stepUp  RestrictedDirectiveStepUp
	err     error
}

// probeRestrictedDirectiveGate runs one request through RequireSecureSession,
// the middleware that marks a verified two-factor session, and reports what
// the gate decided behind it.
func probeRestrictedDirectiveGate(t *testing.T, prepare gin.HandlerFunc, sessionID string) restrictedDirectiveProbe {
	t.Helper()

	var probe restrictedDirectiveProbe

	router := gin.New()
	router.POST("/save", prepare, RequireSecureSession(), func(c *gin.Context) {
		probe.allowed = RestrictedDirectiveSaveAllowed(c)
		probe.stepUp, probe.err = RestrictedDirectiveStepUpFor(c)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/save", nil)
	if sessionID != "" {
		req.Header.Set("X-Secure-Session-ID", sessionID)
	}
	router.ServeHTTP(httptest.NewRecorder(), req)

	return probe
}

func setupRestrictedDirectiveUsers(t *testing.T) (plainUser *model.User, otpUser *model.User) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Passkey{}))
	model.Use(db)

	plainUser = &model.User{Model: model.Model{ID: 1}, Name: "plain", Status: true}
	otpUser = &model.User{Model: model.Model{ID: 2}, Name: "otp", Status: true, OTPSecret: []byte("otp-enabled")}
	require.NoError(t, db.Create(plainUser).Error)
	require.NoError(t, db.Create(otpUser).Error)

	return plainUser, otpUser
}

// Only an interactive administrator whose two-factor session was verified for
// this request may save restricted directives. Everyone else is refused, so a
// stolen JWT, a write-scoped service token, an MCP tool or a cluster node
// cannot turn a configuration write into code execution.
func TestRestrictedDirectiveSaveAllowedOnlyForVerifiedInteractiveUser(t *testing.T) {
	plainUser, otpUser := setupRestrictedDirectiveUsers(t)
	otpSessionID := internaluser.SetSecureSessionID(otpUser.ID)

	t.Run("verified two-factor user", func(t *testing.T) {
		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) {
			c.Set("user", otpUser)
			c.Next()
		}, otpSessionID)

		require.NoError(t, probe.err)
		require.True(t, probe.allowed, "a verified two-factor session must be allowed to opt in")
		require.Equal(t, RestrictedDirectiveStepUpAvailable, probe.stepUp)
	})

	t.Run("two-factor user without a verified session", func(t *testing.T) {
		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) {
			c.Set("user", otpUser)
			c.Next()
		}, "")

		require.False(t, probe.allowed, "an unverified session must not be allowed to opt in")
	})

	t.Run("two-factor user with an invalid session id", func(t *testing.T) {
		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) {
			c.Set("user", otpUser)
			c.Next()
		}, "not-a-session")

		require.False(t, probe.allowed, "an invalid session id must not be allowed to opt in")
	})

	t.Run("user without a second factor", func(t *testing.T) {
		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) {
			c.Set("user", plainUser)
			c.Next()
		}, "")

		require.NoError(t, probe.err)
		require.False(t, probe.allowed, "an account with no second factor must not be allowed to opt in")
		require.Equal(t, RestrictedDirectiveStepUpUnavailable, probe.stepUp,
			"the account has to enrol a second factor first")
	})

	t.Run("cluster node principal", func(t *testing.T) {
		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) {
			c.Set("user", otpUser)
			c.Set(nodeauth.GinPrincipalKey, &nodeauth.Principal{AuthMethod: model.NodeAuthMethodLegacy})
			c.Next()
		}, otpSessionID)

		require.NoError(t, probe.err)
		require.False(t, probe.allowed, "a cluster node must never save restricted directives")
		require.Equal(t, RestrictedDirectiveStepUpImpossible, probe.stepUp,
			"a node keeps the plain rejection instead of a step-up offer")
	})

	t.Run("service token principal", func(t *testing.T) {
		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) {
			c.Set("user", otpUser)
			c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{
				Scopes: []string{model.APITokenScopeWrite},
			})
			c.Next()
		}, otpSessionID)

		require.NoError(t, probe.err)
		require.False(t, probe.allowed, "a service token must never save restricted directives")
		require.Equal(t, RestrictedDirectiveStepUpImpossible, probe.stepUp,
			"a service token keeps the plain rejection instead of a step-up offer")
	})

	t.Run("public demo", func(t *testing.T) {
		originalDemo := settings.NodeSettings.Demo
		settings.NodeSettings.Demo = true
		t.Cleanup(func() { settings.NodeSettings.Demo = originalDemo })

		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) {
			c.Set("user", otpUser)
			c.Next()
		}, internaluser.SetSecureSessionID(otpUser.ID))

		require.NoError(t, probe.err)
		require.False(t, probe.allowed,
			"the demo publishes its password and allows passkey enrollment, so a verified session there proves nothing")
		require.Equal(t, RestrictedDirectiveStepUpImpossible, probe.stepUp,
			"a demo visitor gets the plain rejection, not a confirmation they could satisfy")
	})

	t.Run("no authenticated principal at all", func(t *testing.T) {
		probe := probeRestrictedDirectiveGate(t, func(c *gin.Context) { c.Next() }, "")

		require.NoError(t, probe.err)
		require.False(t, probe.allowed)
		require.Equal(t, RestrictedDirectiveStepUpImpossible, probe.stepUp)
	})
}
