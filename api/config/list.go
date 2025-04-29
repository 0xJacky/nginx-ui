package config

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func GetConfigs(c *gin.Context) {
	name := c.Query("name")
	sortBy := c.Query("sort_by")
	order := c.DefaultQuery("order", "desc")

	// Get directory parameter
	encodedDir := c.DefaultQuery("dir", "/")

	// Handle cases where the path might be encoded multiple times
	dir := encodedDir
	// Try decoding until the path no longer changes
	for {
		newDecodedDir, decodeErr := url.QueryUnescape(dir)
		if decodeErr != nil {
			cosy.ErrHandler(c, decodeErr)
			return
		}

		if newDecodedDir == dir {
			break
		}
		dir = newDecodedDir
	}

	// Ensure the directory path format is correct
	dir = strings.TrimSpace(dir)
	if dir != "/" && strings.HasSuffix(dir, "/") {
		dir = strings.TrimSuffix(dir, "/")
	}

	configs, err := config.GetConfigList(dir, func(file os.FileInfo) bool {
		return name == "" || strings.Contains(file.Name(), name)
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	configs = config.Sort(sortBy, order, configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}
