package settings

import "time"

type NginxLog struct {
	IndexingEnabled bool   `json:"indexing_enabled"`
	IndexPath       string `json:"index_path"`
	// IncrementalIndexInterval controls how often the incremental indexing job runs, in minutes.
	// When set to 0 or a negative value, a conservative default will be used.
	IncrementalIndexInterval int `json:"incremental_index_interval"`
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
