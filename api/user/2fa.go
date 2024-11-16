package user

import (
	"encoding/base64"
	"fmt"
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/passkey"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
	"net/http"
	"strings"
	"time"
)

type Status2FA struct {
	Enabled       bool `json:"enabled"`
	OTPStatus     bool `json:"otp_status"`
	PasskeyStatus bool `json:"passkey_status"`
}

func get2FAStatus(c *gin.Context) (status Status2FA) {
	// when accessing the node from the main cluster, there is no user in the context
	u, ok := c.Get("user")
	if ok {
		userPtr := u.(*model.User)
		status.OTPStatus = userPtr.EnabledOTP()
		status.PasskeyStatus = userPtr.EnabledPasskey() && passkey.Enabled()
		status.Enabled = status.OTPStatus || status.PasskeyStatus
	}
	return
}

func Get2FAStatus(c *gin.Context) {
	c.JSON(http.StatusOK, get2FAStatus(c))
}

func SecureSessionStatus(c *gin.Context) {
	status2FA := get2FAStatus(c)
	if !status2FA.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
		})
		return
	}

	ssid := c.GetHeader("X-Secure-Session-ID")
	if ssid == "" {
		ssid = c.Query("X-Secure-Session-ID")
	}
	if ssid == "" {
		c.JSON(http.StatusOK, gin.H{
			"status": false,
		})
		return
	}

	u := api.CurrentUser(c)

	c.JSON(http.StatusOK, gin.H{
		"status": user.VerifySecureSessionID(ssid, u.ID),
	})
}

func Start2FASecureSessionByOTP(c *gin.Context) {
	var json struct {
		OTP          string `json:"otp"`
		RecoveryCode string `json:"recovery_code"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}
	u := api.CurrentUser(c)
	if !u.EnabledOTP() {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "User has not configured OTP as 2FA",
		})
		return
	}

	if json.OTP == "" && json.RecoveryCode == "" {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Message: "The user has enabled OTP as 2FA",
		})
		return
	}

	if err := user.VerifyOTP(u, json.OTP, json.RecoveryCode); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Message: "Invalid OTP or recovery code",
		})
		return
	}

	sessionId := user.SetSecureSessionID(u.ID)

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionId,
	})
}

func BeginStart2FASecureSessionByPasskey(c *gin.Context) {
	if !passkey.Enabled() {
		api.ErrHandler(c, fmt.Errorf("WebAuthn settings are not configured"))
		return
	}
	webauthnInstance := passkey.GetInstance()
	u := api.CurrentUser(c)
	options, sessionData, err := webauthnInstance.BeginLogin(u)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	passkeySessionID := uuid.NewString()
	cache.Set(passkeySessionID, sessionData, passkeyTimeout)
	c.JSON(http.StatusOK, gin.H{
		"session_id": passkeySessionID,
		"options":    options,
	})
}

func FinishStart2FASecureSessionByPasskey(c *gin.Context) {
	if !passkey.Enabled() {
		api.ErrHandler(c, fmt.Errorf("WebAuthn settings are not configured"))
		return
	}
	passkeySessionID := c.GetHeader("X-Passkey-Session-ID")
	sessionDataBytes, ok := cache.Get(passkeySessionID)
	if !ok {
		api.ErrHandler(c, fmt.Errorf("session not found"))
		return
	}
	sessionData := sessionDataBytes.(*webauthn.SessionData)
	webauthnInstance := passkey.GetInstance()
	u := api.CurrentUser(c)
	credential, err := webauthnInstance.FinishLogin(u, *sessionData, c.Request)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	rawID := strings.TrimRight(base64.StdEncoding.EncodeToString(credential.ID), "=")
	p := query.Passkey
	_, _ = p.Where(p.RawID.Eq(rawID)).Updates(&model.Passkey{
		LastUsedAt: time.Now().Unix(),
	})

	sessionId := user.SetSecureSessionID(u.ID)

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionId,
	})
}
