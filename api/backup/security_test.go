package backup

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

func setupAutoBackupSecurityRouter(t *testing.T) (*gin.Engine, string) {
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

	otpUser := &model.User{
		Model:     model.Model{ID: 1},
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
	InitAutoBackupRouter(group)

	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
	})

	return router, payload.Token
}

func TestAutoBackupMutationRequiresSecureSessionForOTPUser(t *testing.T) {
	router, token := setupAutoBackupSecurityRouter(t)

	body, err := json.Marshal(gin.H{"name": "daily"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auto_backup/test_s3", bytes.NewReader(body))
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func setupBackupSecurityRouter(t *testing.T) (*gin.Engine, string, uint64) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	cSettings.AppSettings.JwtSecret = "test-secret"

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s-backup?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	otpUser := &model.User{
		Model:     model.Model{ID: 2},
		Name:      "otp-backup",
		Status:    true,
		Language:  "en",
		OTPSecret: []byte("otp-enabled"),
	}
	require.NoError(t, db.Create(otpUser).Error)

	payload, err := internaluser.GenerateJWT(otpUser)
	require.NoError(t, err)

	router := gin.New()
	InitRouter(router.Group(""))

	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
	})

	return router, payload.Token, otpUser.ID
}

func TestBackupCreateAndRestoreRequireSecureSessionForOTPUser(t *testing.T) {
	router, token, _ := setupBackupSecurityRouter(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "create backup", method: http.MethodGet, path: "/backup"},
		{name: "restore backup", method: http.MethodPost, path: "/restore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestBackupRestoreAllowsValidSecureSessionForOTPUser(t *testing.T) {
	router, token, userID := setupBackupSecurityRouter(t)
	sessionID := internaluser.SetSecureSessionID(userID)

	req := httptest.NewRequest(http.MethodPost, "/restore", nil)
	req.Header.Set("Authorization", token)
	req.Header.Set("X-Secure-Session-ID", sessionID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.NotEqual(t, http.StatusUnauthorized, recorder.Code)
}
