package model

import "time"

// DDNSRecordTarget represents a DNS record to be managed by DDNS.
type DDNSRecordTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// DDNSConfig stores per-domain DDNS configuration and runtime status.
type DDNSConfig struct {
	Enabled         bool               `json:"enabled"`
	IntervalSeconds int                `json:"interval_seconds"`
	Targets         []DDNSRecordTarget `json:"targets"`
	LastIPv4        string             `json:"last_ipv4,omitempty"`
	LastIPv6        string             `json:"last_ipv6,omitempty"`
	LastRunAt       *time.Time         `json:"last_run_at,omitempty"`
	LastError       string             `json:"last_error,omitempty"`
}
