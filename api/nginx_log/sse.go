package nginx_log

import (
	"io"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/gin-gonic/gin"
)

// GetNginxLogsLive is an SSE endpoint that sends real-time log scanning status updates
func GetNginxLogsLive(c *gin.Context) {
	api.SetSSEHeaders(c)
	notify := c.Writer.CloseNotify()

	// Subscribe to scanner status changes
	statusChan := cache.SubscribeStatusChanges()

	// Ensure we unsubscribe when the handler exits
	defer cache.UnsubscribeStatusChanges(statusChan)

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
