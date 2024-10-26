package user

import (
	"github.com/gin-gonic/gin"
)

func InitAuthRouter(r *gin.RouterGroup) {
	r.POST("/login", Login)
	r.DELETE("/logout", Logout)

	r.GET("/begin_passkey_login", BeginPasskeyLogin)
	r.POST("/finish_passkey_login", FinishPasskeyLogin)

	r.GET("/casdoor_uri", GetCasdoorUri)
	r.POST("/casdoor_callback", CasdoorCallback)

	r.GET("/passkeys/config", GetPasskeyConfigStatus)
}

func InitManageUserRouter(r *gin.RouterGroup) {
	r.GET("users", GetUsers)
	r.GET("users/:id", GetUser)
	r.POST("users", AddUser)
	r.POST("users/:id", EditUser)
	r.DELETE("users/:id", DeleteUser)
	r.PATCH("users/:id", RecoverUser)
}

func InitUserRouter(r *gin.RouterGroup) {
	r.GET("/2fa_status", Get2FAStatus)
	r.GET("/2fa_secure_session/status", SecureSessionStatus)
	r.POST("/2fa_secure_session/otp", Start2FASecureSessionByOTP)
	r.GET("/2fa_secure_session/passkey", BeginStart2FASecureSessionByPasskey)
	r.POST("/2fa_secure_session/passkey", FinishStart2FASecureSessionByPasskey)

	r.GET("/otp_secret", GenerateTOTP)
	r.POST("/otp_enroll", EnrollTOTP)
	r.POST("/otp_reset", ResetOTP)

	r.GET("/begin_passkey_register", BeginPasskeyRegistration)
	r.POST("/finish_passkey_register", FinishPasskeyRegistration)

	r.GET("/passkeys", GetPasskeyList)
	r.POST("/passkeys/:id", UpdatePasskey)
	r.DELETE("/passkeys/:id", DeletePasskey)
}
