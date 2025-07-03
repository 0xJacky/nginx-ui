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
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"
)

const passkeyTimeout = 30 * time.Second

func buildCachePasskeyRegKey(id uint64) string {
	return fmt.Sprintf("passkey-reg-%d", id)
}

func GetPasskeyConfigStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": passkey.Enabled(),
	})
}

func BeginPasskeyRegistration(c *gin.Context) {
	u := api.CurrentUser(c)

	webauthnInstance := passkey.GetInstance()

	options, sessionData, err := webauthnInstance.BeginRegistration(u)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	cache.Set(buildCachePasskeyRegKey(u.ID), sessionData, passkeyTimeout)

	c.JSON(http.StatusOK, options.Response)
}

func FinishPasskeyRegistration(c *gin.Context) {
	cUser := api.CurrentUser(c)
	webauthnInstance := passkey.GetInstance()
	sessionDataBytes, ok := cache.Get(buildCachePasskeyRegKey(cUser.ID))
	if !ok {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}

	sessionData := sessionDataBytes.(*webauthn.SessionData)
	credential, err := webauthnInstance.FinishRegistration(cUser, *sessionData, c.Request)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	cache.Del(buildCachePasskeyRegKey(cUser.ID))

	rawId := strings.TrimRight(base64.StdEncoding.EncodeToString(credential.ID), "=")
	passkeyName := c.Query("name")
	p := query.Passkey
	err = p.Create(&model.Passkey{
		UserID:     cUser.ID,
		Name:       passkeyName,
		RawID:      rawId,
		Credential: credential,
		LastUsedAt: time.Now().Unix(),
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func BeginPasskeyLogin(c *gin.Context) {
	if !passkey.Enabled() {
		cosy.ErrHandler(c, user.ErrWebAuthnNotConfigured)
		return
	}
	webauthnInstance := passkey.GetInstance()
	options, sessionData, err := webauthnInstance.BeginDiscoverableLogin()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	sessionID := uuid.NewString()
	cache.Set(sessionID, sessionData, passkeyTimeout)

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"options":    options,
	})
}

func FinishPasskeyLogin(c *gin.Context) {
	if !passkey.Enabled() {
		cosy.ErrHandler(c, user.ErrWebAuthnNotConfigured)
		return
	}
	sessionId := c.GetHeader("X-Passkey-Session-ID")
	sessionDataBytes, ok := cache.Get(sessionId)
	if !ok {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	webauthnInstance := passkey.GetInstance()
	sessionData := sessionDataBytes.(*webauthn.SessionData)
	var outUser *model.User
	_, err := webauthnInstance.FinishDiscoverableLogin(
		func(rawID, userHandle []byte) (user webauthn.User, err error) {
			encodeRawID := strings.TrimRight(base64.StdEncoding.EncodeToString(rawID), "=")
			u := query.User
			logger.Debug("[WebAuthn] Discoverable Login", cast.ToInt(string(userHandle)))

			p := query.Passkey
			_, _ = p.Where(p.RawID.Eq(encodeRawID)).Updates(&model.Passkey{
				LastUsedAt: time.Now().Unix(),
			})

			outUser, err = u.FirstByID(cast.ToUint64(string(userHandle)))
			return outUser, err
		}, *sessionData, c.Request)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	b := query.BanIP
	clientIP := c.ClientIP()
	// login success, clear banned record
	_, _ = b.Where(b.IP.Eq(clientIP)).Delete()

	logger.Info("[User Login]", outUser.Name)
	token, err := user.GenerateJWT(outUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Message: err.Error(),
		})
		return
	}

	secureSessionID := user.SetSecureSessionID(outUser.ID)

	c.JSON(http.StatusOK, LoginResponse{
		Code:            LoginSuccess,
		Message:         "ok",
		AccessTokenPayload: token,
		SecureSessionID:    secureSessionID,
	})
}

func GetPasskeyList(c *gin.Context) {
	u := api.CurrentUser(c)
	p := query.Passkey
	passkeys, err := p.Where(p.UserID.Eq(u.ID)).Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if len(passkeys) == 0 {
		passkeys = make([]*model.Passkey, 0)
	}

	c.JSON(http.StatusOK, passkeys)
}

func UpdatePasskey(c *gin.Context) {
	u := api.CurrentUser(c)
	cosy.Core[model.Passkey](c).
		SetValidRules(gin.H{
			"name": "required",
		}).GormScope(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("user_id", u.ID)
	}).Modify()
}

func DeletePasskey(c *gin.Context) {
	u := api.CurrentUser(c)
	cosy.Core[model.Passkey](c).
		GormScope(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("user_id", u.ID)
		}).PermanentlyDelete()
}
