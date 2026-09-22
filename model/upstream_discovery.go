package model

import "time"

// UpstreamDiscovery binds an nginx upstream to a service an
// upstream.discovery plugin resolves. The servers are written on an interval
// to a file of its own in the nginx configuration directory.
type UpstreamDiscovery struct {
	Model
	// UpstreamName is the name of the upstream block and of its file.
	UpstreamName string `json:"upstream_name" cosy:"add:required;update:omitempty" gorm:"index;not null"`
	// Kind is "plugin:<code>", the provider of an upstream.discovery plugin.
	Kind string `json:"kind" cosy:"add:required;update:omitempty" gorm:"index;not null"`
	// Config holds the values of the provider form. They are credentials, so
	// they are encrypted at rest.
	Config map[string]string `json:"config" cosy:"add:omitempty;update:omitempty" gorm:"serializer:json[aes]"`
	// Service names what to resolve, in the terms of the provider.
	Service string `json:"service" cosy:"add:required;update:omitempty" gorm:"not null"`
	// RefreshSeconds is the refresh interval. 0 on create takes the default.
	RefreshSeconds int `json:"refresh_seconds" cosy:"add:omitempty;update:omitempty"`
	// ExtraDirectives is appended inside the upstream block, e.g. keepalive.
	ExtraDirectives string `json:"extra_directives" cosy:"add:omitempty;update:omitempty" gorm:"type:text"`
	// Enabled has no column default, so a binding created disabled stays so.
	Enabled bool `json:"enabled" cosy:"add:omitempty;update:omitempty" gorm:"index"`

	// The fields below are written by the runner only.
	LastRunAt *time.Time `json:"last_run_at"`
	// NextRunAt is when the binding is due again, nil means now.
	NextRunAt *time.Time `json:"next_run_at" gorm:"index"`
	// LastStatus is RefreshStatusOK, RefreshStatusFailed or empty before the
	// first run.
	LastStatus  string `json:"last_status"`
	LastMessage string `json:"last_message"`
	// TargetCount is the number of servers the file holds.
	TargetCount int `json:"target_count"`
}
