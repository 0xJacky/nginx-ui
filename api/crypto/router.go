package crypto

import "github.com/gin-gonic/gin"

func InitPublicRouter(r *gin.RouterGroup) {
	g := r.Group("/crypto")
	{
		g.POST("public_key", GetPublicKey)
	}
}
