package host

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	g := r.Group("host/setup")
	{
		g.GET("preview", Preview)
		g.POST("preview", Preview)
		g.POST("keypair", GenerateKeypair)
		g.GET("publickey", GetPublicKey)
		g.DELETE("keypair", DeleteKeypair)
		g.POST("verify", Verify)
		g.POST("known-host", TrustHostKey)
		g.POST("host-key/scan", ScanHostKey)
		g.POST("host-key/trust", TrustScannedHostKey)
		g.POST("host-key/replace", ReplaceHostKey)
		g.DELETE("host-key", DeleteHostKey)
	}
}
