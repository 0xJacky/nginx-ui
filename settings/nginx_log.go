package settings

import "time"

type NginxLog struct {
	IndexingEnabled bool   `json:"indexing_enabled"`
	IndexPath       string `json:"index_path"`
	IndexCustomMMDB string `json:"index_custom_mmdb"`
	// IncrementalIndexInterval controls how often the incremental indexing job runs, in minutes.
	// When set to 0 or a negative value, a conservative default will be used.
	IncrementalIndexInterval int `json:"incremental_index_interval"`
	// MaxConcurrentIndexTasks caps how many log groups are indexed at the same
	// time. Each concurrent group buffers a parse batch and an index batch per
	// rotated file, so this is the main lever on peak indexing memory.
	// When set to 0 or a negative value, the value is derived from the CPU
	// budget the process is allowed to use.
	MaxConcurrentIndexTasks int `json:"max_concurrent_index_tasks"`
}

var NginxLogSettings = &NginxLog{}

// GetIncrementalIndexInterval returns the effective incremental indexing interval.
// Defaults to 15 minutes when not configured or configured with an invalid value.
func (n *NginxLog) GetIncrementalIndexInterval() time.Duration {
	if n == nil || n.IncrementalIndexInterval <= 0 {
		return 15 * time.Minute
	}
	return time.Duration(n.IncrementalIndexInterval) * time.Minute
}
