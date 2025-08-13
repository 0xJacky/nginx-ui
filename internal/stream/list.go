package stream

import (
	"context"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
)

// ListOptions represents the options for listing streams
type ListOptions struct {
	Search     string
	Name       string
	Status     string
	OrderBy    string
	Sort       string
	NamespaceID uint64
}

// GetStreamConfigs retrieves and processes stream configurations with database integration
func GetStreamConfigs(ctx context.Context, options *ListOptions, streams []*model.Stream) ([]config.Config, error) {
	// Convert to generic options
	genericOptions := &config.GenericListOptions{
		Search:      options.Search,
		Name:        options.Name,
		Status:      options.Status,
		OrderBy:     options.OrderBy,
		Sort:        options.Sort,
		NamespaceID: options.NamespaceID,
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
func buildConfig(fileName string, fileInfo os.FileInfo, status config.ConfigStatus, namespaceID uint64, namespace *model.Namespace) config.Config {
	indexedStream := GetIndexedStream(fileName)

	// Convert proxy targets, expanding upstream references
	var proxyTargets []config.ProxyTarget
	upstreamService := upstream.GetUpstreamService()

	for _, target := range indexedStream.ProxyTargets {
		// Check if target.Host is an upstream name
		if upstreamDef, exists := upstreamService.GetUpstreamDefinition(target.Host); exists {
			// Replace with upstream servers
			for _, server := range upstreamDef.Servers {
				proxyTargets = append(proxyTargets, config.ProxyTarget{
					Host: server.Host,
					Port: server.Port,
					Type: server.Type,
				})
			}
		} else {
			// Regular proxy target
			proxyTargets = append(proxyTargets, config.ProxyTarget{
				Host: target.Host,
				Port: target.Port,
				Type: target.Type,
			})
		}
	}

	return config.Config{
		Name:         fileName,
		ModifiedAt:   fileInfo.ModTime(),
		Size:         fileInfo.Size(),
		IsDir:        fileInfo.IsDir(),
		Status:       status,
		NamespaceID:  namespaceID,
		Namespace:    namespace,
		ProxyTargets: proxyTargets,
	}
}
