package user

import "github.com/gin-gonic/gin"

func InitAuthRouter(r *gin.RouterGroup)  {
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
