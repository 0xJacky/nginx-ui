package host

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/host/setup"
	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	gossh "golang.org/x/crypto/ssh"
)

var resetSSHClient = nginx.ResetHostNginxState

// Preview renders all snippets from the posted SetupParams (or current
// settings if body is empty). Does not persist anything.
// bindSetupParams reads the posted params. An absent body deliberately means
// "use the saved settings"; a malformed body is an error, not a fallback.
func bindSetupParams(c *gin.Context, p *setup.SetupParams) bool {
	if c.Request.ContentLength == 0 {
		*p = setup.ParamsFromSettings()
		return true
	}
	if err := c.ShouldBindJSON(p); err != nil {
		if errors.Is(err, io.EOF) {
			*p = setup.ParamsFromSettings()
			return true
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return false
	}
	return true
}

func Preview(c *gin.Context) {
	var p setup.SetupParams
	if !bindSetupParams(c, &p) {
		return
	}
	r, err := setup.RenderAll(p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

// abortBadRequest answers a validation failure with 400 while keeping the
// cosy error code and message the frontend maps to a translated string.
func abortBadRequest(c *gin.Context, err error) {
	var cErr *cosy.Error
	if errors.As(err, &cErr) {
		c.AbortWithStatusJSON(http.StatusBadRequest, cErr)
		return
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
}

// isManagedPrivateKeyPath reports whether path is one of the two locations
// Nginx UI itself manages: the built-in default and the configured key path.
func isManagedPrivateKeyPath(path string) bool {
	configuredPath := filepath.Clean(settings.NginxSettings.GetHostPrivateKeyPath())
	return path == settings.DefaultHostPrivateKeyPath || path == configuredPath
}

type keypairResponse struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key,omitempty"`
}

type keyPathRequest struct {
	PrivateKeyPath string `json:"private_key_path"`
}

func normalizePrivateKeyPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("private key path is required")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("private key path must be absolute")
	}
	return filepath.Clean(path), nil
}

// GenerateKeypair creates a fresh ed25519 keypair, writes the private key to
// HostPrivateKeyPath, returns the public key. The private key is also returned
// once for the caller to display/download — never returned by GetPublicKey().
func GenerateKeypair(c *gin.Context) {
	var req keyPathRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "invalid key path request"})
			return
		}
	}
	path := req.PrivateKeyPath
	if path == "" {
		path = settings.NginxSettings.GetHostPrivateKeyPath()
	}
	path, err := normalizePrivateKeyPath(path)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if !isManagedPrivateKeyPath(path) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "generated keys may only use the default or configured private key path"})
		return
	}
	pub, err := setup.GenerateKeypair(path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	// The one-time private key is the only copy the operator gets, so a failed
	// read must not be rendered as a clean 200 with the field simply absent.
	priv, err := os.ReadFile(path)
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(setup.ErrKeyfileRead, path, err.Error()))
		return
	}
	c.JSON(http.StatusOK, keypairResponse{PublicKey: pub, PrivateKey: string(priv)})
}

func GetPublicKey(c *gin.Context) {
	path := c.Query("private_key_path")
	if path == "" {
		path = settings.NginxSettings.GetHostPrivateKeyPath()
	}
	path, err := normalizePrivateKeyPath(path)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	// The wizard's "existing key" flow reads a key the operator has typed but
	// not saved yet, so the path cannot be limited to the managed locations.
	// Any other path is a file-existence oracle and a public key reader for
	// the whole container, so it needs the same verified session as saving
	// that path into the settings would.
	if !isManagedPrivateKeyPath(path) && !middleware.VerifiedTwoFactorOrProxy(c, hostSetupTwoFactorMessage) {
		return
	}
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		// Nothing there yet. The wizard treats this as a normal starting state.
		c.JSON(http.StatusNotFound, gin.H{"public_key": ""})
		return
	}

	pub, err := setup.LoadPublicKey(path)
	if err != nil {
		// The file exists but cannot be used. This must not look like an empty
		// path, otherwise the wizard would offer to overwrite it without asking.
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
			"public_key": "",
			"message":    err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"public_key": pub})
}

