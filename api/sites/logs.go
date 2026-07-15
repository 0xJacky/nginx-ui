package sites

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// GetSiteLogs returns the resolved access/error log paths of a site,
// used by the frontend to open the log viewer or traffic analytics dashboard.
func GetSiteLogs(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))

	logs, err := site.GetLogs(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs": logs,
	})
}
