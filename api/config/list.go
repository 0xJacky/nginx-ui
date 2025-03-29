package config

import (
	"net/http"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

func GetConfigs(c *gin.Context) {
	name := c.Query("name")
	sortBy := c.Query("sort_by")
	order := c.DefaultQuery("order", "desc")
	dir := c.DefaultQuery("dir", "/")

	configFiles, err := os.ReadDir(nginx.GetConfPath(dir))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	configs := make([]config.Config, 0)

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, _ := file.Info()

		if name != "" && !strings.Contains(file.Name(), name) {
			continue
		}

		switch mode := fileInfo.Mode(); {
		case mode.IsRegular(): // regular file, not a hidden file
			if "." == file.Name()[0:1] {
				continue
			}
		case mode&os.ModeSymlink != 0: // is a symbol
			var targetPath string
			targetPath, err = os.Readlink(nginx.GetConfPath(dir, file.Name()))
			if err != nil {
				logger.Error("Read Symlink Error", targetPath, err)
				continue
			}

			var targetInfo os.FileInfo
			targetInfo, err = os.Stat(targetPath)
			if err != nil {
				logger.Error("Stat Error", targetPath, err)
				continue
			}
			// hide the file if it's target file is a directory
			if targetInfo.IsDir() {
				continue
			}
		}

		configs = append(configs, config.Config{
			Name:       file.Name(),
			ModifiedAt: fileInfo.ModTime(),
			Size:       fileInfo.Size(),
			IsDir:      fileInfo.IsDir(),
		})
	}

	configs = config.Sort(sortBy, order, configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}
