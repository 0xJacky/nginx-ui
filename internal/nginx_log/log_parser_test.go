package nginx_log

// This file has been split into multiple test modules for better organization:
//
// - log_parser_parse_test.go: Tests for log parsing functionality, timestamp parsing, format detection
// - log_parser_useragent_test.go: Tests for user agent parsing, browser detection, OS detection, device detection
// - log_parser_bench_test.go: Benchmark tests for performance measurement
//
// All original test functionality is preserved across these modules.
