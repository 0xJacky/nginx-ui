package streams

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
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupStreamSecurityTest(t *testing.T) string {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalJWTSecret := cosysettings.AppSettings.JwtSecret
	cosysettings.AppSettings.JwtSecret = "test-secret"

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	otpUser := &model.User{
		Model:     model.Model{ID: 2},
		Name:      "otp",
		Status:    true,
		Language:  "en",
		OTPSecret: []byte("otp-enabled"),
	}
	if err := db.Create(otpUser).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	payload, err := internaluser.GenerateJWT(otpUser)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	t.Cleanup(func() {
		cache.Shutdown()
		cosysettings.AppSettings.JwtSecret = originalJWTSecret
	})

	return payload.Token
}

func TestStreamSaveRequiresSecureSessionForOTPUser(t *testing.T) {
	token := setupStreamSecurityTest(t)

	router := gin.New()
	group := router.Group("/", middleware.AuthRequired())
	InitRouter(group)

	body, err := json.Marshal(gin.H{
		"content": "server {\n    listen 8080;\n}\n",
	})
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/streams/tcp_proxy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}
