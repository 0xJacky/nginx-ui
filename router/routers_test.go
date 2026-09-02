//go:build unembed

package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cosyRouter "github.com/uozi-tech/cosy/router"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestControllerAccountRoutesIgnoreSelectedRemoteNode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		model.Use(nil)
	})
	cSettings.AppSettings.JwtSecret = "router-test-secret"

	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.Node{}))
	model.Use(database)
	query.SetDefault(database)

	currentUser := &model.User{
		Model:  model.Model{ID: 1},
		Name:   "admin",
		Status: true,
	}
	require.NoError(t, database.Create(currentUser).Error)
	payload, err := internaluser.GenerateJWT(currentUser)
	require.NoError(t, err)

	cosyRouter.Init()
	InitRouter()

	request := httptest.NewRequest(http.MethodGet, "/api/2fa_status", nil)
	request.Header.Set("Authorization", payload.Token)
	request.Header.Set("X-Node-ID", "999999")
	recorder := httptest.NewRecorder()
	cosyRouter.GetEngine().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}
