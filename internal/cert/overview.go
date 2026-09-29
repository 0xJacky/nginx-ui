package cert

import (
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

// State summarizes a certificate for the list and the editor header.
type State string

const (
	StateValid     State = "valid"
	StateExpiring  State = "expiring"
	StateExpired   State = "expired"
	StateFailed    State = "failed"
	StateIssuing   State = "issuing"
	StateNotIssued State = "not_issued"
	StateManual    State = "manual"
)

// RenewalMethod tells how a certificate gets its next version.
type RenewalMethod string

const (
	RenewalMethodDNS01      RenewalMethod = "dns01"
	RenewalMethodHTTP01     RenewalMethod = "http01"
	RenewalMethodUpload     RenewalMethod = "upload"
	RenewalMethodSelfSigned RenewalMethod = "self_signed"
	RenewalMethodSync       RenewalMethod = "sync"
)

// ExpiringWithinDays is the window of the "expiring soon" state and filter.
const ExpiringWithinDays = 30

// ListFilter selects certificates by their state in the list.
type ListFilter string

const (
	FilterAll      ListFilter = "all"
	FilterExpiring ListFilter = "expiring"
	FilterFailed   ListFilter = "failed"
	FilterExpired  ListFilter = "expired"
)

// Overview holds the values derived from a certificate record and its file.
type Overview struct {
	State State `json:"state"`
	// DaysLeft is the number of whole days until expiry, negative once expired.
	DaysLeft *int `json:"days_left,omitempty"`
	// RenewAt is when automatic renewal is expected to run.
	RenewAt       *time.Time    `json:"renew_at,omitempty"`
	RenewalMethod RenewalMethod `json:"renewal_method"`
	// LastRenewalAt and LastRenewalError describe the latest issuance or
	// automatic renewal attempt, whichever ran last.
	LastRenewalAt    *time.Time `json:"last_renewal_at,omitempty"`
	LastRenewalError string     `json:"last_renewal_error,omitempty"`
	RenewalFailed    bool       `json:"renewal_failed"`
}

// BuildOverview derives the overview of c. info is nil when the certificate
// file is missing or unreadable.
func BuildOverview(c *model.Cert, info *Info, now time.Time, renewalThresholdDays int) Overview {
	o := Overview{RenewalMethod: renewalMethod(c)}
	if c == nil {
		o.State = StateNotIssued
		return o
	}

	o.LastRenewalAt, o.LastRenewalError, o.RenewalFailed = lastRenewal(c)

	if info != nil && !info.NotAfter.IsZero() {
		days := daysUntil(now, info.NotAfter)
		o.DaysLeft = &days

		if c.AutoCert == model.AutoCertEnabled || c.AutoCert == model.AutoCertSelfSigned {
			renewAt := certificateRenewalTime(info, renewalThresholdDays)
			if c.NextAutoRenewAt != nil && c.NextAutoRenewAt.After(info.NotBefore) {
				renewAt = *c.NextAutoRenewAt
			}
			if !renewAt.IsZero() {
				o.RenewAt = &renewAt
			}
		}
	}

	o.State = overviewState(c, info, now, o.RenewalFailed)
	return o
}

func overviewState(c *model.Cert, info *Info, now time.Time, failed bool) State {
	switch {
	case c.Status == model.CertStatusPending:
		return StateIssuing
	case failed:
		return StateFailed
	case info == nil || info.NotAfter.IsZero():
		return StateNotIssued
	case now.After(info.NotAfter):
		return StateExpired
	case info.NotAfter.Sub(now) <= ExpiringWithinDays*24*time.Hour:
		return StateExpiring
	case c.AutoCert == model.AutoCertDisabled || c.AutoCert == 0:
		return StateManual
	default:
		return StateValid
	}
}

// lastRenewal picks the latest of the recorded issuance attempt and the
// latest automatic renewal attempt.
func lastRenewal(c *model.Cert) (*time.Time, string, bool) {
	at := c.LastAttemptAt
	errorMessage := c.LastError
	failed := c.Status == model.CertStatusFailure

	if c.LastAutoRenewAt != nil && (at == nil || c.LastAutoRenewAt.After(*at)) {
		at = c.LastAutoRenewAt
		errorMessage = c.LastAutoRenewError
		failed = c.LastAutoRenewError != ""
	}

	if !failed {
		errorMessage = ""
	}
	return at, errorMessage, failed
}

func renewalMethod(c *model.Cert) RenewalMethod {
	if c == nil {
		return RenewalMethodUpload
	}

	switch c.AutoCert {
	case model.AutoCertSelfSigned:
		return RenewalMethodSelfSigned
	case model.AutoCertSync:
		return RenewalMethodSync
	}

	switch strings.ToLower(c.ChallengeMethod) {
	case DNS01:
		return RenewalMethodDNS01
	case HTTP01:
		return RenewalMethodHTTP01
	}
	return RenewalMethodUpload
}

func daysUntil(now, t time.Time) int {
	d := t.Sub(now)
	days := int(d / (24 * time.Hour))
	if d < 0 && d%(24*time.Hour) != 0 {
		days--
	}
	return days
}

// MatchesFilter reports whether a certificate belongs to a list filter.
// Filters overlap on purpose: an expired certificate whose renewal failed
// is counted under both.
func (o Overview) MatchesFilter(filter ListFilter, info *Info, now time.Time) bool {
	switch filter {
	case FilterExpiring:
		return info != nil && !info.NotAfter.IsZero() && !now.After(info.NotAfter) &&
			info.NotAfter.Sub(now) <= ExpiringWithinDays*24*time.Hour
	case FilterFailed:
		return o.RenewalFailed && o.State != StateIssuing
	case FilterExpired:
		return info != nil && !info.NotAfter.IsZero() && now.After(info.NotAfter)
	default:
		return true
	}
}

// ParseListFilter maps a query value to a filter, falling back to all.
func ParseListFilter(value string) ListFilter {
	switch ListFilter(value) {
	case FilterExpiring, FilterFailed, FilterExpired:
		return ListFilter(value)
	default:
		return FilterAll
	}
}

// CanSwitchAutoRenewal reports whether automatic renewal of c may be turned
// on or off: only certificates issued through ACME by Nginx UI qualify.
func CanSwitchAutoRenewal(c *model.Cert) bool {
	if c == nil || c.ChallengeMethod == "" || len(c.Domains) == 0 {
		return false
	}
	switch c.AutoCert {
	case model.AutoCertEnabled, model.AutoCertDisabled, model.AutoCertPaused:
		return true
	default:
		return false
	}
}

// SetAutoRenewal turns automatic renewal of an ACME certificate on or off.
func SetAutoRenewal(c *model.Cert, enabled bool) error {
	if !CanSwitchAutoRenewal(c) {
		return ErrAutoRenewalNotSupported
	}

	db := model.UseDB()
	if db == nil {
		return ErrDatabaseNotInitialized
	}

	state := model.AutoCertDisabled
	if enabled {
		state = model.AutoCertEnabled
	}
	if err := db.Model(&model.Cert{}).Where("id = ?", c.ID).Update("auto_cert", state).Error; err != nil {
		return err
	}
	c.AutoCert = state
	return nil
}
