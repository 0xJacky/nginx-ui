package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
)

func GetModules(c *gin.Context) {
	modules := nginx.GetModules()
	modulesList := make([]nginx.Module, 0, modules.Len())
	for _, module := range modules.AllFromFront() {
		modulesList = append(modulesList, module)
	}
	c.JSON(http.StatusOK, modulesList)
}
