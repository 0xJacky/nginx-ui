package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/performance"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// GetPerformanceSettings retrieves current Nginx performance settings
func GetPerformanceSettings(c *gin.Context) {
	// Get Nginx worker configuration info
	perfInfo, err := performance.GetNginxWorkerConfigInfo()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, perfInfo)
}

// UpdatePerformanceSettings updates Nginx performance settings
func UpdatePerformanceSettings(c *gin.Context) {
	var perfOpt performance.PerfOpt
	if !cosy.BindAndValid(c, &perfOpt) {
		return
	}

	err := performance.UpdatePerfOpt(&perfOpt)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	GetPerformanceSettings(c)
}
