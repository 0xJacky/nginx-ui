package terminal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTerminalRequiresSecureSessionForOTPUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	group := router.Group("/", func(c *gin.Context) {
		c.Set("user", &model.User{
			Model:     model.Model{ID: 1},
			Name:      "otp",
			Status:    true,
			OTPSecret: []byte("otp-enabled"),
		})
		c.Next()
	})
	InitRouter(group)

	req := httptest.NewRequest(http.MethodGet, "/pty", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
