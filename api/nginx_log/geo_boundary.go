package nginx_log

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	cosySettings "github.com/uozi-tech/cosy/settings"
)

var geoBoundaryFileNamePattern = regexp.MustCompile(`^\d{6}_full\.json$`)

func resolveGeoBoundaryDir() string {
	basePath := strings.TrimSpace(settings.NginxLogSettings.GeoMapPath)
	if basePath == "" {
		basePath = "maps"
	}

	if filepath.IsAbs(basePath) {
		return filepath.Clean(basePath)
	}

	confDir := filepath.Dir(cosySettings.ConfPath)
	return filepath.Clean(filepath.Join(confDir, basePath))
}

func GetGeoBoundaryFile(c *gin.Context) {
	filename := strings.TrimSpace(c.Param("filename"))
	if !geoBoundaryFileNamePattern.MatchString(filename) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "invalid map file name",
		})
		return
	}

	filePath := filepath.Join(resolveGeoBoundaryDir(), filename)
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{
				"message": "map file not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "failed to read map file",
		})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", fileBytes)
}
