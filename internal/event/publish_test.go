package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureOne subscribes for the duration of fn and returns the single event
// it published. It fails the test if fn published zero or more than one.
func captureOne(t *testing.T, fn func()) Event {
	t.Helper()
	var got []Event
	unsubscribe := Subscribe(func(e Event) { got = append(got, e) })
	defer unsubscribe()

	fn()

	require.Len(t, got, 1)
	return got[0]
}

func TestPublishCertIssued(t *testing.T) {
	e := captureOne(t, func() { PublishCertIssued(7, "example.com", []string{"example.com", "www.example.com"}, false) })

	assert.Equal(t, TypeCertIssued, e.Type)
	data, ok := e.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(7), data["cert_id"])
	assert.Equal(t, "example.com", data["name"])
	assert.Equal(t, []string{"example.com", "www.example.com"}, data["domains"])
	assert.Equal(t, false, data["renewed"])
}

func TestPublishCertIssuedReportsARenewalAsRenewed(t *testing.T) {
	e := captureOne(t, func() { PublishCertIssued(7, "example.com", []string{"example.com"}, true) })

	assert.Equal(t, TypeCertRenewed, e.Type)
	data, ok := e.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(7), data["cert_id"])
	assert.Equal(t, true, data["renewed"])
}

func TestPublishCertExpiring(t *testing.T) {
	notAfter := time.Now().Add(48 * time.Hour)
	e := captureOne(t, func() { PublishCertExpiring(3, "example.com", []string{"example.com"}, notAfter) })

	assert.Equal(t, TypeCertExpiring, e.Type)
	data, ok := e.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(3), data["cert_id"])
	assert.Equal(t, "example.com", data["name"])
	assert.Equal(t, []string{"example.com"}, data["domains"])
	assert.Equal(t, notAfter, data["not_after"])
}

func TestPublishSiteSaved(t *testing.T) {
	e := captureOne(t, func() { PublishSiteSaved("example.conf") })

	assert.Equal(t, TypeSiteSaved, e.Type)
	assert.Equal(t, map[string]any{"name": "example.conf"}, e.Data)
}

func TestPublishSiteEnabledAndDisabled(t *testing.T) {
	e := captureOne(t, func() { PublishSiteEnabled("example.conf") })
	assert.Equal(t, TypeSiteEnabled, e.Type)
	assert.Equal(t, map[string]any{"name": "example.conf"}, e.Data)

	e = captureOne(t, func() { PublishSiteDisabled("example.conf") })
	assert.Equal(t, TypeSiteDisabled, e.Type)
	assert.Equal(t, map[string]any{"name": "example.conf"}, e.Data)
}

func TestPublishNginxReloaded(t *testing.T) {
	e := captureOne(t, func() { PublishNginxReloaded(true, "reloaded") })
	assert.Equal(t, TypeNginxReloaded, e.Type)
	assert.Equal(t, map[string]any{"ok": true, "output": "reloaded"}, e.Data)

	e = captureOne(t, func() { PublishNginxReloaded(false, "nginx: [emerg] boom") })
	assert.Equal(t, TypeNginxReloadFailed, e.Type)
	assert.Equal(t, map[string]any{"ok": false, "output": "nginx: [emerg] boom"}, e.Data)
}

func TestPublishNodeStatusChanged(t *testing.T) {
	e := captureOne(t, func() { PublishNodeStatusChanged(42, true) })
	assert.Equal(t, TypeNodeStatusChanged, e.Type)
	assert.Equal(t, map[string]any{"node_id": uint64(42), "online": true}, e.Data)

	e = captureOne(t, func() { PublishNodeStatusChanged(42, false) })
	assert.Equal(t, TypeNodeStatusChanged, e.Type)
	assert.Equal(t, map[string]any{"node_id": uint64(42), "online": false}, e.Data)
}

func TestPublishBackupCompleted(t *testing.T) {
	e := captureOne(t, func() { PublishBackupCompleted("nightly", true) })
	assert.Equal(t, TypeBackupCompleted, e.Type)
	assert.Equal(t, map[string]any{"name": "nightly", "ok": true}, e.Data)
}

func TestPublishAuthLoginFailed(t *testing.T) {
	e := captureOne(t, func() { PublishAuthLoginFailed("alice", "203.0.113.5") })
	assert.Equal(t, TypeAuthLoginFailed, e.Type)
	assert.Equal(t, map[string]any{"username": "alice", "ip": "203.0.113.5"}, e.Data)
}
