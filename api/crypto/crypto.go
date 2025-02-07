package crypto

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/sign"
	"github.com/gin-gonic/gin"
)

// GetPublicKey generates a new ED25519 key pair and registers it in the cache
func GetPublicKey(c *gin.Context) {
	sign, err := sign.GetCryptoParams()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_key": sign.PublicKey,
	})
}
