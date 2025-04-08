//go:build !unembed

package router

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func initEmbedRoute(r *gin.Engine) {
	r.Use(middleware.CacheJs())

	r.Use(middleware.ServeStatic()...)
}
