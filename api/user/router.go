package user

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitAuthRouter(r *gin.RouterGroup) {
	r.POST("/login", middleware.EncryptedParams(), Login)
	r.DELETE("/logout", Logout)

	r.GET("/begin_passkey_login", BeginPasskeyLogin)
	r.POST("/finish_passkey_login", FinishPasskeyLogin)
	r.POST("/finish_passkey_pre_auth", FinishPasskeyPreAuthentication)

	r.GET("/casdoor_uri", GetCasdoorUri)
	r.POST("/casdoor_callback", CasdoorCallback)

	r.GET("/oidc_uri", GetOIDCUri)
	r.GET("/oidc_callback", OIDCCallback)
	r.POST("/oidc_callback", OIDCCallback)

	r.GET("/passkeys/config", GetPasskeyConfigStatus)
}

func InitTokenRouter(r *gin.RouterGroup) {
	r.POST("/token/short", middleware.RequireInteractiveUser(), IssueShortToken)
}

func InitUserRouter(r *gin.RouterGroup) {
	interactive := r.Group("", middleware.RequireInteractiveUser())
	interactive.GET("/2fa_status", Get2FAStatus)
	interactive.GET("/2fa_secure_session/status", SecureSessionStatus)
	interactive.POST("/2fa_secure_session/otp", Start2FASecureSessionByOTP)
	interactive.GET("/2fa_secure_session/passkey", BeginStart2FASecureSessionByPasskey)
	interactive.POST("/2fa_secure_session/passkey", FinishStart2FASecureSessionByPasskey)

	interactive.GET("/passkeys", GetPasskeyList)

	o := interactive.Group("", middleware.RequireSecureSession())
	{
		o.GET("/otp_secret", GenerateTOTP)
		o.POST("/otp_enroll", EnrollTOTP)
		o.GET("/otp_reset", ResetOTP)

		o.GET("/begin_passkey_register", BeginPasskeyRegistration)
		o.POST("/finish_passkey_register", FinishPasskeyRegistration)
		o.POST("/passkeys/:id", UpdatePasskey)
		o.DELETE("/passkeys/:id", DeletePasskey)

		o.GET("/recovery_codes", ViewRecoveryCodes)
		o.GET("/recovery_codes_generate", GenerateRecoveryCodes)
	}

	interactive.GET("/user", GetCurrentUser)
	interactive.POST("/user", middleware.RequireSecureSession(), UpdateCurrentUser)
	interactive.POST("/user/password", middleware.RequireSecureSession(), middleware.RejectInDemo(), UpdateCurrentUserPassword)
	interactive.POST("/user/language", UpdateCurrentUserLanguage)
}
