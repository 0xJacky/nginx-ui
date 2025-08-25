package cache

import (
	"context"
	"testing"
	"time"
)

func TestNodeCache(t *testing.T) {
	// Initialize cache for testing
	Init(context.Background())

	// Mock nodes data for testing
	mockNodes := []interface{}{
		map[string]interface{}{"id": 1, "name": "node1", "enabled": true},
		map[string]interface{}{"id": 2, "name": "node2", "enabled": true},
	}

	// Test setting cache
	SetCachedNodes(mockNodes)

	// Test getting from cache
	cached, found := GetCachedNodes()
	if !found {
		t.Error("Expected to find cached nodes")
	}

	if cached == nil {
		t.Error("Expected cached nodes to not be nil")
	}

	// Test invalidation
	InvalidateNodeCache()
	_, found = GetCachedNodes()
	if found {
		t.Error("Expected cache to be invalidated")
	}
}

func TestCacheConstants(t *testing.T) {
	if NodeCacheKey != "enabled_nodes" {
		t.Errorf("Expected NodeCacheKey to be 'enabled_nodes', got %s", NodeCacheKey)
	}

	if NodeCacheTTL != 10*time.Minute {
		t.Errorf("Expected NodeCacheTTL to be 10 minutes, got %v", NodeCacheTTL)
	}
}