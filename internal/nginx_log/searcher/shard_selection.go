package searcher

import (
	"github.com/blevesearch/bleve/v2"
)

func indexAliasForLogPaths(
	defaultAlias bleve.IndexAlias,
	shards []bleve.Index,
	useMainLogPath bool,
	logPaths []string,
) (bleve.IndexAlias, func()) {
	if !useMainLogPath || len(logPaths) == 0 {
		return defaultAlias, func() {}
	}

	requested := make(map[string]struct{}, len(logPaths))
	for _, path := range logPaths {
		requested[path] = struct{}{}
	}
	selected := make([]bleve.Index, 0, len(shards))
	for _, shard := range shards {
		if _, ok := requested[shard.Name()]; ok {
			selected = append(selected, shard)
		}
	}
	if len(selected) == 0 || len(selected) == len(shards) {
		return defaultAlias, func() {}
	}

	alias := bleve.NewIndexAlias(selected...)
	if len(selected) > 0 {
		_ = alias.SetIndexMapping(selected[0].Mapping())
	}
	return alias, func() { _ = alias.Close() }
}
