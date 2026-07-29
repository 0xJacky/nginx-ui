package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

// getToken reads credentials only from the explicit Authorization header.
// Browser-managed cookies and URL query parameters are ambient credentials and
// must not authorize management API requests because they enable CSRF and leak
// tokens through URLs.
func getToken(c *gin.Context) (token string) {
	return c.GetHeader("Authorization")
}

// getTokenWS reads token from header or query only (no cookie fallback).
// This prevents Cross-Site WebSocket Hijacking (CSWSH) by ensuring
// browsers cannot silently authenticate WebSocket upgrades via cookies.
func getTokenWS(c *gin.Context) (token string) {
	if token = c.GetHeader("Authorization"); token != "" {
		return
	}

	if token = c.Query("token"); token != "" {
		if len(token) > 16 {
			// Try URL-safe base64 first (browsers send `+` -> `-`, `/` -> `_` to
			// avoid query-string corruption); fall back to standard base64 for
			// backward compatibility with older clients.
			if tokenBytes, err := base64.RawURLEncoding.DecodeString(token); err == nil {
				return string(tokenBytes)
			}
			tokenBytes, _ := base64.StdEncoding.DecodeString(token)
			return string(tokenBytes)
		}
		return token
	}

	return ""
}

// getXNodeID from header or query
func getXNodeID(c *gin.Context) (xNodeID string) {
	if xNodeID = c.GetHeader("X-Node-ID"); xNodeID != "" {
		return xNodeID
	}

	return c.Query("x_node_id")
}

func authenticateNodeRequest(c *gin.Context) (bool, error) {
	if c.Request.URL.Query().Has("node_secret") {
		return true, fmt.Errorf("node credentials are not accepted in query parameters")
	}

	if c.GetHeader("Signature-Input") != "" || c.GetHeader("Signature") != "" {
		principal, err := nodeauth.VerifyRequest(c.Request)
		if err != nil {
			nodeauth.CloseStagedBody(c.Request)
			return true, err
		}
		c.Request = nodeauth.WithPrincipal(c.Request, principal)
		c.Set(nodeauth.GinPrincipalKey, principal)
		c.Set("user", user.GetInitUser(c))
		return true, nil
	}

	secret := strings.TrimSpace(c.GetHeader("X-Node-Secret"))
	if secret == "" {
		return false, nil
	}
	// Nodes registered with a shared secret authenticate this way until the
	// maintenance pass upgrades them to signed requests.
	configuredSecret := settings.NodeSettings.Secret
	if configuredSecret == "" ||
		len(secret) != len(configuredSecret) ||
		subtle.ConstantTimeCompare([]byte(secret), []byte(configuredSecret)) != 1 {
		return true, fmt.Errorf("legacy node authentication failed")
	}
	principal := &nodeauth.Principal{
		CredentialID:         "legacy",
		ControllerInstanceID: "legacy",
		AuthMethod:           model.NodeAuthMethodLegacy,
	}
	c.Request = nodeauth.WithPrincipal(c.Request, principal)
	c.Set(nodeauth.GinPrincipalKey, principal)
	c.Set("user", user.GetInitUser(c))
	return true, nil
}

// AuthRequired is a middleware that checks if the user is authenticated
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		abortWithAuthFailure := func() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Authorization failed",
			})
		}

		xNodeID := getXNodeID(c)
		if xNodeID != "" {
			c.Set("ProxyNodeID", xNodeID)
		}

		if handled, err := authenticateNodeRequest(c); handled {
			if err != nil {
				abortWithAuthFailure()
				return
			}
			defer nodeauth.CloseStagedBody(c.Request)
			c.Next()
			return
		}

		token := getToken(c)
		if token == "" {
			abortWithAuthFailure()
			return
		}

		var (
			u  *model.User
			ok bool
		)

		if len(token) <= 16 {
			// Short token (16 characters)
			u, ok = user.GetTokenUserByShortToken(token)
			if !ok {
				abortWithAuthFailure()
				return
			}
		} else {
			// Long JWT token
			u, ok = user.GetTokenUser(token)
			if !ok {
				abortWithAuthFailure()
				return
			}
		}

		c.Set("user", u)
		c.Next()
	}
}

// AuthRequiredWS is a WebSocket-specific auth middleware that does NOT read cookies.
// This prevents CSWSH attacks by requiring explicit token passing via header or query param.
func AuthRequiredWS() gin.HandlerFunc {
	return func(c *gin.Context) {
		abortWithAuthFailure := func() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Authorization failed",
			})
		}

		xNodeID := getXNodeID(c)
		if xNodeID != "" {
			c.Set("ProxyNodeID", xNodeID)
		}

		if handled, err := authenticateNodeRequest(c); handled {
			if err != nil {
				abortWithAuthFailure()
				return
			}
			defer nodeauth.CloseStagedBody(c.Request)
			c.Next()
			return
		}

		token := getTokenWS(c)
		if token == "" {
			abortWithAuthFailure()
			return
		}

		var (
			u  *model.User
			ok bool
		)

		if len(token) <= 16 {
			u, ok = user.GetTokenUserByShortToken(token)
			if !ok {
				abortWithAuthFailure()
				return
			}
		} else {
			u, ok = user.GetTokenUser(token)
			if !ok {
				abortWithAuthFailure()
				return
			}
		}

		c.Set("user", u)
		c.Next()
	}
}

type ServerFileSystemType struct {
	http.FileSystem
}

func (f ServerFileSystemType) Exists(prefix string, _path string) bool {
	file, err := f.Open(path.Join(prefix, _path))
	if file != nil {
		defer func(file http.File) {
			err = file.Close()
			if err != nil {
				logger.Error("file not found", err)
			}
		}(file)
	}
	return err == nil
}

// CacheJs is a middleware that send header to client to cache js file
func CacheJs() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.Contains(c.Request.URL.String(), "js") {
			c.Header("Cache-Control", "max-age: 1296000")
			if c.Request.Header.Get("If-Modified-Since") == settings.LastModified {
				c.AbortWithStatus(http.StatusNotModified)
			}
			c.Header("Last-Modified", settings.LastModified)
		}
	}
}
