package nginx_log

import (
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
)

// GetLogList returns a list of Nginx log files with their index status
func GetLogList(c *gin.Context) {
	filters := []func(*nginx_log.NginxLogWithIndex) bool{}

	if logType := c.Query("type"); logType != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogWithIndex) bool {
			return entry.Type == logType
		})
	}

	if name := c.Query("name"); name != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogWithIndex) bool {
			return strings.Contains(entry.Name, name)
		})
	}

	if path := c.Query("path"); path != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogWithIndex) bool {
			return strings.Contains(entry.Path, path)
		})
	}

	// Add filter for indexed status if requested
	if indexed := c.Query("indexed"); indexed != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogWithIndex) bool {
			switch indexed {
			case "true":
				return entry.IndexStatus == nginx_log.IndexStatusIndexed
			case "false":
				return entry.IndexStatus == nginx_log.IndexStatusNotIndexed
			case "indexing":
				return entry.IndexStatus == nginx_log.IndexStatusIndexing
			default:
				return true
			}
		})
	}

	data := nginx_log.GetAllLogsWithIndexGrouped(filters...)

	orderBy := c.DefaultQuery("sort_by", "name")
	sort := c.DefaultQuery("order", "desc")

	data = nginx_log.SortWithIndex(orderBy, sort, data)

	// Calculate summary statistics
	totalCount := len(data)
	indexedCount := 0
	indexingCount := 0
	var totalDocuments uint64 = 0
	
	for _, log := range data {
		switch log.IndexStatus {
		case nginx_log.IndexStatusIndexed:
			indexedCount++
			if log.DocumentCount != 0 {
				// Note: This might count documents multiple times if same docs are in multiple files
				// For now, we'll just use the first indexed file's count as total
				if totalDocuments == 0 {
					totalDocuments = log.DocumentCount
				}
			}
		case nginx_log.IndexStatusIndexing:
			indexingCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
		"summary": gin.H{
			"total_files":     totalCount,
			"indexed_files":   indexedCount,
			"indexing_files":  indexingCount,
			"document_count":  totalDocuments,
		},
	})
}