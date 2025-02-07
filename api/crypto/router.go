package crypto

import "github.com/gin-gonic/gin"

func InitPublicRouter(r *gin.RouterGroup) {
	g := r.Group("/crypto")
	{
		g.GET("public_key", GetPublicKey)
	}
}
