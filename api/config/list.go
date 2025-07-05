package config

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// ConfigFileEntity represents a generic configuration file entity
type ConfigFileEntity struct {
	path       string
	envGroupID uint64
	envGroup   *model.EnvGroup
}

// GetPath implements ConfigEntity interface
func (c *ConfigFileEntity) GetPath() string {
	return c.path
}

// GetEnvGroupID implements ConfigEntity interface
func (c *ConfigFileEntity) GetEnvGroupID() uint64 {
	return c.envGroupID
}

// GetEnvGroup implements ConfigEntity interface
func (c *ConfigFileEntity) GetEnvGroup() *model.EnvGroup {
	return c.envGroup
}

func GetConfigs(c *gin.Context) {
	search := c.Query("search")
	sortBy := c.Query("sort_by")
	order := c.DefaultQuery("order", "desc")
	envGroupIDStr := c.Query("env_group_id")

	// Get directory parameter
	encodedDir := c.DefaultQuery("dir", "/")

	// Handle cases where the path might be encoded multiple times
	dir := helper.UnescapeURL(encodedDir)

	// Ensure the directory path format is correct
	dir = strings.TrimSpace(dir)
	if dir != "/" && strings.HasSuffix(dir, "/") {
		dir = strings.TrimSuffix(dir, "/")
	}

	// Parse env_group_id
	var envGroupID uint64
	if envGroupIDStr != "" {
		if id, err := strconv.ParseUint(envGroupIDStr, 10, 64); err == nil {
			envGroupID = id
		}
	}

	// Create options
	options := &config.GenericListOptions{
		Search:      search,
		OrderBy:     sortBy,
		Sort:        order,
		EnvGroupID:  envGroupID,
		IncludeDirs: true, // Keep directories for the list.go endpoint
	}

	// Get config files from directory and create entities
	configFiles, err := os.ReadDir(nginx.GetConfPath(dir))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Create entities for each config file
	var entities []*ConfigFileEntity
	for _, file := range configFiles {
		// Skip directories only if IncludeDirs is false
		if file.IsDir() && !options.IncludeDirs {
			continue
		}

		// For generic config files, we don't have database records
		// so envGroupID and envGroup will be 0 and nil
		entity := &ConfigFileEntity{
			path:       filepath.Join(nginx.GetConfPath(dir), file.Name()),
			envGroupID: 0,
			envGroup:   nil,
		}
		entities = append(entities, entity)
	}

	// Create processor for generic config files
	processor := &config.GenericConfigProcessor{
		Paths: config.ConfigPaths{
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
func createConfigBuilder(dir string) config.ConfigBuilder {
	return func(fileName string, fileInfo os.FileInfo, status config.ConfigStatus, envGroupID uint64, envGroup *model.EnvGroup) config.Config {
		return config.Config{
			Name:       fileName,
			ModifiedAt: fileInfo.ModTime(),
			Size:       fileInfo.Size(),
			IsDir:      fileInfo.IsDir(),
			Status:     status,
			EnvGroupID: envGroupID,
			EnvGroup:   envGroup,
			Dir:        dir,
		}
	}
}
