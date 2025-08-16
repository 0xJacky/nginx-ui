package nginx_log

// This file has been split into multiple modules for better organization:
//
// - log_indexer_core.go: Core structure, initialization, index creation, closing
// - log_indexer_rebuild.go: Index rebuilding, file deletion, cleanup operations
// - log_indexer_status.go: Time range queries, index status, availability checks
// - log_indexer_tasks.go: Task debouncing, execution, and processing
//
// Note: The following functions are implemented in other modules:
// - File indexing operations: indexer_file_*.go modules
// - Search operations: indexer_search.go
// - Statistics operations: indexer_stats.go
// - Cache operations: log_cache.go
//
// All original functionality is preserved across these modules.
