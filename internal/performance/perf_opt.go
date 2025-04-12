package performance

import (
	"os"
	"sort"

	ngxConfig "github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

type ProxyCacheConfig struct {
	Enabled          bool   `json:"enabled"`
	Path             string `json:"path"`              // Cache file path
	Levels           string `json:"levels"`            // Cache directory levels
	UseTempPath      string `json:"use_temp_path"`     // Use temporary path (on/off)
	KeysZone         string `json:"keys_zone"`         // Shared memory zone name and size
	Inactive         string `json:"inactive"`          // Time after which inactive cache is removed
	MaxSize          string `json:"max_size"`          // Maximum size of cache
	MinFree          string `json:"min_free"`          // Minimum free space
	ManagerFiles     string `json:"manager_files"`     // Number of files processed by manager
	ManagerSleep     string `json:"manager_sleep"`     // Manager check interval
	ManagerThreshold string `json:"manager_threshold"` // Manager processing threshold
	LoaderFiles      string `json:"loader_files"`      // Number of files loaded at once
	LoaderSleep      string `json:"loader_sleep"`      // Loader check interval
	LoaderThreshold  string `json:"loader_threshold"`  // Loader processing threshold

	// Additionally, the following parameters are available as part of nginx commercial subscription:
	// Purger           string `json:"purger"`            // Enable cache purger (on/off)
	// PurgerFiles      string `json:"purger_files"`      // Number of files processed by purger
	// PurgerSleep      string `json:"purger_sleep"`      // Purger check interval
	// PurgerThreshold  string `json:"purger_threshold"`  // Purger processing threshold
}

// PerfOpt represents Nginx performance optimization settings
type PerfOpt struct {
	WorkerProcesses           string           `json:"worker_processes"`              // auto or number
	WorkerConnections         string           `json:"worker_connections"`            // max connections
	KeepaliveTimeout          string           `json:"keepalive_timeout"`             // timeout in seconds
	Gzip                      string           `json:"gzip"`                          // on or off
	GzipMinLength             string           `json:"gzip_min_length"`               // min length to compress
	GzipCompLevel             string           `json:"gzip_comp_level"`               // compression level
	ClientMaxBodySize         string           `json:"client_max_body_size"`          // max body size (with unit: k, m, g)
	ServerNamesHashBucketSize string           `json:"server_names_hash_bucket_size"` // hash bucket size
	ClientHeaderBufferSize    string           `json:"client_header_buffer_size"`     // header buffer size (with unit: k, m, g)
	ClientBodyBufferSize      string           `json:"client_body_buffer_size"`       // body buffer size (with unit: k, m, g)
	ProxyCache                ProxyCacheConfig `json:"proxy_cache,omitzero"`          // proxy cache settings
}

// UpdatePerfOpt updates the Nginx performance optimization settings
func UpdatePerfOpt(opt *PerfOpt) error {
	confPath := nginx.GetConfPath("nginx.conf")
	if confPath == "" {
		return errors.New("failed to get nginx.conf path")
	}

	// Read the current configuration
	content, err := os.ReadFile(confPath)
	if err != nil {
		return errors.Wrap(err, "failed to read nginx.conf")
	}

	// Parse the configuration
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	conf, err := p.Parse()
	if err != nil {
		return errors.Wrap(err, "failed to parse nginx.conf")
	}

	// Process the configuration and update performance settings
	updateNginxConfig(conf.Block, opt)

	// Dump the updated configuration
	updatedConf := dumper.DumpBlock(conf.Block, dumper.IndentedStyle)

	return ngxConfig.Save(confPath, updatedConf, nil)

}

// updateNginxConfig updates the performance settings in the Nginx configuration
func updateNginxConfig(block config.IBlock, opt *PerfOpt) {
	if block == nil {
		return
	}

	directives := block.GetDirectives()
	// Update main context directives
	updateOrAddDirective(block, directives, "worker_processes", opt.WorkerProcesses)

	// Look for events, http, and other blocks
	for _, directive := range directives {
		if directive.GetName() == "events" && directive.GetBlock() != nil {
			// Update events block directives
			eventsBlock := directive.GetBlock()
			eventsDirectives := eventsBlock.GetDirectives()
			updateOrAddDirective(eventsBlock, eventsDirectives, "worker_connections", opt.WorkerConnections)
		} else if directive.GetName() == "http" && directive.GetBlock() != nil {
			// Update http block directives
			httpBlock := directive.GetBlock()
			httpDirectives := httpBlock.GetDirectives()
			updateOrAddDirective(httpBlock, httpDirectives, "keepalive_timeout", opt.KeepaliveTimeout)
			updateOrAddDirective(httpBlock, httpDirectives, "gzip", opt.Gzip)
			updateOrAddDirective(httpBlock, httpDirectives, "gzip_min_length", opt.GzipMinLength)
			updateOrAddDirective(httpBlock, httpDirectives, "gzip_comp_level", opt.GzipCompLevel)
			updateOrAddDirective(httpBlock, httpDirectives, "client_max_body_size", opt.ClientMaxBodySize)
			updateOrAddDirective(httpBlock, httpDirectives, "server_names_hash_bucket_size", opt.ServerNamesHashBucketSize)
			updateOrAddDirective(httpBlock, httpDirectives, "client_header_buffer_size", opt.ClientHeaderBufferSize)
			updateOrAddDirective(httpBlock, httpDirectives, "client_body_buffer_size", opt.ClientBodyBufferSize)

			// Handle proxy_cache_path directive
			updateOrRemoveProxyCachePath(httpBlock, httpDirectives, &opt.ProxyCache)

			sortDirectives(httpDirectives)
		}
	}
}

// updateOrAddDirective updates a directive if it exists, or adds it to the block if it doesn't
func updateOrAddDirective(block config.IBlock, directives []config.IDirective, name string, value string) {
	if value == "" {
		return
	}

	// Search for existing directive
	for _, directive := range directives {
		if directive.GetName() == name {
			// Update existing directive
			if len(directive.GetParameters()) > 0 {
				directive.GetParameters()[0].Value = value
			}
			return
		}
	}

	// If we get here, we need to add a new directive
	// Create a new directive and add it to the block
	// This requires knowledge of the underlying implementation
	// For now, we'll use the Directive type from gonginx/config
	newDirective := &config.Directive{
		Name:       name,
		Parameters: []config.Parameter{{Value: value}},
	}

	// Add the new directive to the block
	// This is specific to the gonginx library implementation
	switch block := block.(type) {
	case *config.Config:
		block.Block.Directives = append(block.Block.Directives, newDirective)
	case *config.Block:
		block.Directives = append(block.Directives, newDirective)
	case *config.HTTP:
		block.Directives = append(block.Directives, newDirective)
	}
}

// sortDirectives sorts directives alphabetically by name
func sortDirectives(directives []config.IDirective) {
	sort.SliceStable(directives, func(i, j int) bool {
		// Ensure both i and j can return valid names
		return directives[i].GetName() < directives[j].GetName()
	})
}

// updateOrRemoveProxyCachePath adds or removes the proxy_cache_path directive based on whether it's enabled
func updateOrRemoveProxyCachePath(block config.IBlock, directives []config.IDirective, proxyCache *ProxyCacheConfig) {
	// If not enabled, remove the directive if it exists
	if !proxyCache.Enabled {
		for i, directive := range directives {
			if directive.GetName() == "proxy_cache_path" {
				// Remove the directive
				switch block := block.(type) {
				case *config.Block:
					block.Directives = append(block.Directives[:i], block.Directives[i+1:]...)
				case *config.HTTP:
					block.Directives = append(block.Directives[:i], block.Directives[i+1:]...)
				}
				return
			}
		}
		return
	}

	// If enabled, build the proxy_cache_path directive with all parameters
	params := []config.Parameter{}

	// First parameter is the path (required)
	if proxyCache.Path != "" {
		params = append(params, config.Parameter{Value: proxyCache.Path})
		_ = os.MkdirAll(proxyCache.Path, 0755)
	} else {
		// No path specified, can't add the directive
		return
	}

	// Add optional parameters
	if proxyCache.Levels != "" {
		params = append(params, config.Parameter{Value: "levels=" + proxyCache.Levels})
	}

	if proxyCache.UseTempPath != "" {
		params = append(params, config.Parameter{Value: "use_temp_path=" + proxyCache.UseTempPath})
	}

	if proxyCache.KeysZone != "" {
		params = append(params, config.Parameter{Value: "keys_zone=" + proxyCache.KeysZone})
	} else {
		// keys_zone is required, can't add the directive without it
		return
	}

	if proxyCache.Inactive != "" {
		params = append(params, config.Parameter{Value: "inactive=" + proxyCache.Inactive})
	}

	if proxyCache.MaxSize != "" {
		params = append(params, config.Parameter{Value: "max_size=" + proxyCache.MaxSize})
	}

	if proxyCache.MinFree != "" {
		params = append(params, config.Parameter{Value: "min_free=" + proxyCache.MinFree})
	}

	if proxyCache.ManagerFiles != "" {
		params = append(params, config.Parameter{Value: "manager_files=" + proxyCache.ManagerFiles})
	}

	if proxyCache.ManagerSleep != "" {
		params = append(params, config.Parameter{Value: "manager_sleep=" + proxyCache.ManagerSleep})
	}

	if proxyCache.ManagerThreshold != "" {
		params = append(params, config.Parameter{Value: "manager_threshold=" + proxyCache.ManagerThreshold})
	}

	if proxyCache.LoaderFiles != "" {
		params = append(params, config.Parameter{Value: "loader_files=" + proxyCache.LoaderFiles})
	}

	if proxyCache.LoaderSleep != "" {
		params = append(params, config.Parameter{Value: "loader_sleep=" + proxyCache.LoaderSleep})
	}

	if proxyCache.LoaderThreshold != "" {
		params = append(params, config.Parameter{Value: "loader_threshold=" + proxyCache.LoaderThreshold})
	}

	// if proxyCache.Purger != "" {
	// 	params = append(params, config.Parameter{Value: "purger=" + proxyCache.Purger})
	// }

	// if proxyCache.PurgerFiles != "" {
	// 	params = append(params, config.Parameter{Value: "purger_files=" + proxyCache.PurgerFiles})
	// }

	// if proxyCache.PurgerSleep != "" {
	// 	params = append(params, config.Parameter{Value: "purger_sleep=" + proxyCache.PurgerSleep})
	// }

	// if proxyCache.PurgerThreshold != "" {
	// 	params = append(params, config.Parameter{Value: "purger_threshold=" + proxyCache.PurgerThreshold})
	// }

	// Check if directive already exists
	for i, directive := range directives {
		if directive.GetName() == "proxy_cache_path" {
			// Remove the old directive
			switch block := block.(type) {
			case *config.HTTP:
				block.Directives = append(block.Directives[:i], block.Directives[i+1:]...)
			}
			break
		}
	}

	// Create new directive
	newDirective := &config.Directive{
		Name:       "proxy_cache_path",
		Parameters: params,
	}

	// Add the directive to the block
	switch block := block.(type) {
	case *config.HTTP:
		block.Directives = append(block.Directives, newDirective)
	}
}
