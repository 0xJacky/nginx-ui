package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

// GenericListOptions represents the options for listing configurations
type GenericListOptions struct {
	Search      string
	Name        string
	Status      string
	OrderBy     string
	Sort        string
	NamespaceID uint64
	IncludeDirs bool // Whether to include directories in the results, default is false (filter out directories)
}

// Entity represents a generic configuration entity interface
type Entity interface {
	GetPath() string
	GetNamespaceID() uint64
	GetNamespace() *model.Namespace
}

// Paths holds the directory paths for available and enabled configurations
type Paths struct {
	AvailableDir string
	EnabledDir   string
}

// StatusMapBuilder is a function type for building status maps with custom logic
type StatusMapBuilder func(configFiles, enabledConfig []os.DirEntry) map[string]Status

// Builder is a function type for building Config objects with custom logic
type Builder func(fileName string, fileInfo os.FileInfo, status Status, namespaceID uint64, namespace *model.Namespace) Config

// FilterMatcher is a function type for custom filtering logic
type FilterMatcher func(fileName string, status Status, namespaceID uint64, options *GenericListOptions) bool

// GenericConfigProcessor holds all the custom functions for processing configurations
type GenericConfigProcessor struct {
	Paths            Paths
	StatusMapBuilder StatusMapBuilder
	ConfigBuilder    Builder
	FilterMatcher    FilterMatcher
}

// GetGenericConfigs is a unified function for retrieving and processing configurations
func GetGenericConfigs[T Entity](
	ctx context.Context,
	options *GenericListOptions,
	entities []T,
	processor *GenericConfigProcessor,
) ([]Config, error) {
	// Read configuration directories
	configFiles, err := os.ReadDir(nginx.GetConfPath(processor.Paths.AvailableDir))
	if err != nil {
		return nil, err
	}

	enabledConfig, err := os.ReadDir(nginx.GetConfPath(processor.Paths.EnabledDir))
	if err != nil {
		return nil, err
	}

	// Build configuration status map using custom logic
	statusMap := processor.StatusMapBuilder(configFiles, enabledConfig)

	// Create entities map for quick lookup
	entitiesMap := lo.SliceToMap(entities, func(item T) (string, T) {
		return filepath.Base(item.GetPath()), item
	})

	// If fuzzy search is enabled, use search index to filter files
	var searchFilteredFiles []string
	var hasSearchResults bool
	if options.Search != "" {
		logger.Debugf("Starting fuzzy search for query '%s' in directory '%s'", options.Search, processor.Paths.AvailableDir)
		searchFilteredFiles, err = performFuzzySearch(ctx, options.Search, processor.Paths.AvailableDir)
		if err != nil {
			// Fallback to original behavior if search fails
			logger.Debugf("Fuzzy search failed, falling back to simple string matching: %v", err)
			searchFilteredFiles = nil
			hasSearchResults = false
		} else {
			hasSearchResults = true
			logger.Debugf("Fuzzy search completed, found %d matching files", len(searchFilteredFiles))
		}
	}

	// Process and filter configurations
	var configs []Config
	for _, file := range configFiles {
		if file.IsDir() && !options.IncludeDirs {
			continue
		}

		fileInfo, err := file.Info()
		if err != nil {
			continue
		}

		fileName := file.Name()
		status := statusMap[fileName]

		// Get environment group info from database
		var namespaceID uint64
		var namespace *model.Namespace
		if entity, ok := entitiesMap[fileName]; ok {
			namespaceID = entity.GetNamespaceID()
			namespace = entity.GetNamespace()
		}

		// Apply filters using custom logic
		if !processor.FilterMatcher(fileName, status, namespaceID, options) {
			continue
		}

		// Apply fuzzy search filter if enabled
		if hasSearchResults {
			// Check if the file is in the search results
			if !contains(searchFilteredFiles, fileName) {
				// For directories, perform simple string matching since they are not indexed
				if fileInfo.IsDir() {
					// Only include directories if IncludeDirs is true and they match the search
					if options.IncludeDirs {
						// Perform case-insensitive substring matching for directories
						if !strings.Contains(strings.ToLower(fileName), strings.ToLower(options.Search)) {
							continue
						}
					} else {
						// Directories should have been filtered out earlier, but skip just in case
						continue
					}
				} else {
					// For regular files, if they're not in the search results, skip them
					continue
				}
			}
		} else if options.Search != "" {
			// Fallback to simple string matching if search index failed or returned no results
			if !strings.Contains(strings.ToLower(fileName), strings.ToLower(options.Search)) {
				continue
			}
		}

		// Build configuration using custom logic
		configs = append(configs, processor.ConfigBuilder(fileName, fileInfo, status, namespaceID, namespace))
	}

	// Sort and return
	sortedConfigs := Sort(options.OrderBy, options.Sort, configs)

	// Debug log the final results
	if options.Search != "" {
		logger.Debugf("Final search results for query '%s': returning %d configs out of %d total files",
			options.Search, len(sortedConfigs), len(configFiles))
	}

	return sortedConfigs, nil
}

