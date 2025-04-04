package index

import (
	"io"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/gin-gonic/gin"
)

// GetIndexStatus is an SSE endpoint that sends real-time index status updates
func GetIndexStatus(c *gin.Context) {
	api.SetSSEHeaders(c)
	notify := c.Writer.CloseNotify()

	// Subscribe to scanner status changes
	statusChan := cache.SubscribeScanningStatus()

	// Ensure we unsubscribe when the handler exits
	defer cache.UnsubscribeScanningStatus(statusChan)

	// Main event loop
	for {
		select {
		case status, ok := <-statusChan:
			// If channel closed, exit
			if !ok {
				return
			}

			// Send status update
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("message", gin.H{
					"scanning": status,
				})
				return false
			})
		case <-time.After(30 * time.Second):
			// Send heartbeat to keep connection alive
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("heartbeat", "")
				return false
			})
		case <-notify:
			// Client disconnected
			return
		}
	}
}
