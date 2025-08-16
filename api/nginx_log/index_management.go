package nginx_log

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// RebuildIndex rebuilds the log index asynchronously (all files or specific file)
// Returns immediately with HTTP 202 Accepted, actual rebuild happens in background
func RebuildIndex(c *gin.Context) {
	var request controlStruct
	if err := c.ShouldBindJSON(&request); err != nil {
		// No JSON body means rebuild all indexes
		request.Path = ""
	}

	service := nginx_log.GetBackgroundLogService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrBackgroundServiceNotAvailable)
		return
	}

	indexer := service.GetIndexer()
	if indexer == nil {
		cosy.ErrHandler(c, nginx_log.ErrIndexerNotAvailable)
		return
	}

	// If path is provided, rebuild specific file; otherwise rebuild all
	if request.Path != "" {
		// Rebuild specific file asynchronously
		go func() {
			var wg sync.WaitGroup
			if err := indexer.ForceReindexFileGroup(request.Path, &wg); err != nil {
				logger.Errorf("Failed to rebuild index for file group %s: %v", request.Path, err)
				return
			}
			wg.Wait()

			// Update status after all tasks complete
			statusManager := nginx_log.GetIndexingStatusManager()
			statusManager.UpdateIndexingStatus()

			// Completion notification is now handled automatically by ProgressTracker

			logger.Infof("Successfully completed rebuild for file group: %s", request.Path)
		}()

		c.JSON(http.StatusAccepted, gin.H{
			"message": fmt.Sprintf("Index rebuild started for: %s", request.Path),
			"status":  "accepted",
		})
	} else {
		// Rebuild all indexes asynchronously
		go func() {
			if err := indexer.RebuildIndex(); err != nil {
				logger.Errorf("Failed to rebuild all indexes: %v", err)
				return
			}

			// Update status after rebuild complete
			statusManager := nginx_log.GetIndexingStatusManager()
			statusManager.UpdateIndexingStatus()

			logger.Info("Successfully completed rebuild for all indexes")
		}()

		c.JSON(http.StatusAccepted, gin.H{
			"message": "Index rebuild started for all logs",
			"status":  "accepted",
		})
	}
}
