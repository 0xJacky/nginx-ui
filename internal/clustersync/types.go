// Package clustersync replicates local Nginx UI content to the nodes of a
// cluster. It is the single engine behind directory deployment, one-click node
// synchronization, namespace replication and the periodic auto sync job.
package clustersync

import (
	"sort"
	"sync"
)

// Kind identifies the type of an item pushed to a node.
type Kind string

const (
	// KindConfig is a plain configuration file below the Nginx config directory.
	KindConfig Kind = "config"
	// KindSite is a file in sites-available together with its enabled state.
	KindSite Kind = "site"
	// KindStream is a file in streams-available together with its enabled state.
	KindStream Kind = "stream"
	// KindNamespace is the namespace record itself.
	KindNamespace Kind = "namespace"
)

// Result reports the outcome of a single item on a single node.
type Result struct {
	NodeID  uint64 `json:"node_id"`
	Node    string `json:"node"`
	Kind    Kind   `json:"kind"`
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	// SkippedExisting counts files the node left alone because they already
	// exist there and overwrite was off. SkippedPaths names them when the node
	// is new enough to report it.
	SkippedExisting int      `json:"skipped_existing,omitempty"`
	SkippedPaths    []string `json:"skipped_paths,omitempty"`
}

// SkipReason explains why a local file was left out of a synchronization run.
type SkipReason string

const (
	// SkipUnsupportedType is a file whose name or extension cannot be replicated.
	SkipUnsupportedType SkipReason = "unsupported_type"
	// SkipTooLarge is a file above the size limit.
	SkipTooLarge SkipReason = "too_large"
	// SkipNotText is a binary file.
	SkipNotText SkipReason = "not_text"
	// SkipUnreadable is a file that could not be read.
	SkipUnreadable SkipReason = "unreadable"
	// SkipEntryConfig is the node specific nginx.conf entry file.
	SkipEntryConfig SkipReason = "entry_config"
)

// SkippedFile is a local file that was not replicated.
type SkippedFile struct {
	Path   string     `json:"path"`
	Reason SkipReason `json:"reason"`
}

// Summary aggregates every result of one synchronization run.
type Summary struct {
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Results   []Result `json:"results"`
	// Skipped lists local files that were left out and why.
	Skipped []SkippedFile `json:"skipped"`
}

// Scope selects which content a synchronization run replicates.
type Scope struct {
	Configs bool `json:"configs"`
	Sites   bool `json:"sites"`
	Streams bool `json:"streams"`
	// Overwrite replaces files that already exist on the target node.
	Overwrite bool `json:"overwrite"`
}

// IsEmpty reports whether the scope would replicate nothing at all.
func (s Scope) IsEmpty() bool {
	return !s.Configs && !s.Sites && !s.Streams
}

// FullScope replicates every kind of content and overwrites existing files. It
// is what a one-click "sync everything to this node" action needs.
func FullScope() Scope {
	return Scope{Configs: true, Sites: true, Streams: true, Overwrite: true}
}

// collector accumulates results from the per-node goroutines.
type collector struct {
	mutex   sync.Mutex
	results []Result
}

func (c *collector) add(result Result) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.results = append(c.results, result)
}

func (c *collector) ok(node nodeRef, kind Kind, name string) {
	c.add(Result{NodeID: node.id, Node: node.name, Kind: kind, Name: name, Success: true})
}

func (c *collector) okWith(node nodeRef, kind Kind, name string, outcome outcome) {
	c.add(Result{
		NodeID: node.id, Node: node.name, Kind: kind, Name: name, Success: true,
		SkippedExisting: outcome.skippedExisting, SkippedPaths: outcome.skippedPaths,
	})
}

func (c *collector) fail(node nodeRef, kind Kind, name string, err error) {
	c.add(Result{NodeID: node.id, Node: node.name, Kind: kind, Name: name, Error: err.Error()})
}

// summary sorts the collected results deterministically and counts them.
func (c *collector) summary() *Summary {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	sort.SliceStable(c.results, func(i, j int) bool {
		if c.results[i].Node != c.results[j].Node {
			return c.results[i].Node < c.results[j].Node
		}
		if c.results[i].Kind != c.results[j].Kind {
			return c.results[i].Kind < c.results[j].Kind
		}
		return c.results[i].Name < c.results[j].Name
	})

	summary := &Summary{Results: c.results, Total: len(c.results), Skipped: []SkippedFile{}}
	for _, result := range c.results {
		if result.Success {
			summary.Succeeded++
			continue
		}
		summary.Failed++
	}

	if summary.Results == nil {
		summary.Results = []Result{}
	}

	return summary
}
