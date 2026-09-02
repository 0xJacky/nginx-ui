package model

import "time"

// SiteHealthAlertState stores transition state so repeated sweeps and process
// restarts do not generate duplicate outage notifications.
type SiteHealthAlertState struct {
	Model
	SiteKey             string     `gorm:"uniqueIndex" json:"site_key"`
	LastStatus          string     `json:"last_status"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	FailureNotified     bool       `json:"failure_notified"`
	LastNotifiedAt      *time.Time `json:"last_notified_at"`
}
