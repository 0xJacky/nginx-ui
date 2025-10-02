package config

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

// FileEntity represents a generic configuration file entity
type FileEntity struct {
	path        string
	namespaceID uint64
	namespace   *model.Namespace
}

// GetPath implements Entity interface
func (c *FileEntity) GetPath() string {
	return c.path
}

// GetNamespaceID implements Entity interface
func (c *FileEntity) GetNamespaceID() uint64 {
	return c.namespaceID
}

// GetNamespace implements Entity interface
func (c *FileEntity) GetNamespace() *model.Namespace {
	return c.namespace
}

func GetConfigs(c *gin.Context) {
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "name")
	order := c.DefaultQuery("order", "asc")
	namespaceId := cast.ToUint64(c.Query("namespace_id"))

	// Get directory parameter
	encodedDir := c.DefaultQuery("dir", "/")

	// Handle cases where the path might be encoded multiple times
	dir := helper.UnescapeURL(encodedDir)

	// Ensure the directory path format is correct
	dir = strings.TrimSpace(dir)
	if dir != "/" && strings.HasSuffix(dir, "/") {
		dir = strings.TrimSuffix(dir, "/")
	}

	// Create options
	options := &config.GenericListOptions{
		Search:      search,
		OrderBy:     sortBy,
		Sort:        order,
		NamespaceID: namespaceId,
		IncludeDirs: true, // Keep directories for the list.go endpoint
	}

	// Get config files from directory and create entities
	configFiles, err := os.ReadDir(nginx.GetConfPath(dir))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Create entities for each config file
	var entities []*FileEntity
	for _, file := range configFiles {
		// Skip directories only if IncludeDirs is false
		if file.IsDir() && !options.IncludeDirs {
			continue
		}

		// For generic config files, we don't have database records
		// so namespaceID and namespace will be 0 and nil
		entity := &FileEntity{
			path:        filepath.Join(nginx.GetConfPath(dir), file.Name()),
			namespaceID: 0,
			namespace:   nil,
		}
		entities = append(entities, entity)
	}

	// Create processor for generic config files
	processor := &config.GenericConfigProcessor{
		Paths: config.Paths{
			AvailableDir: dir,
			EnabledDir:   dir, // For generic configs, available and enabled are the same
		},
		StatusMapBuilder: config.DefaultStatusMapBuilder,
		ConfigBuilder:    createConfigBuilder(dir),
		FilterMatcher:    config.DefaultFilterMatcher,
	}

	// Get configurations using the generic processor
	configs, err := config.GetGenericConfigs(c, options, entities, processor)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}

// createConfigBuilder creates a custom config builder for generic config files
func createConfigBuilder(dir string) config.Builder {
	return func(fileName string, fileInfo os.FileInfo, status config.Status, namespaceID uint64, namespace *model.Namespace) config.Config {
		return config.Config{
			Name:        fileName,
			ModifiedAt:  fileInfo.ModTime(),
			Size:        fileInfo.Size(),
			IsDir:       fileInfo.IsDir(),
			Status:      status,
			NamespaceID: namespaceID,
			Namespace:   namespace,
			Dir:         dir,
		}
	}
}
