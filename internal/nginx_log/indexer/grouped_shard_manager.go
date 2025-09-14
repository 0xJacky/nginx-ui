package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

// GroupedShardManager manages shards grouped by MainLogPath. Each group uses a
// unique UUID directory:
//
//	index_path/<uuid>/shard_{N}
//
// Key points:
// - Lazy group creation: on first write for a MainLogPath we resolve/create UUID and shards
// - GetAllShards returns all shards across groups with a stable global shard ID mapping
// - Destroy removes all index data
// - Concurrency-safe with RWMutex
type GroupedShardManager struct {
	config *Config

	mu sync.RWMutex

	// mainLogPath -> uuid
	pathToUUID map[string]string

	// uuid -> group
	groups map[string]*ShardGroup

	// Stable global shard mapping (contiguous IDs across groups)
	// globalID -> (uuid, localShardID)
	globalToLocal map[int]groupShardRef
	// key: uuid#localID -> globalID
	localToGlobal map[string]int
	// Next available global shard ID
	nextGlobalID int
}

type groupShardRef struct {
	uuid    string
	localID int
}

// ShardGroup represents a shard set belonging to a single log group (identified by uuid)
type ShardGroup struct {
	UUID        string
	MainLogPath string
	Shards      map[int]bleve.Index
	ShardPaths  map[int]string
	ShardCount  int
}

// NewGroupedShardManager creates a new grouped shard manager
func NewGroupedShardManager(config *Config) *GroupedShardManager {
	if config == nil {
		config = DefaultIndexerConfig()
	}
	return &GroupedShardManager{
		config:        config,
		pathToUUID:    make(map[string]string),
		groups:        make(map[string]*ShardGroup),
		globalToLocal: make(map[int]groupShardRef),
		localToGlobal: make(map[string]int),
	}
}

// Initialize ensures the index root directory exists. Groups are created on demand.
func (gsm *GroupedShardManager) Initialize() error {
	if err := os.MkdirAll(gsm.config.IndexPath, 0755); err != nil {
		return fmt.Errorf("failed to create index root: %w", err)
	}

	// Load existing shard groups from DB/disk so searcher has shards after restart
	if err := gsm.loadExistingGroups(); err != nil {
		// Non-fatal: continue even if loading fails; indexing will lazily create groups
		logger.Warnf("Failed to load existing shard groups: %v", err)
	}
	return nil
}

// Close closes all open shards across groups.
func (gsm *GroupedShardManager) Close() error {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	var errs []error
	for _, group := range gsm.groups {
		for id, shard := range group.Shards {
			if shard == nil {
				continue
			}
			if err := shard.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close shard %d in group %s: %w", id, group.UUID, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing grouped shards: %v", errs)
	}
	return nil
}

// HealthCheck verifies all opened shards are accessible.
func (gsm *GroupedShardManager) HealthCheck() error {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()

	for _, group := range gsm.groups {
		for id, shard := range group.Shards {
			if shard == nil {
				return fmt.Errorf("shard %d in group %s is nil", id, group.UUID)
			}
			if _, err := shard.DocCount(); err != nil {
				return fmt.Errorf("health check failed for shard %d in group %s: %w", id, group.UUID, err)
			}
		}
	}
	return nil
}

// GetShardForDocument routes to the corresponding shard within a group based on MainLogPath and key.
func (gsm *GroupedShardManager) GetShardForDocument(mainLogPath string, key string) (bleve.Index, int, error) {
	if mainLogPath == "" {
		return nil, -1, fmt.Errorf("mainLogPath required for grouped shard routing")
	}

	group, err := gsm.getOrCreateGroup(mainLogPath)
	if err != nil {
		return nil, -1, err
	}
	shardID := defaultHashFunc(key, group.ShardCount)
	shard := group.Shards[shardID]
	if shard == nil {
		return nil, -1, fmt.Errorf("shard %d not initialized for group %s", shardID, group.UUID)
	}
	return shard, shardID, nil
}

// GetShard is a compatibility interface: only available when there is exactly one group.
func (gsm *GroupedShardManager) GetShard(key string) (bleve.Index, int, error) {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()

	if len(gsm.groups) == 0 {
		return nil, -1, fmt.Errorf("no shard groups initialized")
	}
	if len(gsm.groups) > 1 {
		return nil, -1, fmt.Errorf("ambiguous GetShard: multiple shard groups present; use GetShardForDocument")
	}
	for _, group := range gsm.groups {
		shardID := defaultHashFunc(key, group.ShardCount)
		shard := group.Shards[shardID]
		if shard == nil {
			return nil, -1, fmt.Errorf("shard %d not initialized for group %s", shardID, group.UUID)
		}
		return shard, shardID, nil
	}
	return nil, -1, fmt.Errorf("unexpected: no groups iterated")
}

