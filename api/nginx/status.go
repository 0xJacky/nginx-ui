// Implementation of GetDetailedStatus API
// This feature is designed to address Issue #850, providing Nginx load monitoring functionality similar to BT Panel
// Returns detailed Nginx status information, including request statistics, connections, worker processes, and other data
package nginx

import (
	"errors"
	"net/http"
	"strings"
	"time"

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
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

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
			if err := sendPerformanceData(c); err != nil {
				logger.Warn("Error sending SSE data:", err)
				return
			}
		case <-ctx.Done():
			// Client closed connection or request canceled
			logger.Debug("Client closed connection")
			return
		}
	}
}

// sendPerformanceData sends performance data once
func sendPerformanceData(c *gin.Context) error {
	response := performance.GetPerformanceData()

	// Send SSE event
	c.SSEvent("message", response)

	// Flush buffer to ensure data is sent immediately
	c.Writer.Flush()
	return nil
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
	reloadOutput := nginx.Reload()
	if len(reloadOutput) > 0 && (strings.Contains(strings.ToLower(reloadOutput), "error") ||
		strings.Contains(strings.ToLower(reloadOutput), "failed")) {
		cosy.ErrHandler(c, errors.New("Reload Nginx failed"))
		return
	}

	// Check status after operation
	newStubStatus := performance.GetStubStatus()

	c.JSON(http.StatusOK, newStubStatus)
}
