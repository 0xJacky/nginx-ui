package user

import (
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// loginFailureDelay is how long a failed password login waits before it
// answers. It slows down the client that failed, and runs after the client's
// login lock is released so no other request waits for it.
var loginFailureDelay = func() time.Duration {
	return time.Duration(rand.IntN(10)) * time.Second
}

type LoginUser struct {
	Name         string `json:"name" binding:"required,max=255"`
	Password     string `json:"password" binding:"required,max=255"`
	OTP          string `json:"otp"`
	RecoveryCode string `json:"recovery_code"`
}

const (
	ErrMaxAttempts  = 4291
	PasskeyRequired = 198
	Enabled2FA      = 199
	Error2FACode    = 4034
	LoginSuccess    = 200
)

type LoginResponse struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Code    int    `json:"code"`
	*user.AccessTokenPayload
	SecureSessionID  string `json:"secure_session_id,omitempty"`
	SecureSessionTTL int    `json:"secure_session_ttl,omitempty"`
	PreAuthID        string `json:"pre_auth_id,omitempty"`
	Options          any    `json:"options,omitempty"`
	MFAStage         string `json:"mfa_stage,omitempty"`
}

// loginClientBanned reports whether clientIP has used up its login attempts
// in the current ban window.
func loginClientBanned(clientIP string) bool {
	b := query.BanIP
	banIP, _ := b.Where(b.IP.Eq(clientIP),
		b.ExpiredAt.Gte(time.Now().Unix()),
		b.Attempts.Gte(settings.AuthSettings.MaxAttempts),
	).Count()
	return banIP > 0
}

func respondMaxAttempts(c *gin.Context) {
	c.JSON(http.StatusTooManyRequests, LoginResponse{
		Message: "Max attempts",
		Code:    ErrMaxAttempts,
	})
}

func Login(c *gin.Context) {
	clientIP := c.ClientIP()
	// Refuse a banned client before it waits for any lock.
	if loginClientBanned(clientIP) {
		respondMaxAttempts(c)
		return
	}

	var json LoginUser
	ok := cosy.BindAndValid(c, &json)
	if !ok {
		return
	}

	// Check the ban again under the client's lock, so the check, the
	// credential verification and the recorded failure act as one step for
	// this client. Other clients use their own locks and never wait here.
	unlock := loginClientLocks.lock(clientIP)
	defer unlock()
	if loginClientBanned(clientIP) {
		respondMaxAttempts(c)
		return
	}

	u, err := user.Login(json.Name, json.Password)
	if err != nil {
		user.BanIP(clientIP)
		event.PublishAuthLoginFailed(json.Name, clientIP)
		// The failure is recorded; release the lock before the delay.
		unlock()
		time.Sleep(loginFailureDelay())
		cosy.ErrHandler(c, err)
		return
	}

	if u.MFAPending || json.OTP == "" && json.RecoveryCode == "" {
		if beginRequiredMFA(c, u, user.LoginProofPassword) {
			return
		}
	}
	// Check if the user enables 2FA
	var secureSessionID string
	var secureSessionTTL int

	loginProof := user.LoginProofPassword
	if u.EnabledOTP() || (u.RecoveryCodeGenerated() && json.RecoveryCode != "") {
		if json.OTP == "" && json.RecoveryCode == "" {
			c.JSON(http.StatusOK, LoginResponse{
				Message: "The user has enabled 2FA",
				Code:    Enabled2FA,
			})
			user.BanIP(clientIP)
			return
		}

		if _, err = user.VerifyOTP(u, json.OTP, json.RecoveryCode); err != nil {
			cosy.ErrHandler(c, err)
			user.BanIP(clientIP)
			event.PublishAuthLoginFailed(u.Name, clientIP)
			return
		}

		secureSessionID = user.SetSecureSessionID(u.ID, u.MFAVersion)
		secureSessionTTL = int(user.SecureSessionDuration().Seconds())
		loginProof = user.LoginProofOTP
	} else {
		enabledPasskey, passkeyErr := u.EnabledPasskey()
		if passkeyErr != nil {
			cosy.ErrHandler(c, passkeyErr)
			return
		}
		if enabledPasskey {
			beginPasskeyPreAuthentication(c, u)
			return
		}
	}

	// login success, clear banned record
	user.ClearBanIP(clientIP)

	logger.Info("[User Login]", u.Name)
	accessToken, err := user.IssueLoginToken(u, loginProof)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	middleware.EnsureSecureSessionCookie(c)

	c.JSON(http.StatusOK, LoginResponse{
		Code:               LoginSuccess,
		Message:            "ok",
		AccessTokenPayload: accessToken,
		SecureSessionID:    secureSessionID,
		SecureSessionTTL:   secureSessionTTL,
	})
}

func Logout(c *gin.Context) {
	token := middleware.AuthorizationToken(c.GetHeader("Authorization"))
	var err error
	if len(token) <= 16 {
		err = user.RevokeShortToken(token)
	} else {
		err = user.RevokeSessionToken(token)
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
