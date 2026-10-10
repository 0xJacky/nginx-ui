package middleware

import (
	"bufio"
	"net"

	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
)

// RequireMFAManagement never accepts a service token or node principal.
func RequireMFAManagement(c *gin.Context) bool {
	RequireInteractiveUser()(c)
	if c.IsAborted() {
		return false
	}
	return VerifiedSecureSession(c, "MFA verification is required to manage MFA")
}

type mfaResponseWriter struct {
	gin.ResponseWriter
	user *model.User
}

func (w *mfaResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.ResponseWriter.Hijack()
	if err != nil {
		return nil, rw, err
	}
	return user.TrackMFAConnection(conn, w.user.ID, w.user.MFAVersion), rw, nil
}
