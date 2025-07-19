package crypto

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
)

// GetPublicKey generates a new ED25519 key pair and registers it in the cache
func GetPublicKey(c *gin.Context) {
	var data struct {
		Timestamp   int64  `json:"timestamp" binding:"required"`
		Fingerprint string `json:"fingerprint" binding:"required"`
	}

	if !cosy.BindAndValid(c, &data) {
		return
	}

	if time.Now().Unix()-data.Timestamp > 10 {
		cosy.ErrHandler(c, crypto.ErrTimeout)
		return
	}

	params, err := crypto.GetCryptoParams()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_key": params.PublicKey,
		"request_id": uuid.NewString(),
	})
}
