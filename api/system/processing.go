package system

import (
	"time"

	"io"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/gin-gonic/gin"
)

type ProcessingStatus struct {
	IndexScanning      bool `json:"index_scanning"`
	AutoCertProcessing bool `json:"auto_cert_processing"`
}

// GetProcessingStatus is an SSE endpoint that sends real-time processing status updates
func GetProcessingStatus(c *gin.Context) {
	api.SetSSEHeaders(c)
	notify := c.Writer.CloseNotify()

	indexScanning := cache.SubscribeScanningStatus()
	defer cache.UnsubscribeScanningStatus(indexScanning)
	autoCert := cert.SubscribeProcessingStatus()
	defer cert.UnsubscribeProcessingStatus(autoCert)

	// Track current status
	status := ProcessingStatus{
		IndexScanning:      false,
		AutoCertProcessing: false,
	}

	sendStatus := func() {
		c.Stream(func(w io.Writer) bool {
			c.SSEvent("message", status)
			return false
		})
	}

	for {
		select {
		case indexStatus, ok := <-indexScanning:
			if !ok {
				return
			}
			status.IndexScanning = indexStatus
			sendStatus()
		case certStatus, ok := <-autoCert:
			if !ok {
				return
			}
			status.AutoCertProcessing = certStatus
			sendStatus()
		case <-time.After(30 * time.Second):
			c.Stream(func(w io.Writer) bool {
				c.SSEvent("heartbeat", "")
				return false
			})
		case <-kernel.Context.Done():
			return
		case <-notify:
			// Client disconnected
			return
		}
	}
}
