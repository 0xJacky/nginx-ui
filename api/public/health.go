package public

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Healthz reports whether the Nginx UI backend can serve HTTP requests.
func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
