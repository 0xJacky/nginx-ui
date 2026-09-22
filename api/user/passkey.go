package user

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
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
)

const passkeyTimeout = 30 * time.Second
const currentPasswordHeader = "X-Current-Password"

const (
	passkeyRegistrationCachePrefix  = "passkey:registration:"
	passkeyPreAuthCachePrefix       = "passkey:pre-auth:"
	passkeyLoginCachePrefix         = "passkey:login:"
	passkeySecureSessionCachePrefix = "passkey:secure-session:"
)

func buildCachePasskeyRegKey(id uint64) string {
	return fmt.Sprintf("%s%d", passkeyRegistrationCachePrefix, id)
}

type passkeyPreAuthSession struct {
	UserID      uint64
	SessionData *webauthn.SessionData
}

func buildPasskeyPreAuthKey(id string) string {
	return passkeyPreAuthCachePrefix + id
}

func buildPasskeyLoginKey(id string) string {
	return passkeyLoginCachePrefix + id
}

func buildPasskeySecureSessionKey(id string) string {
	return passkeySecureSessionCachePrefix + id
}

func isCanonicalSessionID(sessionID string) bool {
	parsed, err := uuid.Parse(sessionID)
	return err == nil && parsed.String() == sessionID
}

func takeWebAuthnSession(sessionID string, buildKey func(string) string) (*webauthn.SessionData, bool) {
	if !isCanonicalSessionID(sessionID) {
		return nil, false
	}

	sessionValue, ok := cache.Take(buildKey(sessionID))
	if !ok {
		return nil, false
	}

	sessionData, ok := sessionValue.(*webauthn.SessionData)
	return sessionData, ok && sessionData != nil
}

func takePasskeyRegistrationSession(userID uint64) (*webauthn.SessionData, bool) {
	sessionValue, ok := cache.Take(buildCachePasskeyRegKey(userID))
	if !ok {
		return nil, false
	}

	sessionData, ok := sessionValue.(*webauthn.SessionData)
	return sessionData, ok && sessionData != nil
}

func beginPasskeyPreAuthentication(c *gin.Context, currentUser *model.User) {
	if !passkey.Enabled() {
		cosy.ErrHandler(c, user.ErrWebAuthnNotConfigured)
		return
	}
	options, sessionData, err := passkey.GetInstance().BeginLogin(currentUser)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	preAuthID := uuid.NewString()
	cache.Set(buildPasskeyPreAuthKey(preAuthID), &passkeyPreAuthSession{
		UserID:      currentUser.ID,
		SessionData: sessionData,
	}, passkeyTimeout)
	c.JSON(http.StatusOK, LoginResponse{
		Code:      PasskeyRequired,
		Message:   "Passkey verification is required",
		PreAuthID: preAuthID,
		Options:   options,
	})
}

func FinishPasskeyPreAuthentication(c *gin.Context) {
	if !passkey.Enabled() {
		cosy.ErrHandler(c, user.ErrWebAuthnNotConfigured)
		return
	}
	preAuthID := strings.TrimSpace(c.GetHeader("X-Passkey-Pre-Auth-ID"))
	session, ok := takePasskeyPreAuthSession(preAuthID)
	if !ok {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	userQuery := query.User
	currentUser, err := userQuery.FirstByID(session.UserID)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	credential, err := passkey.GetInstance().FinishLogin(currentUser, *session.SessionData, c.Request)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	rawID := strings.TrimRight(base64.StdEncoding.EncodeToString(credential.ID), "=")
	passkeyQuery := query.Passkey
	_, _ = passkeyQuery.Where(
		passkeyQuery.UserID.Eq(currentUser.ID),
		passkeyQuery.RawID.Eq(rawID),
	).Updates(&model.Passkey{LastUsedAt: time.Now().Unix()})

	token, err := user.IssueLoginToken(currentUser, user.LoginProofPasskey)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	banIPQuery := query.BanIP
	_, _ = banIPQuery.Where(banIPQuery.IP.Eq(c.ClientIP())).Delete()
	secureSessionID := user.SetSecureSessionID(currentUser.ID)
	middleware.EnsureSecureSessionCookie(c)
	c.JSON(http.StatusOK, LoginResponse{
		Code:               LoginSuccess,
		Message:            "ok",
		AccessTokenPayload: token,
		SecureSessionID:    secureSessionID,
		SecureSessionTTL:   int(user.SecureSessionDuration().Seconds()),
	})
}

func takePasskeyPreAuthSession(preAuthID string) (*passkeyPreAuthSession, bool) {
	if !isCanonicalSessionID(preAuthID) {
		return nil, false
	}
	key := buildPasskeyPreAuthKey(preAuthID)
	sessionValue, ok := cache.Take(key)
	if !ok {
		return nil, false
	}
	session, ok := sessionValue.(*passkeyPreAuthSession)
	return session, ok && session != nil && session.SessionData != nil
}

func GetPasskeyConfigStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": passkey.Enabled(),
	})
}

func BeginPasskeyRegistration(c *gin.Context) {
	if !verifyCurrentPassword(c, c.GetHeader(currentPasswordHeader)) {
		return
	}

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
	sessionData, ok := takePasskeyRegistrationSession(cUser.ID)
	if !ok {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}

	credential, err := webauthnInstance.FinishRegistration(cUser, *sessionData, c.Request)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
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
	cache.Set(buildPasskeyLoginKey(sessionID), sessionData, passkeyTimeout)

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
	sessionID := c.GetHeader("X-Passkey-Session-ID")
	sessionData, ok := takeWebAuthnSession(sessionID, buildPasskeyLoginKey)
	if !ok {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	webauthnInstance := passkey.GetInstance()
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
	token, err := user.IssueLoginToken(outUser, user.LoginProofPasskey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Message: err.Error(),
		})
		return
	}

	secureSessionID := user.SetSecureSessionID(outUser.ID)

	middleware.EnsureSecureSessionCookie(c)

	c.JSON(http.StatusOK, LoginResponse{
		Code:               LoginSuccess,
		Message:            "ok",
		AccessTokenPayload: token,
		SecureSessionID:    secureSessionID,
		SecureSessionTTL:   int(user.SecureSessionDuration().Seconds()),
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
