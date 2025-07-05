package site

import (
	"context"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
)

// ListOptions represents the options for listing sites
type ListOptions struct {
	Search     string
	Status     string
	OrderBy    string
	Sort       string
	EnvGroupID uint64
}

// GetSiteConfigs retrieves and processes site configurations with database integration
func GetSiteConfigs(ctx context.Context, options *ListOptions, sites []*model.Site) ([]config.Config, error) {
	// Convert to generic options
	genericOptions := &config.GenericListOptions{
		Search:      options.Search,
		Status:      options.Status,
		OrderBy:     options.OrderBy,
		Sort:        options.Sort,
		EnvGroupID:  options.EnvGroupID,
		IncludeDirs: false, // Filter out directories for site configurations
	}

	// Create processor with site-specific logic
	processor := &config.GenericConfigProcessor{
		Paths: config.ConfigPaths{
			AvailableDir: "sites-available",
			EnabledDir:   "sites-enabled",
		},
		StatusMapBuilder: config.SiteStatusMapBuilder(MaintenanceSuffix),
		ConfigBuilder:    buildConfig,
		FilterMatcher:    config.DefaultFilterMatcher,
	}

	return config.GetGenericConfigs(ctx, genericOptions, sites, processor)
}

// buildConfig creates a config.Config from file information with site-specific data
func buildConfig(fileName string, fileInfo os.FileInfo, status config.ConfigStatus, envGroupID uint64, envGroup *model.EnvGroup) config.Config {
	indexedSite := GetIndexedSite(fileName)

	// Convert proxy targets
	proxyTargets := make([]config.ProxyTarget, len(indexedSite.ProxyTargets))
	for i, target := range indexedSite.ProxyTargets {
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
		Urls:         indexedSite.Urls,
		ProxyTargets: proxyTargets,
	}
}
