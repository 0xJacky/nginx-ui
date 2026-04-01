package user

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// IssueShortToken creates a short token for WebSocket authentication.
// Requires both JWT (via AuthRequired) and the session-binding cookie.
func IssueShortToken(c *gin.Context) {
	sessionCookie, err := c.Cookie(middleware.SecureSessionCookieName)
	if err != nil || sessionCookie == "" {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Session binding cookie required",
		})
		return
	}

	u := api.CurrentUser(c)
	shortToken, err := user.GenerateShortToken(u.ID)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"short_token": shortToken,
	})
}
