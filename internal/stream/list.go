package stream

import (
	"context"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
)

// ListOptions represents the options for listing streams
type ListOptions struct {
	Search     string
	Status     string
	OrderBy    string
	Sort       string
	EnvGroupID uint64
}

// GetStreamConfigs retrieves and processes stream configurations with database integration
func GetStreamConfigs(ctx context.Context, options *ListOptions, streams []*model.Stream) ([]config.Config, error) {
	// Convert to generic options
	genericOptions := &config.GenericListOptions{
		Search:      options.Search,
		Status:      options.Status,
		OrderBy:     options.OrderBy,
		Sort:        options.Sort,
		EnvGroupID:  options.EnvGroupID,
		IncludeDirs: false, // Filter out directories for stream configurations
	}

	// Create processor with stream-specific logic
	processor := &config.GenericConfigProcessor{
		Paths: config.ConfigPaths{
			AvailableDir: "streams-available",
			EnabledDir:   "streams-enabled",
		},
		StatusMapBuilder: config.DefaultStatusMapBuilder,
		ConfigBuilder:    buildConfig,
		FilterMatcher:    config.DefaultFilterMatcher,
	}

	return config.GetGenericConfigs(ctx, genericOptions, streams, processor)
}

// buildConfig creates a config.Config from file information with stream-specific data
func buildConfig(fileName string, fileInfo os.FileInfo, status config.ConfigStatus, envGroupID uint64, envGroup *model.EnvGroup) config.Config {
	indexedStream := GetIndexedStream(fileName)

	// Convert proxy targets
	proxyTargets := make([]config.ProxyTarget, len(indexedStream.ProxyTargets))
	for i, target := range indexedStream.ProxyTargets {
		proxyTargets[i] = config.ProxyTarget{
			Host: target.Host,
			Port: target.Port,
			Type: target.Type,
		}
	}

	return config.Config{
		Name:         fileName,
		ModifiedAt:   fileInfo.ModTime(),
		Size:         fileInfo.Size(),
		IsDir:        fileInfo.IsDir(),
		Status:       status,
		EnvGroupID:   envGroupID,
		EnvGroup:     envGroup,
		ProxyTargets: proxyTargets,
	}
}
