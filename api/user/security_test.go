package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
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

type manageUserAuthorizationFixture struct {
	router           *gin.Engine
	database         *gorm.DB
	interactiveToken string
	readToken        string
	writeToken       string
}

func setupManageUserAuthorizationFixture(t *testing.T) manageUserAuthorizationFixture {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	originalCryptoSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	cSettings.AppSettings.JwtSecret = "test-secret"
	settings.CryptoSettings.Secret = "user-authorization-test-root"
	settings.NodeSettings.InstanceID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.User{})
	database := cosy.InitDB(sqlite.Open(fmt.Sprintf("file:%s-authorization?mode=memory&cache=shared", t.Name())))
	require.NoError(t, database.AutoMigrate(
		&model.AuthToken{},
		&model.Passkey{},
		&model.MCPServiceToken{},
	))

	model.Use(database)
	query.Use(database)
	query.SetDefault(database)

	administrator := &model.User{
		Model:    model.Model{ID: 1},
		Name:     "interactive-admin",
		Status:   true,
		Language: "en",
	}
	require.NoError(t, database.Create(administrator).Error)
	require.NoError(t, database.Create(&model.User{
		Model:    model.Model{ID: 2},
		Name:     "managed-user",
		Status:   true,
		Language: "en",
	}).Error)

	interactivePayload, err := internaluser.GenerateJWT(administrator)
	require.NoError(t, err)
	_, readToken, err := internalmcp.CreateServiceToken("user-reader", []string{model.APITokenScopeRead}, nil, administrator.ID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("user-writer", []string{model.APITokenScopeWrite}, nil, administrator.ID)
	require.NoError(t, err)

	router := gin.New()
	group := router.Group("/api", middleware.AuthRequired())
	InitManageUserRouter(group)

	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		settings.CryptoSettings.Secret = originalCryptoSecret
		settings.NodeSettings.InstanceID = originalInstanceID
	})

	return manageUserAuthorizationFixture{
		router:           router,
		database:         database,
		interactiveToken: interactivePayload.Token,
		readToken:        readToken,
		writeToken:       writeToken,
	}
}

func performUserRequest(router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	var requestBody bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		requestBody = *bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, &requestBody)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestManageUserServiceTokensAreReadOnly(t *testing.T) {
	fixture := setupManageUserAuthorizationFixture(t)

	require.Equal(t, http.StatusOK, performUserRequest(fixture.router, http.MethodGet, "/api/users", fixture.readToken, nil).Code)
	require.Equal(t, http.StatusOK, performUserRequest(fixture.router, http.MethodGet, "/api/users/2", fixture.readToken, nil).Code)

	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "create", method: http.MethodPost, path: "/api/users", body: gin.H{"name": "service-created", "password": "password", "status": true}},
		{name: "modify", method: http.MethodPost, path: "/api/users/2", body: gin.H{"name": "service-modified"}},
		{name: "delete", method: http.MethodDelete, path: "/api/users/2"},
		{name: "recover", method: http.MethodPatch, path: "/api/users/2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performUserRequest(fixture.router, tt.method, tt.path, fixture.writeToken, tt.body)
			require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		})
	}

	var users []model.User
	require.NoError(t, fixture.database.Unscoped().Order("id").Find(&users).Error)
	require.Len(t, users, 2)
	require.Equal(t, "managed-user", users[1].Name)
	require.Nil(t, users[1].DeletedAt)
}

func TestManageUserMutationsAllowInteractiveAdministrator(t *testing.T) {
	fixture := setupManageUserAuthorizationFixture(t)

	response := performUserRequest(fixture.router, http.MethodPost, "/api/users", fixture.interactiveToken, gin.H{
		"name": "interactive-created", "password": "password", "status": true,
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	var created model.User
	require.NoError(t, fixture.database.Where("name = ?", "interactive-created").First(&created).Error)
	require.NotEmpty(t, created.Password)

	response = performUserRequest(fixture.router, http.MethodPost, fmt.Sprintf("/api/users/%d", created.ID), fixture.interactiveToken, gin.H{
		"name": "interactive-modified",
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NoError(t, fixture.database.First(&created, created.ID).Error)
	require.Equal(t, "interactive-modified", created.Name)

	response = performUserRequest(fixture.router, http.MethodDelete, fmt.Sprintf("/api/users/%d", created.ID), fixture.interactiveToken, nil)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.ErrorIs(t, fixture.database.First(&model.User{}, created.ID).Error, gorm.ErrRecordNotFound)

	response = performUserRequest(fixture.router, http.MethodPatch, fmt.Sprintf("/api/users/%d", created.ID), fixture.interactiveToken, nil)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.NoError(t, fixture.database.First(&model.User{}, created.ID).Error)
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
