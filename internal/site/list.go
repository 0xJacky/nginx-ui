package site

import (
	"context"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"gorm.io/gorm/clause"
)

// ListOptions represents the options for listing sites
type ListOptions struct {
	Search      string
	Name        string
	Status      string
	OrderBy     string
	Sort        string
	NamespaceID uint64
}

// GetSiteConfigs retrieves and processes site configurations with database integration
func GetSiteConfigs(ctx context.Context, options *ListOptions, sites []*model.Site) ([]config.Config, error) {
	// Convert to generic options
	genericOptions := &config.GenericListOptions{
		Search:      options.Search,
		Name:        options.Name,
		Status:      options.Status,
		OrderBy:     options.OrderBy,
		Sort:        options.Sort,
		NamespaceID: options.NamespaceID,
		IncludeDirs: false, // Filter out directories for site configurations
	}

	// Create processor with site-specific logic
	processor := &config.GenericConfigProcessor{
		Paths: config.Paths{
			AvailableDir: "sites-available",
			EnabledDir:   "sites-enabled",
		},
		StatusMapBuilder: config.SiteStatusMapBuilder(MaintenanceSuffix),
		ConfigBuilder:    buildConfig,
		FilterMatcher:    config.DefaultFilterMatcher,
	}

	configs, err := config.GetGenericConfigs(ctx, genericOptions, sites, processor)
	if err != nil {
		return nil, err
	}

	configs = applyRemoteStatus(configs, sites, options.Status)
	return attachSiteMetadata(configs, sites)
}

// applySiteDescriptions maps DB site descriptions to config entries by basename.
func applySiteDescriptions(configs []config.Config, sites []*model.Site) []config.Config {
	sitesByName := make(map[string]*model.Site, len(sites))
	for _, siteModel := range sites {
		sitesByName[filepath.Base(siteModel.Path)] = siteModel
	}

	for i := range configs {
		if siteModel, ok := sitesByName[configs[i].Name]; ok {
			configs[i].Description = siteModel.Description
		}
	}

	return configs
}

func attachSiteMetadata(configs []config.Config, sites []*model.Site) ([]config.Config, error) {
	sitesByName := make(map[string]*model.Site, len(sites))
	for _, siteModel := range sites {
		sitesByName[filepath.Base(siteModel.Path)] = siteModel
	}

	missingPathsSet := make(map[string]struct{})
	for _, cfg := range configs {
		if _, ok := sitesByName[cfg.Name]; ok {
			continue
		}
		path, err := ResolveAvailablePath(cfg.Name)
		if err != nil {
			continue
		}
		missingPathsSet[path] = struct{}{}
	}

	if len(missingPathsSet) > 0 {
		missingPaths := make([]string, 0, len(missingPathsSet))
		records := make([]*model.Site, 0, len(missingPathsSet))
		for path := range missingPathsSet {
			missingPaths = append(missingPaths, path)
			records = append(records, &model.Site{Path: path})
		}

		s := query.Site
		if err := s.Clauses(clause.OnConflict{DoNothing: true}).Create(records...); err != nil {
			return nil, err
		}

		var created []*model.Site
		if err := model.UseDB().Where("path IN ?", missingPaths).Find(&created).Error; err != nil {
			return nil, err
		}

		for _, siteModel := range created {
			sitesByName[filepath.Base(siteModel.Path)] = siteModel
		}
	}

	for i := range configs {
		siteModel := sitesByName[configs[i].Name]
		if siteModel == nil {
			continue
		}
		configs[i].Index = siteModel.ID
		configs[i].Description = siteModel.Description
	}

	return configs, nil
}

// applyRemoteStatus overrides the filesystem derived status for sites owned by a
// remote namespace, where the deployment intent is stored in the database.
func applyRemoteStatus(configs []config.Config, sites []*model.Site, statusFilter string) []config.Config {
	remote := make(map[string]bool, len(sites))
	for _, siteModel := range sites {
		if siteModel.Namespace.IsRemoteDeploy() {
			remote[filepath.Base(siteModel.Path)] = siteModel.RemoteEnabled
		}
	}
	if len(remote) == 0 {
		return configs
	}

	filtered := make([]config.Config, 0, len(configs))
	for _, item := range configs {
		if enabled, ok := remote[item.Name]; ok {
			item.Status = config.Status(remoteStatus(enabled))
		}
		if statusFilter != "" && string(item.Status) != statusFilter {
			continue
		}
		filtered = append(filtered, item)
	}

	return filtered
}

// buildConfig creates a config.Config from file information with site-specific data
func buildConfig(fileName string, fileInfo os.FileInfo, status config.Status, index uint64, namespaceID uint64, namespace *model.Namespace) config.Config {
	indexedSite := GetIndexedSite(fileName)

	// Convert proxy targets, expanding upstream references
	var proxyTargets []config.ProxyTarget
	upstreamService := upstream.GetUpstreamService()

	for _, target := range indexedSite.ProxyTargets {
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
		Index:        index,
		Name:         fileName,
		ModifiedAt:   fileInfo.ModTime(),
		Size:         fileInfo.Size(),
		IsDir:        fileInfo.IsDir(),
		Status:       status,
		NamespaceID:  namespaceID,
		Namespace:    namespace,
		Urls:         indexedSite.Urls,
		ProxyTargets: proxyTargets,
	}
}
