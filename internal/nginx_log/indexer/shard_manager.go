package indexer

import (
	"crypto/md5"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// DefaultShardManager implements sharding logic for distributed indexing
type DefaultShardManager struct {
	config     *Config
	shards     map[int]bleve.Index
	shardPaths map[int]string
	mu         sync.RWMutex
	hashFunc   ShardHashFunc
}

// ShardHashFunc defines how to determine which shard a document belongs to
type ShardHashFunc func(key string, shardCount int) int

// NewDefaultShardManager creates a new shard manager
func NewDefaultShardManager(config *Config) *DefaultShardManager {
	return &DefaultShardManager{
		config:     config,
		shards:     make(map[int]bleve.Index),
		shardPaths: make(map[int]string),
		hashFunc:   DefaultHashFunc,
	}
}

// Initialize sets up all shards
func (sm *DefaultShardManager) Initialize() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i := 0; i < sm.config.ShardCount; i++ {
		if err := sm.createShardLocked(i); err != nil {
			return fmt.Errorf("failed to create shard %d: %w", i, err)
		}
	}

	return nil
}

// GetShard returns the appropriate shard for a given key
func (sm *DefaultShardManager) GetShard(key string) (bleve.Index, int, error) {
	shardID := sm.hashFunc(key, sm.config.ShardCount)
	index, err := sm.GetShardByID(shardID)
	return index, shardID, err
}

// GetShardByID returns the shard with the given ID
func (sm *DefaultShardManager) GetShardByID(id int) (bleve.Index, error) {
	sm.mu.RLock()
	shard, exists := sm.shards[id]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%s: %d", ErrShardNotFound, id)
	}

	return shard, nil
}

// GetAllShards returns all active shards
func (sm *DefaultShardManager) GetAllShards() []bleve.Index {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shards := make([]bleve.Index, 0, len(sm.shards))
	for i := 0; i < sm.config.ShardCount; i++ {
		if shard, exists := sm.shards[i]; exists {
			shards = append(shards, shard)
		}
	}

	return shards
}

// GetShardStats returns statistics for all shards
func (sm *DefaultShardManager) GetShardStats() []*ShardInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := make([]*ShardInfo, 0, len(sm.shards))
	for id, shard := range sm.shards {
		if shard == nil {
			continue
		}

		docCount, _ := shard.DocCount()

		var size int64
		if path, exists := sm.shardPaths[id]; exists {
			if stat, err := os.Stat(path); err == nil {
				size = stat.Size()
			}
		}

		stats = append(stats, &ShardInfo{
			ID:            id,
			Path:          sm.shardPaths[id],
			DocumentCount: docCount,
			Size:          size,
			LastUpdated:   0, // TODO: Track last update time
		})
	}

	return stats
}

// CreateShard creates a new shard with the given ID
func (sm *DefaultShardManager) CreateShard(id int, path string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.createShardLocked(id)
}

// createShardLocked creates a shard while holding the lock
func (sm *DefaultShardManager) createShardLocked(id int) error {
	// Use efficient string building for shard path
	shardNameBuf := make([]byte, 0, 16)
	shardNameBuf = append(shardNameBuf, "shard_"...)
	shardNameBuf = utils.AppendInt(shardNameBuf, id)
	shardName := utils.BytesToStringUnsafe(shardNameBuf)
	shardPath := filepath.Join(sm.config.IndexPath, shardName)

	// Ensure directory exists
	if err := os.MkdirAll(shardPath, 0755); err != nil {
		return fmt.Errorf("failed to create shard directory: %w", err)
	}

	// Create or open the shard index
	var shard bleve.Index
	var err error

	if _, statErr := os.Stat(filepath.Join(shardPath, "index_meta.json")); os.IsNotExist(statErr) {
		// Create new index with optimized disk space configuration
		mapping := CreateLogIndexMapping()
		
		// Optimize FloorSegmentFileSize for better disk space usage
		// FloorSegmentFileSize controls the minimum size of segment files.
		// Larger values reduce file fragmentation and improve I/O efficiency,
		// which can save disk space by reducing metadata overhead.
		// 5MB provides a good balance between space efficiency and performance.
		kvConfig := map[string]interface{}{
			"scorchMergePlanOptions": map[string]interface{}{
				"FloorSegmentFileSize": 5000000, // 5MB minimum segment file size
			},
		}
		
		shard, err = bleve.NewUsing(shardPath, mapping, bleve.Config.DefaultIndexType, bleve.Config.DefaultMemKVStore, kvConfig)
		if err != nil {
			return fmt.Errorf("failed to create new shard index: %w", err)
		}
	} else {
		// Open existing index
		shard, err = bleve.Open(shardPath)
		if err != nil {
			return fmt.Errorf("failed to open existing shard index: %w", err)
		}
	}

	sm.shards[id] = shard
	sm.shardPaths[id] = shardPath

	return nil
}