// performFuzzySearch performs fuzzy search using the search index
func performFuzzySearch(ctx context.Context, query, availableDir string) ([]string, error) {

	// Determine search type based on directory
	var searchType string
	switch {
	case strings.Contains(availableDir, "sites"):
		searchType = "site"
	case strings.Contains(availableDir, "streams"):
		searchType = "stream"
	default:
		searchType = "config"
	}

	// Perform search with the determined type
	var results []cache.SearchResult
	var err error

	// Use a larger limit to ensure we get all matching results
	// Since we're filtering by filename, we want to get all possible matches
	// Set a reasonable upper limit to prevent performance issues
	searchLimit := 5000

	if searchType != "" {
		results, err = cache.GetSearchIndexer().SearchByType(ctx, query, searchType, searchLimit)
	} else {
		results, err = cache.GetSearchIndexer().Search(ctx, query, searchLimit)
	}

	if err != nil {
		return nil, err
	}

	// Extract filenames from search results
	var filenames []string
	for _, result := range results {
		filename := filepath.Base(result.Document.Path)
		filenames = append(filenames, filename)
	}

	// Debug log the search results
	logger.Debugf("Search engine returned files for query '%s' in dir '%s': %v (total: %d)",
		query, availableDir, filenames, len(filenames))

	return filenames, nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// DefaultStatusMapBuilder provides the basic status map building logic
func DefaultStatusMapBuilder(configFiles, enabledConfig []os.DirEntry) map[string]Status {
	statusMap := make(map[string]Status)

	// Initialize all as disabled
	for _, file := range configFiles {
		statusMap[file.Name()] = StatusDisabled
	}

	// Update enabled status
	for _, enabledFile := range enabledConfig {
		name := nginx.GetConfNameBySymlinkName(enabledFile.Name())
		statusMap[name] = StatusEnabled
	}

	return statusMap
}

// SiteStatusMapBuilder provides status map building logic with maintenance support
func SiteStatusMapBuilder(maintenanceSuffix string) StatusMapBuilder {
	return func(configFiles, enabledConfig []os.DirEntry) map[string]Status {
		statusMap := make(map[string]Status)

		// Initialize all as disabled
		for _, file := range configFiles {
			statusMap[file.Name()] = StatusDisabled
		}

		// Update enabled and maintenance status
		for _, enabledSite := range enabledConfig {
			name := enabledSite.Name()
			if strings.HasSuffix(name, maintenanceSuffix) {
				originalName := strings.TrimSuffix(name, maintenanceSuffix)
				statusMap[originalName] = StatusMaintenance
			} else {
				statusMap[nginx.GetConfNameBySymlinkName(name)] = StatusEnabled
			}
		}

		return statusMap
	}
}

// DefaultFilterMatcher provides the standard filtering logic with name search
func DefaultFilterMatcher(fileName string, status Status, namespaceID uint64, options *GenericListOptions) bool {
	// Exact name matching
	if options.Name != "" && !strings.Contains(fileName, options.Name) {
		return false
	}
	if options.Status != "" && status != Status(options.Status) {
		return false
	}
	if options.NamespaceID != 0 && namespaceID != options.NamespaceID {
		return false
	}
	return true
}

// FuzzyFilterMatcher provides filtering logic with fuzzy search support
func FuzzyFilterMatcher(fileName string, status Status, namespaceID uint64, options *GenericListOptions) bool {
	// Exact name matching takes precedence over fuzzy search
	if options.Name != "" && fileName != options.Name {
		return false
	}
	if options.Status != "" && status != Status(options.Status) {
		return false
	}
	if options.NamespaceID != 0 && namespaceID != options.NamespaceID {
		return false
	}
	return true
}

// DefaultConfigBuilder provides basic config building logic
func DefaultConfigBuilder(fileName string, fileInfo os.FileInfo, status Status, namespaceID uint64, namespace *model.Namespace) Config {
	return Config{
		Name:        fileName,
		ModifiedAt:  fileInfo.ModTime(),
		Size:        fileInfo.Size(),
		IsDir:       fileInfo.IsDir(),
		Status:      status,
		NamespaceID: namespaceID,
		Namespace:   namespace,
	}
}
