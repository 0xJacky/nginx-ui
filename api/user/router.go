package user

import (
    "github.com/gin-gonic/gin"
)

func InitAuthRouter(r *gin.RouterGroup) {
	r.POST("/login", Login)
	r.DELETE("/logout", Logout)

	r.GET("/casdoor_uri", GetCasdoorUri)
	r.POST("/casdoor_callback", CasdoorCallback)
}

func InitManageUserRouter(r *gin.RouterGroup) {
	r.GET("users", GetUsers)
	r.GET("user/:id", GetUser)
	r.POST("user", AddUser)
	r.POST("user/:id", EditUser)
	r.DELETE("user/:id", DeleteUser)
}

func InitUserRouter(r *gin.RouterGroup) {
	r.GET("/otp_status", OTPStatus)
	r.GET("/otp_secret", GenerateTOTP)
	r.POST("/otp_enroll", EnrollTOTP)
	r.POST("/otp_reset", ResetOTP)

    r.GET("/otp_secure_session_status", SecureSessionStatus)
	r.POST("/otp_secure_session", StartSecure2FASession)
}
