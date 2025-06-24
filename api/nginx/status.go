// Implementation of GetDetailedStatus API
// This feature is designed to address Issue #850, providing Nginx load monitoring functionality similar to BT Panel
// Returns detailed Nginx status information, including request statistics, connections, worker processes, and other data
package nginx

import (
	"net/http"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/performance"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// NginxPerformanceInfo stores Nginx performance-related information
type NginxPerformanceInfo struct {
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

// StreamDetailStatus streams Nginx detailed status information using SSE
func StreamDetailStatus(c *gin.Context) {
	// Set SSE response headers
	api.SetSSEHeaders(c)

	// Create context that cancels when client disconnects
	ctx := c.Request.Context()

	// Create a ticker channel to prevent goroutine leaks
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send initial data immediately
	sendPerformanceData(c)

	// Use goroutine to send data periodically
	for {
		select {
		case <-ticker.C:
			// Send performance data
			sendPerformanceData(c)
		case <-ctx.Done():
			// Client closed connection or request canceled
			logger.Debug("Client closed connection")
			return
		}
	}
}

// sendPerformanceData sends performance data once
func sendPerformanceData(c *gin.Context) {
	response := performance.GetPerformanceData()

	// Send SSE event
	c.SSEvent("message", response)

	// Flush buffer to ensure data is sent immediately
	c.Writer.Flush()
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