func DeleteKeypair(c *gin.Context) {
	path, err := normalizePrivateKeyPath(settings.NginxSettings.GetHostPrivateKeyPath())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

type verifyRequest struct {
	setup.SetupParams
	SkipNginxT bool `json:"skip_nginx_t"`
	// Groups limits the run to a subset of the pipeline so an individual wizard
	// step can verify what it configured. Empty runs every check.
	Groups []string `json:"groups"`
}

type connectionTestResult struct {
	Connected bool   `json:"connected"`
	Detail    string `json:"detail"`
}

// TestConnection verifies host identity, key authentication, and remote command
// execution without running nginx or service-manager commands.
func TestConnection(c *gin.Context) {
	var p setup.SetupParams
	if !cosy.BindAndValid(c, &p) {
		return
	}
	if strings.TrimSpace(p.HostAddress) == "" || strings.TrimSpace(p.HostUser) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "host address and SSH user are required"})
		return
	}
	client, err := setup.NewClientFromParams(p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	output, err := client.Exec(ctx, "/bin/echo", "nginx-ui-ssh-ok")
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, connectionTestResult{
		Connected: true,
		Detail:    strings.TrimSpace(output),
	})
}

func Verify(c *gin.Context) {
	var req verifyRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
	}

	p := req.SetupParams
	if p.HostAddress == "" {
		p = setup.ParamsFromSettings()
	}
	client, err := setup.NewClientFromParams(p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result := setup.Verify(ctx, setup.VerifyOptions{
		Client:     client,
		Params:     p,
		SkipNginxT: req.SkipNginxT,
		Groups:     setup.ParseCheckGroups(req.Groups),
	})
	c.JSON(http.StatusOK, result)
}

// Discover locates nginx and its compiled paths on the SSH host. It only runs
// read-only version and package-prefix commands and does not persist settings.
func Discover(c *gin.Context) {
	var p setup.SetupParams
	if !bindSetupParams(c, &p) {
		return
	}
	client, err := setup.NewClientFromParams(p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := setup.DiscoverNginx(ctx, client, p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// SSHTargets probes the addresses a container can use to reach its own host,
// so the wizard can offer a reachable target instead of a guess. It opens no
// SSH session and needs no credentials.
func SSHTargets(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	// The caller may pass the address already entered so a non standard port
	// is probed too, not just the conventional ones.
	requested := strings.TrimSpace(c.Query("address"))
	c.JSON(http.StatusOK, gin.H{"targets": setup.DiscoverSSHTargets(ctx, requested)})
}

// Diagnose detects the SSH target platform and nginx installation using
// read-only commands. It does not persist the detected settings.
func Diagnose(c *gin.Context) {
	var p setup.SetupParams
	if !bindSetupParams(c, &p) {
		return
	}
	client, err := setup.NewClientFromParams(p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := setup.DiagnoseHost(ctx, client, p)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
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
	if err := setup.ValidateHostAddress(req.HostAddress); err != nil {
		abortBadRequest(c, err)
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
	resetSSHClient()
	c.JSON(http.StatusOK, gin.H{"message": "trusted"})
}

func ScanHostKey(c *gin.Context) {
	var req hostKeyScanRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	if err := setup.ValidateHostAddress(req.HostAddress); err != nil {
		abortBadRequest(c, err)
		return
	}

	path := hostKnownHostsPath()
	kh, err := hostssh.NewKnownHosts(path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	var keys []gossh.PublicKey
	// Pasted output carries no evidence about which algorithms answered, so it
	// leaves the coverage nil and no entry is reported stale from it.
	var probed map[string]bool
	if req.KeyscanOutput != "" {
		keys, err = hostssh.ParseSSHKeyscanOutput(req.KeyscanOutput)
	} else {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		keys, probed, err = hostssh.ScanHostKeysWithCoverage(ctx, req.HostAddress, 10*time.Second)
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	result, err := hostssh.ClassifyScannedHostKeys(req.HostAddress, keys, probed, kh)
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
	if err := setup.ValidateHostAddress(req.HostAddress); err != nil {
		abortBadRequest(c, err)
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
	resetSSHClient()
	c.JSON(http.StatusOK, gin.H{"message": "trusted"})
}

func ReplaceHostKey(c *gin.Context) {
	var req hostKeyReplaceRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	if err := setup.ValidateHostAddress(req.HostAddress); err != nil {
		abortBadRequest(c, err)
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
	resetSSHClient()
	c.JSON(http.StatusOK, gin.H{"message": "replaced"})
}

func DeleteHostKey(c *gin.Context) {
	var req hostKeyDeleteRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	if err := setup.ValidateHostAddress(req.HostAddress); err != nil {
		abortBadRequest(c, err)
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
	resetSSHClient()
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
