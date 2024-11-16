package user

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"
)

var mutex = &sync.Mutex{}

type LoginUser struct {
	Name         string `json:"name" binding:"required,max=255"`
	Password     string `json:"password" binding:"required,max=255"`
	OTP          string `json:"otp"`
	RecoveryCode string `json:"recovery_code"`
}

const (
	ErrPasswordIncorrect = 4031
	ErrMaxAttempts       = 4291
	ErrUserBanned        = 4033
	Enabled2FA           = 199
	Error2FACode         = 4034
	LoginSuccess         = 200
)

type LoginResponse struct {
	Message         string `json:"message"`
	Error           string `json:"error,omitempty"`
	Code            int    `json:"code"`
	Token           string `json:"token,omitempty"`
	SecureSessionID string `json:"secure_session_id,omitempty"`
}

func Login(c *gin.Context) {
	// make sure that only one request is processed at a time
	mutex.Lock()
	defer mutex.Unlock()
	// check if the ip is banned
	clientIP := c.ClientIP()
	b := query.BanIP
	banIP, _ := b.Where(b.IP.Eq(clientIP),
		b.ExpiredAt.Gte(time.Now().Unix()),
		b.Attempts.Gte(settings.AuthSettings.MaxAttempts),
	).Count()

	if banIP > 0 {
		c.JSON(http.StatusTooManyRequests, LoginResponse{
			Message: "Max attempts",
			Code:    ErrMaxAttempts,
		})
		return
	}

	var json LoginUser
	ok := cosy.BindAndValid(c, &json)
	if !ok {
		return
	}

	u, err := user.Login(json.Name, json.Password)
	if err != nil {
		random := time.Duration(rand.Int() % 10)
		time.Sleep(random * time.Second)
		switch {
		case errors.Is(err, user.ErrPasswordIncorrect):
			c.JSON(http.StatusForbidden, LoginResponse{
				Message: "Password incorrect",
				Code:    ErrPasswordIncorrect,
			})
		case errors.Is(err, user.ErrUserBanned):
			c.JSON(http.StatusForbidden, LoginResponse{
				Message: "The user is banned",
				Code:    ErrUserBanned,
			})
		default:
			api.ErrHandler(c, err)
		}
		user.BanIP(clientIP)
		return
	}

	// Check if the user enables 2FA
	var secureSessionID string

	if u.EnabledOTP() {
		if json.OTP == "" && json.RecoveryCode == "" {
			c.JSON(http.StatusOK, LoginResponse{
				Message: "The user has enabled 2FA",
				Code:    Enabled2FA,
			})
			user.BanIP(clientIP)
			return
		}

		if err = user.VerifyOTP(u, json.OTP, json.RecoveryCode); err != nil {
			c.JSON(http.StatusForbidden, LoginResponse{
				Message: "Invalid 2FA or recovery code",
				Code:    Error2FACode,
			})
			user.BanIP(clientIP)
			return
		}

		secureSessionID = user.SetSecureSessionID(u.ID)
	}

	// login success, clear banned record
	_, _ = b.Where(b.IP.Eq(clientIP)).Delete()

	logger.Info("[User Login]", u.Name)
	token, err := user.GenerateJWT(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Code:            LoginSuccess,
		Message:         "ok",
		Token:           token,
		SecureSessionID: secureSessionID,
	})
}

func Logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token != "" {
		user.DeleteToken(token)
	}
	c.JSON(http.StatusNoContent, nil)
}
