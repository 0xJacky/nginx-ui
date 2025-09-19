package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
)

func GetModules(c *gin.Context) {
	modules := nginx.GetModules()
	modulesList := make([]*nginx.Module, 0, modules.Len())
	for _, module := range modules.AllFromFront() {
		modulesList = append(modulesList, module)
	}
	c.JSON(http.StatusOK, modulesList)
}

// RefreshModulesCache clears and rebuilds the nginx modules cache on demand.
// It returns the refreshed modules list for immediate use by the frontend.
func RefreshModulesCache(c *gin.Context) {
	// Clear cached modules to force re-parsing
	nginx.ClearModulesCache()

	// Rebuild modules cache immediately
	modules := nginx.GetModules()
	modulesList := make([]*nginx.Module, 0, modules.Len())
	for _, module := range modules.AllFromFront() {
		modulesList = append(modulesList, module)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "modules cache refreshed",
		"modules": modulesList,
	})
}