// GetShardByID resolves to a specific group and local shard ID using a global ID.
func (gsm *GroupedShardManager) GetShardByID(id int) (bleve.Index, error) {
	gsm.mu.RLock()
	ref, ok := gsm.globalToLocal[id]
	if !ok {
		gsm.mu.RUnlock()
		return nil, fmt.Errorf("%s: %d", ErrShardNotFound, id)
	}
	group := gsm.groups[ref.uuid]
	shard := group.Shards[ref.localID]
	gsm.mu.RUnlock()
	if shard == nil {
		return nil, fmt.Errorf("%s: %d", ErrShardNotFound, id)
	}
	return shard, nil
}

// GetAllShards returns all shards across groups, sorted by global ID.
func (gsm *GroupedShardManager) GetAllShards() []bleve.Index {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()

	ids := make([]int, 0, len(gsm.globalToLocal))
	for id := range gsm.globalToLocal {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	shards := make([]bleve.Index, 0, len(ids))
	for _, gid := range ids {
		ref := gsm.globalToLocal[gid]
		if grp, ok := gsm.groups[ref.uuid]; ok {
			shards = append(shards, grp.Shards[ref.localID])
		}
	}
	return shards
}

// GetShardStats summarizes shard statistics across groups, using global IDs.
func (gsm *GroupedShardManager) GetShardStats() []*ShardInfo {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()

	ids := make([]int, 0, len(gsm.globalToLocal))
	for id := range gsm.globalToLocal {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	stats := make([]*ShardInfo, 0, len(ids))
	for _, gid := range ids {
		ref := gsm.globalToLocal[gid]
		group := gsm.groups[ref.uuid]
		shard := group.Shards[ref.localID]
		if shard == nil {
			continue
		}
		docCount, _ := shard.DocCount()

		path := group.ShardPaths[ref.localID]
		var size int64
		if st, err := os.Stat(path); err == nil {
			size = st.Size()
		}

		stats = append(stats, &ShardInfo{
			ID:            gid,
			Path:          path,
			DocumentCount: docCount,
			Size:          size,
			LastUpdated:   0,
		})
	}
	return stats
}

// CreateShard is not directly supported by global ID in grouped manager.
func (gsm *GroupedShardManager) CreateShard(id int, path string) error {
	return fmt.Errorf("CreateShard by global id is not supported in grouped manager")
}

// CloseShard closes a shard by global ID.
func (gsm *GroupedShardManager) CloseShard(id int) error {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()
	ref, ok := gsm.globalToLocal[id]
	if !ok {
		return fmt.Errorf("%s: %d", ErrShardNotFound, id)
	}
	group := gsm.groups[ref.uuid]
	shard := group.Shards[ref.localID]
	if shard == nil {
		return fmt.Errorf("%s: %d", ErrShardNotFound, id)
	}
	if err := shard.Close(); err != nil {
		return fmt.Errorf("failed to close shard %d in group %s: %w", ref.localID, ref.uuid, err)
	}
	delete(group.Shards, ref.localID)
	delete(group.ShardPaths, ref.localID)
	delete(gsm.globalToLocal, id)
	delete(gsm.localToGlobal, gsm.makeLocalKey(ref.uuid, ref.localID))
	return nil
}

// OptimizeShard triggers internal optimization by global ID.
func (gsm *GroupedShardManager) OptimizeShard(id int) error {
	shard, err := gsm.GetShardByID(id)
	if err != nil {
		return err
	}
	return shard.SetInternal([]byte("_optimize"), []byte("trigger"))
}

// Destroy removes all index data for all groups.
func (gsm *GroupedShardManager) Destroy() error {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	var errs []error
	for _, group := range gsm.groups {
		for id, shard := range group.Shards {
			if shard != nil {
				if err := shard.Close(); err != nil {
					errs = append(errs, fmt.Errorf("close shard %d in %s: %w", id, group.UUID, err))
				}
			}
		}
		// Delete directory
		groupPath := filepath.Join(gsm.config.IndexPath, group.UUID)
		if err := os.RemoveAll(groupPath); err != nil {
			errs = append(errs, fmt.Errorf("remove group path %s: %w", groupPath, err))
		}
	}

	// Extra safety: remove any residual entries under the index root that are not tracked in memory.
	// This ensures a clean slate in case of leftovers from previous runs/crashes.
	if gsm.config != nil && gsm.config.IndexPath != "" {
		entries, err := os.ReadDir(gsm.config.IndexPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("read index root %s: %w", gsm.config.IndexPath, err))
		} else {
			for _, entry := range entries {
				residualPath := filepath.Join(gsm.config.IndexPath, entry.Name())
				if err := os.RemoveAll(residualPath); err != nil {
					errs = append(errs, fmt.Errorf("remove residual path %s: %w", residualPath, err))
				}
			}
		}
	}

	// Reset state
	gsm.groups = make(map[string]*ShardGroup)
	gsm.globalToLocal = make(map[int]groupShardRef)
	gsm.localToGlobal = make(map[string]int)
	gsm.nextGlobalID = 0

	if len(errs) > 0 {
		return fmt.Errorf("destroy errors: %v", errs)
	}
	return nil
}

// Helper: get or create group
func (gsm *GroupedShardManager) getOrCreateGroup(mainLogPath string) (*ShardGroup, error) {
	// First check cache
	gsm.mu.RLock()
	if uuid, ok := gsm.pathToUUID[mainLogPath]; ok {
		if group, exists := gsm.groups[uuid]; exists {
			gsm.mu.RUnlock()
			return group, nil
		}
	}
	gsm.mu.RUnlock()

	// Cache miss, resolve/create UUID
	uuidStr, err := gsm.getOrCreateUUID(mainLogPath)
	if err != nil {
		return nil, err
	}

	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	if group, exists := gsm.groups[uuidStr]; exists {
		return group, nil
	}

	// Initialize group directory and shards
	group := &ShardGroup{
		UUID:        uuidStr,
		MainLogPath: mainLogPath,
		Shards:      make(map[int]bleve.Index),
		ShardPaths:  make(map[int]string),
		ShardCount:  gsm.config.ShardCount,
	}

	groupBase := filepath.Join(gsm.config.IndexPath, uuidStr)
	if err := os.MkdirAll(groupBase, 0755); err != nil {
		return nil, fmt.Errorf("failed to create group base %s: %w", groupBase, err)
	}

	for i := 0; i < group.ShardCount; i++ {
		shard, shardPath, err := gsm.openOrCreateShard(groupBase, i)
		if err != nil {
			return nil, fmt.Errorf("failed to init shard %d for group %s: %w", i, uuidStr, err)
		}
		group.Shards[i] = shard
		group.ShardPaths[i] = shardPath

		// Assign global ID
		gID := gsm.nextGlobalID
		gsm.globalToLocal[gID] = groupShardRef{uuid: uuidStr, localID: i}
		gsm.localToGlobal[gsm.makeLocalKey(uuidStr, i)] = gID
		gsm.nextGlobalID++
	}

	gsm.groups[uuidStr] = group
	gsm.pathToUUID[mainLogPath] = uuidStr

	logger.Infof("Initialized shard group %s for mainLogPath %s with %d shards", uuidStr, mainLogPath, group.ShardCount)

	return group, nil
}

func (gsm *GroupedShardManager) openOrCreateShard(groupBase string, shardID int) (bleve.Index, string, error) {
	// shard path: groupBase/shard_{id}
	name := make([]byte, 0, 16)
	name = append(name, "shard_"...)
	name = append(name, []byte(fmt.Sprintf("%d", shardID))...)
	shardName := string(name)
	shardPath := filepath.Join(groupBase, shardName)

	if err := os.MkdirAll(shardPath, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create shard dir: %w", err)
	}

	var idx bleve.Index
	var err error
	if _, statErr := os.Stat(filepath.Join(shardPath, "index_meta.json")); os.IsNotExist(statErr) {
		// New index, reuse original mapping and storage optimizations
		mapping := CreateLogIndexMapping()
		kvConfig := map[string]interface{}{
			"scorchMergePlanOptions": map[string]interface{}{
				"FloorSegmentFileSize": 5000000,
			},
		}
		idx, err = bleve.NewUsing(shardPath, mapping, bleve.Config.DefaultIndexType, bleve.Config.DefaultMemKVStore, kvConfig)
		if err != nil {
			return nil, "", fmt.Errorf("create bleve index: %w", err)
		}
	} else {
		idx, err = bleve.Open(shardPath)
		if err != nil {
			return nil, "", fmt.Errorf("open bleve index: %w", err)
		}
	}
	return idx, shardPath, nil
}

// loadExistingGroups scans the database for existing main_log_path groups and opens their shards.
// This ensures that after process restart, previously built indices are immediately available.
func (gsm *GroupedShardManager) loadExistingGroups() error {
	// Fetch all enabled index records ordered by creation time so the first seen per main_log_path
	// becomes the canonical UUID for the group (consistent with getOrCreateUUID logic)
	q := query.NginxLogIndex
	records, err := q.Where(q.Enabled.Is(true)).Order(q.CreatedAt).Find()
	if err != nil {
		return fmt.Errorf("failed to query existing index records: %w", err)
	}

	// Build unique main_log_path -> uuid mapping
	type groupInfo struct {
		uuid string
		path string
	}
	groups := make(map[string]groupInfo)
	for _, rec := range records {
		if rec == nil || rec.MainLogPath == "" {
			continue
		}
		if _, exists := groups[rec.MainLogPath]; exists {
			continue
		}
		groups[rec.MainLogPath] = groupInfo{uuid: rec.ID.String(), path: rec.MainLogPath}
	}

	if len(groups) == 0 {
		logger.Debugf("loadExistingGroups: no existing groups found")
		return nil
	}

	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	for mainPath, gi := range groups {
		// Skip if already present in memory (e.g., created by recent writes)
		if _, ok := gsm.pathToUUID[mainPath]; ok {
			continue
		}

		groupBase := filepath.Join(gsm.config.IndexPath, gi.uuid)
		// Ensure group directory exists; if not, skip silently
		if _, statErr := os.Stat(groupBase); os.IsNotExist(statErr) {
			// No on-disk index for this group yet; skip to avoid creating empty shards
			logger.Debugf("loadExistingGroups: group directory not found, skipping: %s (%s)", mainPath, gi.uuid)
			continue
		}

		group := &ShardGroup{
			UUID:        gi.uuid,
			MainLogPath: mainPath,
			Shards:      make(map[int]bleve.Index),
			ShardPaths:  make(map[int]string),
			ShardCount:  gsm.config.ShardCount,
		}

		// Open shards that exist; attempt to open up to configured ShardCount
		for i := 0; i < group.ShardCount; i++ {
			// mirror shard naming logic
			shardName := fmt.Sprintf("shard_%d", i)
			shardPath := filepath.Join(groupBase, shardName)
			if _, err := os.Stat(shardPath); err != nil {
				if os.IsNotExist(err) {
					// Shard directory missing - skip without creating to avoid empty indices
					continue
				}
				logger.Warnf("loadExistingGroups: stat shard dir failed: %s: %v", shardPath, err)
				continue
			}

			idx, openErr := bleve.Open(shardPath)
			if openErr != nil {
				logger.Warnf("loadExistingGroups: failed to open shard at %s: %v", shardPath, openErr)
				continue
			}
			group.Shards[i] = idx
			group.ShardPaths[i] = shardPath

			// Assign stable global ID for this shard in current process
			gID := gsm.nextGlobalID
			gsm.globalToLocal[gID] = groupShardRef{uuid: gi.uuid, localID: i}
			gsm.localToGlobal[gsm.makeLocalKey(gi.uuid, i)] = gID
			gsm.nextGlobalID++
		}

		// Only register group if at least one shard was opened
		if len(group.Shards) > 0 {
			gsm.groups[gi.uuid] = group
			gsm.pathToUUID[mainPath] = gi.uuid
			logger.Infof("Loaded shard group %s for mainLogPath %s with %d shard(s)", gi.uuid, mainPath, len(group.Shards))
		}
	}

	return nil
}

// getOrCreateUUID: Find the first record UUID for a MainLogPath in the DB; create a placeholder if not found.
func (gsm *GroupedShardManager) getOrCreateUUID(mainLogPath string) (string, error) {
	// First check cache
	gsm.mu.RLock()
	if uuid, ok := gsm.pathToUUID[mainLogPath]; ok {
		gsm.mu.RUnlock()
		return uuid, nil
	}
	gsm.mu.RUnlock()

	q := query.NginxLogIndex
	// Get the first record in ascending order of creation time
	record, err := q.Where(q.MainLogPath.Eq(mainLogPath)).Order(q.CreatedAt).First()
	if err == nil && record != nil {
		return record.ID.String(), nil
	}

	// If not found, create a placeholder record: Path and MainLogPath are set to mainLogPath
	placeholder := &model.NginxLogIndex{
		BaseModelUUID: model.BaseModelUUID{},
		Path:          mainLogPath,
		MainLogPath:   mainLogPath,
		Enabled:       true,
	}
	if err := q.Create(placeholder); err != nil {
		return "", fmt.Errorf("failed to create placeholder NginxLogIndex for %s: %w", mainLogPath, err)
	}
	return placeholder.ID.String(), nil
}

// makeLocalKey constructs a unique key.
func (gsm *GroupedShardManager) makeLocalKey(uuid string, shardID int) string {
	var b strings.Builder
	b.Grow(len(uuid) + 8)
	b.WriteString(uuid)
	b.WriteString("#")
	b.WriteString(fmt.Sprintf("%d", shardID))
	return b.String()
}

// Simple hash (reusing old logic)
func defaultHashFunc(key string, shardCount int) int {
	// Simplified FNV-1a implementation to avoid introducing additional dependencies
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return int(hash % uint32(shardCount))
}
