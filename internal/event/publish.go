package event

import "time"

// This file collects the small typed helpers around Publish for the domain
// events the plugin protocol promises (see internal/plugin/protocol/events.go).
// Each helper builds the Event and calls Publish, which is synchronous but
// never blocks: every subscriber, including the plugin dispatcher, must
// return immediately (see Subscribe's contract in subscribe.go).

// PublishCertIssued reports that a certificate finished issuance
// successfully. A renewal fires TypeCertRenewed instead of TypeCertIssued, so
// a subscriber tells the two apart by the type alone. The renewed flag stays
// in the data for the subscribers that watch both.
func PublishCertIssued(certID uint64, name string, domains []string, renewed bool) {
	eventType := TypeCertIssued
	if renewed {
		eventType = TypeCertRenewed
	}
	Publish(Event{Type: eventType, Data: map[string]any{
		"cert_id": certID,
		"name":    name,
		"domains": domains,
		"renewed": renewed,
	}})
}

// PublishCertExpiring reports that a certificate crossed one of the expiry
// notification thresholds.
func PublishCertExpiring(certID uint64, name string, domains []string, notAfter time.Time) {
	Publish(Event{Type: TypeCertExpiring, Data: map[string]any{
		"cert_id":   certID,
		"name":      name,
		"domains":   domains,
		"not_after": notAfter,
	}})
}

// PublishSiteSaved reports that a site configuration file was written.
func PublishSiteSaved(name string) {
	Publish(Event{Type: TypeSiteSaved, Data: map[string]any{"name": name}})
}

// PublishSiteEnabled reports that a site was enabled.
func PublishSiteEnabled(name string) {
	Publish(Event{Type: TypeSiteEnabled, Data: map[string]any{"name": name}})
}

// PublishSiteDisabled reports that a site was disabled.
func PublishSiteDisabled(name string) {
	Publish(Event{Type: TypeSiteDisabled, Data: map[string]any{"name": name}})
}

// PublishNginxReloaded reports the outcome of an Nginx reload. A failed
// attempt fires TypeNginxReloadFailed instead, so plugins can subscribe to
// only the outcome they care about.
func PublishNginxReloaded(ok bool, output string) {
	eventType := TypeNginxReloaded
	if !ok {
		eventType = TypeNginxReloadFailed
	}
	Publish(Event{Type: eventType, Data: map[string]any{
		"ok":     ok,
		"output": output,
	}})
}

// PublishNodeStatusChanged reports a cluster node crossing the online or
// offline boundary. Callers must only invoke this on the transition, not on
// every status poll, otherwise a subscriber sees the same edge many times.
func PublishNodeStatusChanged(nodeID uint64, online bool) {
	Publish(Event{Type: TypeNodeStatusChanged, Data: map[string]any{
		"node_id": nodeID,
		"online":  online,
	}})
}

// PublishBackupCompleted reports that a backup run finished, successfully or
// not.
func PublishBackupCompleted(name string, ok bool) {
	Publish(Event{Type: TypeBackupCompleted, Data: map[string]any{
		"name": name,
		"ok":   ok,
	}})
}

// PublishAuthLoginFailed reports a failed login attempt, so a plugin can
// implement its own throttling or blocklist on top of the built-in one.
func PublishAuthLoginFailed(username, ip string) {
	Publish(Event{Type: TypeAuthLoginFailed, Data: map[string]any{
		"username": username,
		"ip":       ip,
	}})
}