// CloseShard closes a shard and removes it from the manager
func (sm *DefaultShardManager) CloseShard(id int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	return sm.closeShardLocked(id)
}

// closeShardLocked closes a shard while already holding the lock
func (sm *DefaultShardManager) closeShardLocked(id int) error {
	shard, exists := sm.shards[id]
	if !exists {
		return fmt.Errorf("%s: %d", ErrShardNotFound, id)
	}

	if err := shard.Close(); err != nil {
		return fmt.Errorf("failed to close shard %d: %w", id, err)
	}

	delete(sm.shards, id)
	delete(sm.shardPaths, id)

	return nil
}

// OptimizeShard optimizes a specific shard
func (sm *DefaultShardManager) OptimizeShard(id int) error {
	shard, err := sm.GetShardByID(id)
	if err != nil {
		return err
	}

	// Bleve doesn't have a direct optimize method, but we can trigger
	// internal optimizations by forcing a merge
	return shard.SetInternal([]byte("_optimize"), []byte("trigger"))
}

// Close closes all shards
func (sm *DefaultShardManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var errs []error
	for id, shard := range sm.shards {
		if err := shard.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close shard %d: %w", id, err))
		}
	}

	sm.shards = make(map[int]bleve.Index)
	sm.shardPaths = make(map[int]string)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing shards: %v", errs)
	}

	return nil
}

// Hash functions for shard selection

// DefaultHashFunc uses FNV-1a hash for shard distribution
func DefaultHashFunc(key string, shardCount int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % shardCount
}

// MD5HashFunc uses MD5 hash for shard distribution
func MD5HashFunc(key string, shardCount int) int {
	h := md5.Sum([]byte(key))
	// Use first 4 bytes as uint32
	val := uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
	return int(val) % shardCount
}

// IPHashFunc optimized for IP address distribution
func IPHashFunc(key string, shardCount int) int {
	// For IP addresses, use the last octet for better distribution
	h := fnv.New32a()

	// If key looks like an IP, hash the last part more heavily
	if len(key) > 7 && key[len(key)-4:] != key[:4] {
		// Weight the end of the string more (likely the varying part of IP)
		for i, b := range []byte(key) {
			if i >= len(key)/2 {
				h.Write([]byte{b, b}) // Double weight for later characters
			} else {
				h.Write([]byte{b})
			}
		}
	} else {
		h.Write([]byte(key))
	}

	return int(h.Sum32()) % shardCount
}

// TimestampHashFunc distributes based on timestamp ranges
func TimestampHashFunc(timestamp int64, shardCount int) int {
	// Distribute by hour to keep related time periods together
	hourBucket := timestamp / 3600 // Unix timestamp to hour bucket
	result := int(hourBucket) % shardCount
	if result < 0 {
		result = -result
	}
	return result
}

// ConsistentHashFunc provides consistent hashing for better distribution
func ConsistentHashFunc(key string, shardCount int) int {
	// Simple consistent hashing - can be enhanced with hash ring
	h1 := fnv.New64a()
	h1.Write([]byte(key))
	hash1 := h1.Sum64()

	h2 := fnv.New64()
	h2.Write([]byte(key + "_salt"))
	hash2 := h2.Sum64()

	// Combine hashes for better distribution
	combined := hash1 ^ hash2
	result := int(combined) % shardCount
	if result < 0 {
		result = -result
	}
	return result
}

// SetHashFunc allows changing the hash function
func (sm *DefaultShardManager) SetHashFunc(fn ShardHashFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.hashFunc = fn
}

// GetShardDistribution returns the current distribution of documents across shards
func (sm *DefaultShardManager) GetShardDistribution() map[int]uint64 {
	stats := sm.GetShardStats()
	distribution := make(map[int]uint64)

	for _, stat := range stats {
		distribution[stat.ID] = stat.DocumentCount
	}

	return distribution
}

// HealthCheck verifies all shards are accessible
func (sm *DefaultShardManager) HealthCheck() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for id, shard := range sm.shards {
		if shard == nil {
			return fmt.Errorf("shard %d is nil", id)
		}

		// Try a simple operation to verify accessibility
		if _, err := shard.DocCount(); err != nil {
			return fmt.Errorf("shard %d health check failed: %w", id, err)
		}
	}

	return nil
}

// Destroy closes all shards and deletes their data from disk.
func (sm *DefaultShardManager) Destroy() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// First, close all shards
	var errs []error
	for id, shard := range sm.shards {
		if err := shard.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close shard %d for deletion: %w", id, err))
		}
	}

	// Then, delete all shard directories
	for _, path := range sm.shardPaths {
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete shard directory %s: %w", path, err))
		}
	}

	// Reset internal state
	sm.shards = make(map[int]bleve.Index)
	sm.shardPaths = make(map[int]string)

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while destroying shards: %v", errs)
	}

	return nil
}
