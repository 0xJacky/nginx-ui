package cache

import (
	"time"

	"github.com/uozi-tech/cosy/logger"
)

const (
	NodeCacheKey = "enabled_nodes"
	NodeCacheTTL = 10 * time.Minute
)

// InvalidateNodeCache removes the node cache entry
func InvalidateNodeCache() {
	Del(NodeCacheKey)
	logger.Debug("Invalidated node cache")
}

// GetCachedNodes retrieves nodes from cache
func GetCachedNodes() (interface{}, bool) {
	return Get(NodeCacheKey)
}

// SetCachedNodes stores nodes in cache
func SetCachedNodes(data interface{}) {
	Set(NodeCacheKey, data, NodeCacheTTL)
	logger.Debug("Cached enabled nodes data")
}