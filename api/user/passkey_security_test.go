package user

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBeginPasskeyRegistrationRequiresCurrentPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/begin_passkey_register", func(c *gin.Context) {
		c.Set("user", &model.User{
			Model:    model.Model{ID: 1},
			Name:     "user",
			Password: string(passwordHash),
			Status:   true,
		})
		BeginPasskeyRegistration(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/begin_passkey_register", nil)
	req.Header.Set(currentPasswordHeader, "wrong-password")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
