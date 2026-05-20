package host

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/host/setup"
	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// Preview renders all snippets from the posted SetupParams (or current
// settings if body is empty). Does not persist anything.
func Preview(c *gin.Context) {
	var p setup.SetupParams
	if err := c.ShouldBindJSON(&p); err != nil {
		p = setup.ParamsFromSettings()
	}
	r, err := setup.RenderAll(p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

type keypairResponse struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key,omitempty"`
}

// GenerateKeypair creates a fresh ed25519 keypair, writes the private key to
// HostPrivateKeyPath, returns the public key. The private key is also returned
// once for the caller to display/download — never returned by GetPublicKey().
func GenerateKeypair(c *gin.Context) {
	path := settings.NginxSettings.HostPrivateKeyPath
	if path == "" {
		path = "/etc/nginx-ui/host_key"
	}
	pub, err := setup.GenerateKeypair(path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	priv, _ := os.ReadFile(path)
	c.JSON(http.StatusOK, keypairResponse{PublicKey: pub, PrivateKey: string(priv)})
}

func GetPublicKey(c *gin.Context) {
	path := settings.NginxSettings.HostPrivateKeyPath
	pub, err := setup.LoadPublicKey(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"public_key": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"public_key": pub})
}

func DeleteKeypair(c *gin.Context) {
	path := settings.NginxSettings.HostPrivateKeyPath
	if path == "" {
		c.JSON(http.StatusNoContent, nil)
		return
	}
	_ = os.Remove(path)
	c.JSON(http.StatusNoContent, nil)
}

type verifyRequest struct {
	SkipNginxT bool `json:"skip_nginx_t"`
}

func Verify(c *gin.Context) {
	var req verifyRequest
	_ = c.ShouldBindJSON(&req)

	client, err := setup.NewClientFromSettings()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result := setup.Verify(ctx, setup.VerifyOptions{
		Client:     client,
		Params:     setup.ParamsFromSettings(),
		SkipNginxT: req.SkipNginxT,
	})
	c.JSON(http.StatusOK, result)
}

type knownHostRequest struct {
	HostAddress string `json:"host_address" binding:"required"`
	Fingerprint string `json:"fingerprint"  binding:"required"`
	PublicKey   string `json:"public_key"   binding:"required"`
}

// TrustHostKey appends a known_hosts entry after the user confirms a fingerprint.
func TrustHostKey(c *gin.Context) {
	var req knownHostRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	path := settings.NginxSettings.HostKnownHostsPath
	if path == "" {
		path = "/etc/nginx-ui/known_hosts"
	}
	if err := hostssh.TrustHostKey(path, req.HostAddress, req.PublicKey); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "trusted"})
}
