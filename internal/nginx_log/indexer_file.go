package nginx_log

// This file has been split into multiple modules for better organization:
//
// - indexer_file_safety.go: File safety and validation functions
// - indexer_file_management.go: File path management, watching, and queuing
// - indexer_file_indexing.go: Main indexing operations and force reindex
// - indexer_file_full.go: Full reindexing operations for log groups
// - indexer_file_streaming.go: Streaming indexing operations and file processing
// - indexer_file_batch.go: Batch processing for log entries
// - indexer_file_utils.go: Utility functions for log file discovery and patterns
//
// All original functionality is preserved across these modules.
