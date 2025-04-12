package config

import (
	"os"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

// PerfOpt represents Nginx performance optimization settings
type PerfOpt struct {
	WorkerProcesses           string `json:"worker_processes"`              // auto or number
	WorkerConnections         string `json:"worker_connections"`            // max connections
	KeepaliveTimeout          string `json:"keepalive_timeout"`             // timeout in seconds
	Gzip                      string `json:"gzip"`                          // on or off
	GzipMinLength             string `json:"gzip_min_length"`               // min length to compress
	GzipCompLevel             string `json:"gzip_comp_level"`               // compression level
	ClientMaxBodySize         string `json:"client_max_body_size"`          // max body size (with unit: k, m, g)
	ServerNamesHashBucketSize string `json:"server_names_hash_bucket_size"` // hash bucket size
	ClientHeaderBufferSize    string `json:"client_header_buffer_size"`     // header buffer size (with unit: k, m, g)
	ClientBodyBufferSize      string `json:"client_body_buffer_size"`       // body buffer size (with unit: k, m, g)
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

	return Save(confPath, updatedConf, nil)

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
