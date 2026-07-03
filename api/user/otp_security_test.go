package user

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestEnrollTOTPRequiresCurrentPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	require.NoError(t, err)

	router := gin.New()
	router.POST("/otp_enroll", func(c *gin.Context) {
		c.Set("user", &model.User{
			Model:    model.Model{ID: 1},
			Name:     "user",
			Password: string(passwordHash),
			Status:   true,
		})
		EnrollTOTP(c)
	})

	body := bytes.NewBufferString(`{"secret":"JBSWY3DPEHPK3PXP","passcode":"000000","password":"wrong-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/otp_enroll", body)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
