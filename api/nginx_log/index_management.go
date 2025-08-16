package nginx_log

import (
	"fmt"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// RebuildIndex rebuilds the log index (all files or specific file)
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
		// Rebuild specific file
		if err := indexer.ForceReindexFile(request.Path); err != nil {
			cosy.ErrHandler(c, cosy.WrapErrorWithParams(nginx_log.ErrFailedToRebuildFileIndex, err.Error()))
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("File index rebuild started successfully for: %s", request.Path),
		})
	} else {
		// Rebuild all indexes
		if err := indexer.RebuildIndex(); err != nil {
			cosy.ErrHandler(c, cosy.WrapErrorWithParams(nginx_log.ErrFailedToRebuildIndex, err.Error()))
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"message": "Index rebuild started successfully",
		})
	}
}