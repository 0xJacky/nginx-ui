package host

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/host/setup"
	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	gossh "golang.org/x/crypto/ssh"
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

type hostKeyScanRequest struct {
	HostAddress   string `json:"host_address" binding:"required"`
	KeyscanOutput string `json:"keyscan_output"`
}

type hostKeyTrustRequest struct {
	HostAddress string `json:"host_address" binding:"required"`
	Algorithm   string `json:"algorithm" binding:"required"`
	Fingerprint string `json:"fingerprint" binding:"required"`
	PublicKey   string `json:"public_key" binding:"required"`
	Confirmed   bool   `json:"confirmed"`
}

type hostKeyReplaceRequest struct {
	HostAddress    string `json:"host_address" binding:"required"`
	Algorithm      string `json:"algorithm" binding:"required"`
	OldFingerprint string `json:"old_fingerprint" binding:"required"`
	NewFingerprint string `json:"new_fingerprint" binding:"required"`
	PublicKey      string `json:"public_key" binding:"required"`
	Confirmed      bool   `json:"confirmed"`
}

type hostKeyDeleteRequest struct {
	HostAddress string `json:"host_address" binding:"required"`
	Algorithm   string `json:"algorithm" binding:"required"`
	Fingerprint string `json:"fingerprint" binding:"required"`
	Confirmed   bool   `json:"confirmed"`
}

func hostKnownHostsPath() string {
	return settings.NginxSettings.GetHostKnownHostsPath()
}

func parseAndVerifyPublicKey(publicKey, fingerprint string) (gossh.PublicKey, error) {
	parsed, _, _, _, err := gossh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return nil, cosy.WrapErrorWithParams(hostssh.ErrPublicKeyParse, err.Error())
	}
	actual := gossh.FingerprintSHA256(parsed)
	if fingerprint != actual {
		return nil, cosy.WrapErrorWithParams(hostssh.ErrHostKeyMismatch, fingerprint, actual)
	}
	return parsed, nil
}

// TrustHostKey appends a known_hosts entry after the user confirms a fingerprint.
// It recomputes the SHA256 fingerprint of the submitted public key and rejects
// requests where the client-provided fingerprint does not match.
func TrustHostKey(c *gin.Context) {
	var req knownHostRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	if _, err := parseAndVerifyPublicKey(req.PublicKey, req.Fingerprint); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if err := hostssh.TrustHostKey(hostKnownHostsPath(), req.HostAddress, req.PublicKey); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "trusted"})
}

func ScanHostKey(c *gin.Context) {
	var req hostKeyScanRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	path := hostKnownHostsPath()
	kh, err := hostssh.NewKnownHosts(path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	var keys []gossh.PublicKey
	if req.KeyscanOutput != "" {
		keys, err = hostssh.ParseSSHKeyscanOutput(req.KeyscanOutput)
	} else {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		keys, err = hostssh.ScanHostKeys(ctx, req.HostAddress, 10*time.Second)
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	result, err := hostssh.ClassifyHostKeys(req.HostAddress, keys, kh)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	result.KnownHostsPath = path
	result.Persistence = hostssh.KnownHostsPersistence{
		Path:        path,
		Recommended: strings.HasPrefix(path, "/etc/nginx-ui/"),
	}
	if !result.Persistence.Recommended {
		result.Persistence.Warning = "known_hosts is outside /etc/nginx-ui; make sure it is persisted across container rebuilds"
	}
	c.JSON(http.StatusOK, result)
}

func TrustScannedHostKey(c *gin.Context) {
	var req hostKeyTrustRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	if !req.Confirmed {
		cosy.ErrHandler(c, hostssh.ErrHostKeyConfirmRequired)
		return
	}
	parsed, err := parseAndVerifyPublicKey(req.PublicKey, req.Fingerprint)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if parsed.Type() != req.Algorithm {
		c.JSON(http.StatusBadRequest, gin.H{"message": "algorithm mismatch", "expected": parsed.Type(), "got": req.Algorithm})
		return
	}
	if err := hostssh.TrustHostKey(hostKnownHostsPath(), req.HostAddress, req.PublicKey); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "trusted"})
}

func ReplaceHostKey(c *gin.Context) {
	var req hostKeyReplaceRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	if !req.Confirmed {
		cosy.ErrHandler(c, hostssh.ErrHostKeyConfirmRequired)
		return
	}
	parsed, err := parseAndVerifyPublicKey(req.PublicKey, req.NewFingerprint)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if parsed.Type() != req.Algorithm {
		c.JSON(http.StatusBadRequest, gin.H{"message": "algorithm mismatch", "expected": parsed.Type(), "got": req.Algorithm})
		return
	}
	if err := hostssh.ReplaceHostKey(hostKnownHostsPath(), req.HostAddress, req.OldFingerprint, req.PublicKey); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "replaced"})
}

func DeleteHostKey(c *gin.Context) {
	var req hostKeyDeleteRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	if !req.Confirmed {
		cosy.ErrHandler(c, hostssh.ErrHostKeyConfirmRequired)
		return
	}
	if err := hostssh.DeleteHostKey(hostKnownHostsPath(), req.HostAddress, req.Algorithm, req.Fingerprint); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
