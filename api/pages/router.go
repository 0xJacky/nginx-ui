package pages

import (
	"github.com/gin-gonic/gin"
)

// InitRouter initializes the pages routes
func InitRouter(r *gin.Engine) {
	// Register maintenance page route
	r.GET("/pages/maintenance", MaintenancePage)
}
