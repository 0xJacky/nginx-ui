package nginx

// Implementation of GetDetailedStatus API
// This feature is designed to address Issue #850, providing Nginx load monitoring functionality similar to BT Panel
// Returns detailed Nginx status information, including request statistics, connections, worker processes, and other data

import (
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/performance"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// PerformanceInfo stores Nginx performance-related information
type PerformanceInfo struct {
	// Basic status information
	performance.StubStatusData

	// Process-related information
	performance.NginxProcessInfo

	// Configuration information
	performance.NginxConfigInfo
}

// GetDetailStatus retrieves detailed Nginx status information
func GetDetailStatus(c *gin.Context) {
	response := performance.GetPerformanceData()
	c.JSON(http.StatusOK, response)
}

// CheckStubStatus gets Nginx stub_status module status
func CheckStubStatus(c *gin.Context) {
	stubStatus := performance.GetStubStatus()

	c.JSON(http.StatusOK, stubStatus)
}

// ToggleStubStatus enables or disables stub_status module
func ToggleStubStatus(c *gin.Context) {
	var json struct {
		Enable bool `json:"enable"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	stubStatus := performance.GetStubStatus()

	// If current status matches desired status, no action needed
	if stubStatus.Enabled == json.Enable {
		c.JSON(http.StatusOK, stubStatus)
		return
	}

	var err error
	if json.Enable {
		err = performance.EnableStubStatus()
	} else {
		err = performance.DisableStubStatus()
	}

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Reload Nginx configuration
	reloadOutput, err := nginx.Reload()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if len(reloadOutput) > 0 && (strings.Contains(strings.ToLower(reloadOutput), "error") ||
		strings.Contains(strings.ToLower(reloadOutput), "failed")) {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(nginx.ErrReloadFailed, reloadOutput))
		return
	}

	// Check status after operation
	newStubStatus := performance.GetStubStatus()

	c.JSON(http.StatusOK, newStubStatus)
}
