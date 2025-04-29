package mcp

import (
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) {
	r.Any("/mcp", middleware.IPWhiteList(), middleware.AuthRequired(),
		func(c *gin.Context) {
			mcp.ServeHTTP(c)
		})
	r.Any("/mcp_message", middleware.IPWhiteList(),
		func(c *gin.Context) {
			mcp.ServeHTTP(c)
		})
}
