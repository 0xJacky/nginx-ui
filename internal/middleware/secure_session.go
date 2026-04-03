package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const SecureSessionCookieName = "_nginx_ui_secure_session"

func ensureSecureSessionCookie(c *gin.Context) {
	if _, err := c.Cookie(SecureSessionCookieName); err != http.ErrNoCookie {
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		SecureSessionCookieName,
		hex.EncodeToString(b),
		0,
		"/",
		"",
		cSettings.ServerSettings.EnableHTTPS,
		true,
	)
}

// EnsureSecureSessionCookie makes sure the session-binding cookie exists.
func EnsureSecureSessionCookie(c *gin.Context) {
	ensureSecureSessionCookie(c)
}

// SecureSessionCookie sets an HttpOnly SameSite=Lax cookie when serving the SPA.
// This cookie acts as a CSRF-proof session binding for the short token endpoint.
func SecureSessionCookie() gin.HandlerFunc {
	return func(c *gin.Context) {
		ensureSecureSessionCookie(c)
		c.Next()
	}
}

func RequireSecureSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := c.Get("user")
		if !ok {
			c.Next()
			return
		}
		cUser := u.(*model.User)
		if !cUser.EnabledOTP() {
			c.Next()
			return
		}
		ssid := c.GetHeader("X-Secure-Session-ID")
		if ssid == "" {
			ssid = c.Query("X-Secure-Session-ID")
		}
		if ssid == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Secure Session ID is empty",
			})
			return
		}

		if user.VerifySecureSessionID(ssid, cUser.ID) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Secure Session ID is invalid",
		})
	}
}
