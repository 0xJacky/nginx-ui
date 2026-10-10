package user

import (
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/passkey"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
)

const MFARequired = 197
const mfaPreAuthTTL = 10 * time.Minute
const mfaPreAuthPrefix = "mfa:pre-auth:"

type mfaPreAuth struct {
	sync.Mutex
	UserID      uint64
	Version     uint64
	Cookie      string
	Source      user.LoginProof
	ExpiresAt   time.Time
	Secret      string
	WebAuthn    *webauthn.SessionData
	Registering bool
	Attempts    int
	Consumed    bool
}

func beginRequiredMFA(c *gin.Context, u *model.User, proof user.LoginProof) bool {
	stage, err := user.LoginMFAStage(u, proof)
	if err != nil {
		cosy.ErrHandler(c, err)
		return true
	}
	if stage == "" {
		return false
	}
	middleware.EnsureSecureSessionCookie(c)
	cookie, err := c.Cookie(middleware.SecureSessionCookieNameForRequest(c))
	if err != nil {
		cosy.ErrHandler(c, err)
		return true
	}
	id := uuid.NewString()
	expiresAt := time.Now().Add(mfaPreAuthTTL).Truncate(time.Second)
	cache.Set(mfaPreAuthPrefix+id, &mfaPreAuth{
		UserID: u.ID, Version: u.MFAVersion, Cookie: cookie, Source: proof,
		ExpiresAt: expiresAt,
	}, time.Until(expiresAt))
	audit.MarkSensitiveResponse(c)
	if proof == user.LoginProofExternal && c.Request.Method == http.MethodGet {
		c.Redirect(http.StatusFound, buildOIDCFrontendLoginRedirectWithQuery("mfa_pre_auth", id))
		return true
	}
	c.JSON(http.StatusOK, LoginResponse{Code: MFARequired, Message: "MFA is required", PreAuthID: id, MFAStage: stage})
	return true
}

// withMFAPreAuth grants only enrollment/verification capabilities, never API access.
func withMFAPreAuth(c *gin.Context, action func(*mfaPreAuth, *model.User, string, string)) {
	audit.MarkSensitiveRequest(c)
	audit.MarkSensitiveResponse(c)
	ban := query.BanIP
	count, err := ban.Where(ban.IP.Eq(c.ClientIP()), ban.ExpiredAt.Gte(time.Now().Unix()), ban.Attempts.Gte(settings.AuthSettings.MaxAttempts)).Count()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if count > 0 {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, LoginResponse{Code: ErrMaxAttempts, Message: "Max attempts"})
		return
	}
	id := c.GetHeader("X-MFA-Pre-Auth-ID")
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	value, ok := cache.Get(mfaPreAuthPrefix + id)
	session, valid := value.(*mfaPreAuth)
	if !ok || !valid {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	session.Lock()
	defer session.Unlock()
	cookie, err := c.Cookie(middleware.SecureSessionCookieNameForRequest(c))
	if err != nil || cookie != session.Cookie || session.Consumed || !time.Now().Before(session.ExpiresAt) || session.Attempts >= 10 {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	unlock := user.LockMFA(session.UserID)
	defer unlock()
	var u model.User
	if err := model.UseDB().First(&u, session.UserID).Error; err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if !u.Status || u.MFAVersion != session.Version {
		cosy.ErrHandler(c, user.ErrSessionNotFound)
		return
	}
	enabled, err := user.HasUsableMFA(&u)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	stage := "setup"
	if enabled {
		stage = "verify"
	}
	// Attach identity for audit only; this route never runs AuthRequired.
	c.Set("user", &u)
	action(session, &u, id, stage)
}

func rejectMFAVerification(c *gin.Context, err error) {
	user.BanIP(c.ClientIP())
	cosy.ErrHandler(c, err)
}

func MFAPreAuthStatus(c *gin.Context) {
	withMFAPreAuth(c, func(s *mfaPreAuth, u *model.User, _ string, stage string) {
		enabledPasskey, err := u.EnabledPasskey()
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"mfa_stage": stage, "otp_status": u.EnabledOTP(),
			"passkey_status": enabledPasskey && passkey.Enabled(), "passkey_available": passkey.Enabled(),
			"recovery_codes_generated": u.RecoveryCodeGenerated(), "expires_at": s.ExpiresAt.Unix(),
		})
	})
}

func completeMFAPreAuth(c *gin.Context, s *mfaPreAuth, u *model.User, id string, proof user.LoginProof, codes *model.RecoveryCodes) {
	token, err := user.IssueLoginToken(u, proof)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	s.Consumed = true
	cache.Del(mfaPreAuthPrefix + id)
	ban := query.BanIP
	_, _ = ban.Where(ban.IP.Eq(c.ClientIP())).Delete()
	ssid := user.SetSecureSessionID(u.ID, u.MFAVersion)
	c.JSON(http.StatusOK, struct {
		LoginResponse
		RecoveryCodes *model.RecoveryCodes `json:"recovery_codes,omitempty"`
	}{LoginResponse: LoginResponse{
		Code: LoginSuccess, Message: "ok", AccessTokenPayload: token,
		SecureSessionID: ssid, SecureSessionTTL: int(user.SecureSessionDuration().Seconds()),
	}, RecoveryCodes: codes})
}

func initMFAPreAuthRouter(r *gin.RouterGroup) {
	g := r.Group("/mfa/pre_auth")
	g.GET("/status", MFAPreAuthStatus)
	g.POST("/totp/begin", BeginMFATOTP)
	g.POST("/totp/finish", FinishMFATOTP)
	g.POST("/otp", VerifyMFAPreAuthOTP)
	g.POST("/passkey/begin", BeginMFAPasskey)
	g.POST("/passkey/finish", FinishMFAPasskey)
}

func allowMFAEnrollment(c *gin.Context) bool {
	return !middleware.BlockInDemo(c)
}
