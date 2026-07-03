package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

func setupUserSecurityRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	cSettings.AppSettings.JwtSecret = "test-secret"

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	require.NoError(t, db.Create(&model.User{
		Model:    model.Model{ID: 1},
		Name:     "admin",
		Status:   true,
		Language: "en",
	}).Error)

	otpUser := &model.User{
		Model:     model.Model{ID: 2},
		Name:      "otp",
		Status:    true,
		Language:  "en",
		OTPSecret: []byte("otp-enabled"),
	}
	require.NoError(t, db.Create(otpUser).Error)

	payload, err := internaluser.GenerateJWT(otpUser)
	require.NoError(t, err)

	router := gin.New()
	group := router.Group("/", middleware.AuthRequired())
	InitManageUserRouter(group)

	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
	})

	return router, payload.Token
}

func TestManageUserMutationRequiresSecureSessionForOTPUser(t *testing.T) {
	router, token := setupUserSecurityRouter(t)

	body, err := json.Marshal(gin.H{"password": "attacker-chosen"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/users/1", bytes.NewReader(body))
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func setupCurrentUserSecurityRouter(t *testing.T) (*gin.Engine, string, uint64) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	cSettings.AppSettings.JwtSecret = "test-secret"

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s-current?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	otpUser := &model.User{
		Model:     model.Model{ID: 3},
		Name:      "otp-current",
		Status:    true,
		Language:  "en",
		OTPSecret: []byte("otp-enabled"),
	}
	require.NoError(t, db.Create(otpUser).Error)

	payload, err := internaluser.GenerateJWT(otpUser)
	require.NoError(t, err)

	router := gin.New()
	group := router.Group("/", middleware.AuthRequired())
	InitUserRouter(group)

	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
	})

	return router, payload.Token, otpUser.ID
}

func TestCurrentUserSecurityRoutesRequireSecureSessionForOTPUser(t *testing.T) {
	router, token, _ := setupCurrentUserSecurityRouter(t)

	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "totp secret", method: http.MethodGet, path: "/otp_secret"},
		{name: "totp enroll", method: http.MethodPost, path: "/otp_enroll", body: gin.H{"secret": "secret", "passcode": "123456"}},
		{name: "passkey begin registration", method: http.MethodGet, path: "/begin_passkey_register"},
		{name: "passkey finish registration", method: http.MethodPost, path: "/finish_passkey_register"},
		{name: "passkey update", method: http.MethodPost, path: "/passkeys/1", body: gin.H{"name": "workstation"}},
		{name: "passkey delete", method: http.MethodDelete, path: "/passkeys/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Reader
			if tt.body != nil {
				payload, err := json.Marshal(tt.body)
				require.NoError(t, err)
				body = *bytes.NewReader(payload)
			}

			req := httptest.NewRequest(tt.method, tt.path, &body)
			req.Header.Set("Authorization", token)
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestCurrentUserSecurityRouteAllowsValidSecureSessionForOTPUser(t *testing.T) {
	router, token, userID := setupCurrentUserSecurityRouter(t)
	sessionID := internaluser.SetSecureSessionID(userID)

	req := httptest.NewRequest(http.MethodGet, "/otp_secret", nil)
	req.Header.Set("Authorization", token)
	req.Header.Set("X-Secure-Session-ID", sessionID)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
}
