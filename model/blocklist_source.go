package model

import "time"

// Outcomes of the last refresh of a generated nginx file, a blocklist source
// or an upstream discovery.
const (
	RefreshStatusOK     = "ok"
	RefreshStatusFailed = "failed"
)

// BlocklistSource is a list of addresses to deny, fetched from a
// security.blocklist plugin on an interval and written to a file of its own
// in the nginx configuration directory.
type BlocklistSource struct {
	Model
	Name string `json:"name" cosy:"add:required;update:omitempty" gorm:"not null"`
	// Kind is "plugin:<code>", the source kind of a security.blocklist plugin.
	Kind string `json:"kind" cosy:"add:required;update:omitempty" gorm:"index;not null"`
	// Config holds the values of the source form. They are credentials, so
	// they are encrypted at rest.
	Config map[string]string `json:"config" cosy:"add:omitempty;update:omitempty" gorm:"serializer:json[aes]"`
	// RefreshSeconds is the refresh interval. 0 on create takes the default
	// of the kind.
	RefreshSeconds int `json:"refresh_seconds" cosy:"add:omitempty;update:omitempty"`
	// Enabled has no column default, so a source created disabled stays so.
	Enabled bool `json:"enabled" cosy:"add:omitempty;update:omitempty" gorm:"index"`

	// The fields below are written by the runner only.
	LastRunAt *time.Time `json:"last_run_at"`
	// NextRunAt is when the source is due again, nil means now.
	NextRunAt *time.Time `json:"next_run_at" gorm:"index"`
	// LastStatus is RefreshStatusOK, RefreshStatusFailed or empty before the
	// first run.
	LastStatus  string `json:"last_status"`
	LastMessage string `json:"last_message"`
	// EntryCount is the number of deny rules the file holds.
	EntryCount int `json:"entry_count"`
}
