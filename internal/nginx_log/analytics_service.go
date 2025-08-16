package nginx_log

// This file has been split into multiple modules for better organization:
//
// - analytics_service_core.go: Core service structure, initialization, search, validation
// - analytics_service_entries.go: Log entry retrieval, index status, preflight checks
// - analytics_service_dashboard.go: Dashboard analytics generation and Bleve integration
// - analytics_service_calculations.go: Statistical calculations for hourly, daily, URL, browser, OS, device stats
// - analytics_service_types.go: Type definitions and constants for analytics service
//
// All original functionality is preserved across these modules.
