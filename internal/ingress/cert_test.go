package ingress

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupCertTest points the nginx configuration directory at a temp dir and
// opens a database with the cert and ingress tables.
func setupCertTest(t *testing.T) string {
	t.Helper()

	confDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(confDir, "ssl"), 0o755))
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = originalConfigDir })

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	// One connection: background status writes and the test's polling reads
	// would otherwise hit SQLITE_LOCKED on the shared cache.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Cert{}, &model.Ingress{}))
	model.Use(db)
	return confDir
}

// writeTestCert writes a self-signed certificate for names and its key to
// certPath and keyPath.
func writeTestCert(t *testing.T, certPath, keyPath string, names []string, notBefore, notAfter time.Time) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: names[0]},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, name := range names {
		if ip := net.ParseIP(name); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, name)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Dir(certPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(keyPath), 0o755))
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
}

// addTestCert writes a certificate valid for a year and stores its record.
func addTestCert(t *testing.T, confDir, filename string, keyType certcrypto.KeyType, names ...string) *model.Cert {
	t.Helper()
	return addTestCertValid(t, confDir, filename, keyType, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour), names...)
}

func addTestCertValid(t *testing.T, confDir, filename string, keyType certcrypto.KeyType,
	notBefore, notAfter time.Time, names ...string) *model.Cert {
	t.Helper()
	dir := filepath.Join(confDir, "ssl", filename+"_"+string(keyType))
	certPath := filepath.Join(dir, "fullchain.cer")
	keyPath := filepath.Join(dir, "private.key")
	writeTestCert(t, certPath, keyPath, names, notBefore, notAfter)
	row := &model.Cert{
		Name:                  names[0],
		Filename:              filename,
		Domains:               names,
		KeyType:               keyType,
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		AutoCert:              model.AutoCertEnabled,
		Status:                model.CertStatusSuccess,
	}
	require.NoError(t, model.UseDB().Create(row).Error)
	return row
}

func certIngress(name, mode string, hosts ...string) *model.Ingress {
	return &model.Ingress{
		Name:  name,
		Owner: "test",
		Spec: model.IngressSpec{
			Hosts: hosts,
			TLS:   model.IngressTLS{Mode: mode},
		},
		Generation: 1,
	}
}

func resolve(t *testing.T, m *acmeCertManager, ing *model.Ingress) CertState {
	t.Helper()
	state, err := m.Resolve(context.Background(), ing)
	require.NoError(t, err)
	return state
}

func TestResolveNoneIsNotRequired(t *testing.T) {
	m := newACMECertManager()
	state := resolve(t, m, certIngress("plain", model.IngressTLSModeNone, "a.example.com"))
	assert.False(t, state.Ready)
	assert.Equal(t, CertReasonNotRequired, state.Reason)
}

func TestResolveUnknownModeFails(t *testing.T) {
	m := newACMECertManager()
	_, err := m.Resolve(context.Background(), certIngress("odd", "bogus", "a.example.com"))
	require.Error(t, err)
}

func TestResolveExistingCertificate(t *testing.T) {
	confDir := setupCertTest(t)
	m := newACMECertManager()

	multi := addTestCert(t, confDir, "multi", certcrypto.EC256, "a.example.com", "b.example.com")
	wildcard := addTestCert(t, confDir, "wildcard", certcrypto.EC256, "*.example.org", "example.org")
	idn := addTestCert(t, confDir, "idn", certcrypto.EC256, "xn--fsqu00a.example.net")
	expired := addTestCertValid(t, confDir, "expired", certcrypto.EC256,
		time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour), "a.example.com")
	missing := addTestCert(t, confDir, "missing", certcrypto.EC256, "a.example.com")
	require.NoError(t, os.Remove(missing.SSLCertificateKeyPath))

	withCert := func(id uint64, hosts ...string) *model.Ingress {
		ing := certIngress("existing", model.IngressTLSModeCert, hosts...)
		ing.Spec.TLS.CertID = id
		return ing
	}

	t.Run("covers every host", func(t *testing.T) {
		state := resolve(t, m, withCert(multi.ID, "a.example.com", "B.example.com"))
		assert.True(t, state.Ready)
		assert.Equal(t, CertReasonExisting, state.Reason)
		assert.Equal(t, multi.ID, state.CertID)
		assert.Equal(t, multi.SSLCertificatePath, state.CertPath)
		assert.Equal(t, multi.SSLCertificateKeyPath, state.KeyPath)
		require.NotNil(t, state.NotAfter)
		assert.True(t, state.NotAfter.After(time.Now()))
	})

	t.Run("wildcard", func(t *testing.T) {
		state := resolve(t, m, withCert(wildcard.ID, "www.example.org", "*.example.org", "example.org"))
		assert.True(t, state.Ready, state.Message)
		assert.Equal(t, CertReasonExisting, state.Reason)
	})

	t.Run("wildcard does not cover a deeper name", func(t *testing.T) {
		state := resolve(t, m, withCert(wildcard.ID, "a.b.example.org"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonCertNotFound, state.Reason)
		assert.Contains(t, state.Message, "a.b.example.org")
	})

	t.Run("internationalized host", func(t *testing.T) {
		state := resolve(t, m, withCert(idn.ID, "例子.example.net"))
		assert.True(t, state.Ready, state.Message)
	})

	t.Run("names not covered", func(t *testing.T) {
		state := resolve(t, m, withCert(multi.ID, "a.example.com", "c.example.com"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonCertNotFound, state.Reason)
		assert.Contains(t, state.Message, "c.example.com")
	})

	t.Run("expired", func(t *testing.T) {
		state := resolve(t, m, withCert(expired.ID, "a.example.com"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonExpired, state.Reason)
		assert.Equal(t, expired.ID, state.CertID)
		require.NotNil(t, state.NotAfter)
	})

	t.Run("missing file", func(t *testing.T) {
		state := resolve(t, m, withCert(missing.ID, "a.example.com"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonCertNotFound, state.Reason)
		assert.Empty(t, state.CertPath)
	})

	t.Run("unknown record", func(t *testing.T) {
		state := resolve(t, m, withCert(9999, "a.example.com"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonCertNotFound, state.Reason)
	})

	t.Run("no cert id", func(t *testing.T) {
		state := resolve(t, m, withCert(0, "a.example.com"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonCertNotFound, state.Reason)
	})
}

func TestResolveACMECertificate(t *testing.T) {
	confDir := setupCertTest(t)
	m := newACMECertManager()

	t.Run("nothing issued yet", func(t *testing.T) {
		state := resolve(t, m, certIngress("fresh", model.IngressTLSModeACME, "fresh.example.com"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonIssuing, state.Reason)
	})

	issued := addTestCert(t, confDir, SiteName("app"), certcrypto.RSA2048, "app.example.com", "www.example.com")

	t.Run("issued", func(t *testing.T) {
		state := resolve(t, m, certIngress("app", model.IngressTLSModeACME, "app.example.com", "www.example.com"))
		assert.True(t, state.Ready)
		assert.Equal(t, CertReasonIssued, state.Reason)
		assert.Equal(t, issued.ID, state.CertID)
		assert.Equal(t, issued.SSLCertificatePath, state.CertPath)
		require.NotNil(t, state.NotAfter)
	})

	t.Run("legacy key type alias", func(t *testing.T) {
		ing := certIngress("app", model.IngressTLSModeACME, "app.example.com")
		ing.Spec.TLS.KeyType = "2048"
		state := resolve(t, m, ing)
		assert.True(t, state.Ready)
		assert.Equal(t, issued.ID, state.CertID)
	})

	t.Run("other key type has no certificate", func(t *testing.T) {
		ing := certIngress("app", model.IngressTLSModeACME, "app.example.com")
		ing.Spec.TLS.KeyType = "P256"
		state := resolve(t, m, ing)
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonIssuing, state.Reason)
	})

	t.Run("added host keeps serving the current certificate", func(t *testing.T) {
		state := resolve(t, m, certIngress("app", model.IngressTLSModeACME, "app.example.com", "new.example.com"))
		assert.True(t, state.Ready)
		assert.Equal(t, CertReasonIssuing, state.Reason)
		assert.Contains(t, state.Message, "new.example.com")
		assert.Equal(t, issued.SSLCertificatePath, state.CertPath)
	})

	t.Run("last failure of the record", func(t *testing.T) {
		row := addTestCertValid(t, confDir, SiteName("broken"), certcrypto.RSA2048,
			time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour), "broken.example.com")
		require.NoError(t, model.UseDB().Model(row).Updates(map[string]any{
			"status": model.CertStatusFailure, "last_error": "acme: unauthorized",
		}).Error)
		state := resolve(t, m, certIngress("broken", model.IngressTLSModeACME, "broken.example.com"))
		assert.False(t, state.Ready)
		assert.Equal(t, CertReasonIssueFailed, state.Reason)
		assert.Contains(t, state.Message, "acme: unauthorized")
		assert.Contains(t, state.Message, "expired")
	})
}

// --- Ensure ---

type fakeTimer struct {
	delay   time.Duration
	fire    func()
	stopped bool
}

// certHarness drives an acmeCertManager with fake effects.
type certHarness struct {
	t       *testing.T
	confDir string
	m       *acmeCertManager

	mu       sync.Mutex
	now      time.Time
	timers   []*fakeTimer
	probeErr error
	issueErr error
	probes   int
	issues   []*cert.ConfigPayload
	enqueued []string
	// issueGate, when set, blocks every issuance until it is closed.
	issueGate chan struct{}
	running   int
	maxActive int
}

func newCertHarness(t *testing.T) *certHarness {
	h := &certHarness{t: t, confDir: setupCertTest(t), now: time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)}
	m := newACMECertManager()
	m.now = func() time.Time {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.now
	}
	m.afterFunc = func(d time.Duration, f func()) func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		timer := &fakeTimer{delay: d, fire: f}
		h.timers = append(h.timers, timer)
		return func() bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			stopped := !timer.stopped
			timer.stopped = true
			return stopped
		}
	}
	m.enqueue = func(names ...string) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.enqueued = append(h.enqueued, names...)
	}
	m.probe = func(context.Context, certRequest) (acmehint.Evidence, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.probes++
		return acmehint.Evidence{}, h.probeErr
	}
	m.issue = h.issue
	h.m = m
	return h
}

// issue writes a certificate for the payload and records it the way
// cert.IssueWithRecord does, but leaves auto renewal off and the paths empty
// so the manager has to fix them.
func (h *certHarness) issue(_ context.Context, siteName string, payload *cert.ConfigPayload) (*model.Cert, error) {
	h.mu.Lock()
	h.issues = append(h.issues, payload)
	h.running++
	h.maxActive = max(h.maxActive, h.running)
	gate, err := h.issueGate, h.issueErr
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.running--
		h.mu.Unlock()
	}()
	if gate != nil {
		<-gate
	}
	if err != nil {
		return &model.Cert{}, err
	}

	var row model.Cert
	db := model.UseDB()
	if err := db.Where("filename = ? AND key_type = ?", siteName, payload.KeyType).
		Attrs(model.Cert{Name: payload.ServerName[0], Filename: siteName, KeyType: payload.KeyType,
			Domains: payload.ServerName, AutoCert: model.AutoCertDisabled}).
		FirstOrCreate(&row).Error; err != nil {
		return nil, err
	}
	// Like cert.IssueWithRecord, a reissue writes over the files of the record.
	payload.UseExistingCertificatePaths(row.SSLCertificatePath, row.SSLCertificateKeyPath)
	writeTestCert(h.t, payload.GetCertificatePath(), payload.GetCertificateKeyPath(), payload.ServerName,
		time.Now().Add(-time.Hour), time.Now().Add(90*24*time.Hour))
	if err := db.Model(&row).Update("status", model.CertStatusSuccess).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (h *certHarness) set(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fn()
}

func (h *certHarness) addIngress(ing *model.Ingress) *model.Ingress {
	h.t.Helper()
	require.NoError(h.t, model.UseDB().Create(ing).Error)
	return ing
}

func (h *certHarness) state(name string) (acmeState, bool) {
	h.m.mu.Lock()
	defer h.m.mu.Unlock()
	s := h.m.states[name]
	if s == nil {
		return acmeState{}, false
	}
	return *s, true
}

// waitIdle waits until no run is in flight for name.
func (h *certHarness) waitIdle(name string) {
	h.t.Helper()
	require.Eventually(h.t, func() bool {
		s, ok := h.state(name)
		return !ok || !s.inFlight
	}, 5*time.Second, 5*time.Millisecond)
}

func (h *certHarness) waitEnqueued(name string) {
	h.t.Helper()
	require.Eventually(h.t, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, n := range h.enqueued {
			if n == name {
				return true
			}
		}
		return false
	}, 5*time.Second, 5*time.Millisecond)
}

// lastTimer returns the most recent pending timer.
func (h *certHarness) lastTimer() *fakeTimer {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	require.NotEmpty(h.t, h.timers)
	return h.timers[len(h.timers)-1]
}

// fireTimer advances the clock by the timer delay and fires it.
func (h *certHarness) fireTimer(timer *fakeTimer) {
	h.mu.Lock()
	h.now = h.now.Add(timer.delay)
	stopped := timer.stopped
	timer.stopped = true
	h.mu.Unlock()
	if !stopped {
		timer.fire()
	}
}

func (h *certHarness) counts() (probes, issues int, enqueued []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.probes, len(h.issues), append([]string(nil), h.enqueued...)
}

func certCondition(t *testing.T, name string) model.IngressCondition {
	t.Helper()
	ing, err := loadIngress(name)
	require.NoError(t, err)
	c, ok := FindCondition(&ing.Status, model.IngressConditionCertReady)
	require.True(t, ok, "CertReady condition not written")
	return c
}

// waitCertCondition waits until the CertReady condition has the reason; the
// manager writes it right after it records the state.
func waitCertCondition(t *testing.T, name, reason string) model.IngressCondition {
	t.Helper()
	var c model.IngressCondition
	require.Eventually(t, func() bool {
		ing, err := loadIngress(name)
		if err != nil {
			return false
		}
		var ok bool
		c, ok = FindCondition(&ing.Status, model.IngressConditionCertReady)
		return ok && c.Reason == reason
	}, 5*time.Second, 5*time.Millisecond)
	return c
}

func TestEnsureIssuesEnablesAutoRenewAndEnqueues(t *testing.T) {
	h := newCertHarness(t)
	ing := certIngress("app", model.IngressTLSModeACME, "app.example.com", "例子.example.com")
	ing.Spec.TLS.ACMEUserID = 7
	h.addIngress(ing)

	h.m.Ensure(context.Background(), ing)
	h.waitEnqueued("app")
	h.waitIdle("app")

	probes, issues, _ := h.counts()
	assert.Equal(t, 1, probes)
	require.Equal(t, 1, issues)
	payload := h.issues[0]
	assert.Equal(t, []string{"app.example.com", "xn--fsqu00a.example.com"}, payload.ServerName)
	assert.Equal(t, cert.HTTP01, payload.ChallengeMethod)
	assert.Equal(t, certcrypto.RSA2048, payload.KeyType)
	assert.Equal(t, uint64(7), payload.ACMEUserID)
	assert.Empty(t, payload.ConfigName)

	var row model.Cert
	require.NoError(t, model.UseDB().Where("filename = ?", SiteName("app")).First(&row).Error)
	assert.Equal(t, model.AutoCertEnabled, row.AutoCert)
	assert.Equal(t, payload.GetCertificatePath(), row.SSLCertificatePath)
	assert.Equal(t, payload.GetCertificateKeyPath(), row.SSLCertificateKeyPath)

	_, ok := h.state("app")
	assert.False(t, ok, "a successful run leaves no state behind")
	state := resolve(t, h.m, ing)
	assert.True(t, state.Ready)
	assert.Equal(t, CertReasonIssued, state.Reason)
	assert.Equal(t, row.ID, state.CertID)

	// The run reported its progress while issuing.
	c := certCondition(t, "app")
	assert.False(t, c.Status)
	assert.Equal(t, CertReasonIssuing, c.Reason)

	// Nothing to do once the certificate is in place.
	h.m.Ensure(context.Background(), ing)
	_, issues, _ = h.counts()
	assert.Equal(t, 1, issues)
}

func TestEnsureDNSNotReadyBacksOffUntilReady(t *testing.T) {
	h := newCertHarness(t)
	ing := h.addIngress(certIngress("late-dns", model.IngressTLSModeACME, "late.example.com"))
	h.set(func() {
		h.probeErr = &certProbeError{Reason: CertReasonDNSNotReady, Err: errors.New("late.example.com has no A or AAAA record")}
	})

	h.m.Ensure(context.Background(), ing)
	h.waitIdle("late-dns")

	_, issues, _ := h.counts()
	assert.Zero(t, issues, "the CA is not contacted while DNS is not ready")
	state := resolve(t, h.m, ing)
	assert.False(t, state.Ready)
	assert.Equal(t, CertReasonDNSNotReady, state.Reason)
	assert.Contains(t, state.Message, "no A or AAAA record")
	c := waitCertCondition(t, "late-dns", CertReasonDNSNotReady)
	assert.False(t, c.Status)
	assert.Contains(t, c.Message, "no A or AAAA record")

	// Within the backoff Ensure does nothing.
	h.m.Ensure(context.Background(), ing)
	probes, _, _ := h.counts()
	assert.Equal(t, 1, probes)

	// Each retry doubles the wait, capped at 30 minutes.
	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute,
		16 * time.Minute, 30 * time.Minute, 30 * time.Minute}
	for i, delay := range want {
		timer := h.lastTimer()
		require.Equal(t, delay, timer.delay, "retry %d", i+1)
		if i == len(want)-1 {
			break
		}
		h.fireTimer(timer)
		require.Eventually(t, func() bool {
			probes, _, _ := h.counts()
			return probes == i+2
		}, 5*time.Second, 5*time.Millisecond)
		h.waitIdle("late-dns")
	}

	// DNS fixed: the next retry issues.
	h.set(func() { h.probeErr = nil })
	h.fireTimer(h.lastTimer())
	h.waitEnqueued("late-dns")
	_, issues, _ = h.counts()
	assert.Equal(t, 1, issues)
	assert.True(t, resolve(t, h.m, ing).Ready)
}

func TestEnsureIssueFailureBacksOff(t *testing.T) {
	h := newCertHarness(t)
	ing := h.addIngress(certIngress("fail", model.IngressTLSModeACME, "fail.example.com"))
	h.set(func() {
		h.issueErr = errors.New("acme: error: 403 :: urn:ietf:params:acme:error:unauthorized :: invalid response")
	})

	h.m.Ensure(context.Background(), ing)
	h.waitIdle("fail")

	state := resolve(t, h.m, ing)
	assert.False(t, state.Ready)
	assert.Equal(t, CertReasonIssueFailed, state.Reason)
	assert.Contains(t, state.Message, "invalid response")
	waitCertCondition(t, "fail", CertReasonIssueFailed)
	assert.Equal(t, time.Minute, h.lastTimer().delay)
	_, _, enqueued := h.counts()
	assert.Empty(t, enqueued)

	h.m.Ensure(context.Background(), ing)
	_, issues, _ := h.counts()
	assert.Equal(t, 1, issues, "no retry before the backoff elapsed")

	h.fireTimer(h.lastTimer())
	require.Eventually(t, func() bool {
		_, issues, _ := h.counts()
		return issues == 2
	}, 5*time.Second, 5*time.Millisecond)
	h.waitIdle("fail")
	assert.Equal(t, 2*time.Minute, h.lastTimer().delay)
}

func TestEnsureSpecChangeResetsBackoff(t *testing.T) {
	h := newCertHarness(t)
	ing := h.addIngress(certIngress("change", model.IngressTLSModeACME, "old.example.com"))
	h.set(func() { h.issueErr = errors.New("boom") })

	h.m.Ensure(context.Background(), ing)
	h.waitIdle("change")
	firstTimer := h.lastTimer()

	h.set(func() { h.issueErr = nil })
	ing.Spec.Hosts = []string{"new.example.com"}
	h.m.Ensure(context.Background(), ing)
	h.waitEnqueued("change")
	_, issues, _ := h.counts()
	assert.Equal(t, 2, issues)
	assert.True(t, firstTimer.stopped, "the old retry is cancelled")
}

func TestEnsureRunsOneIssuanceAtATime(t *testing.T) {
	h := newCertHarness(t)
	gate := make(chan struct{})
	h.set(func() { h.issueGate = gate })
	first := h.addIngress(certIngress("first", model.IngressTLSModeACME, "first.example.com"))
	second := h.addIngress(certIngress("second", model.IngressTLSModeACME, "second.example.com"))

	for range 3 {
		h.m.Ensure(context.Background(), first)
	}
	h.m.Ensure(context.Background(), second)
	require.Eventually(t, func() bool {
		_, issues, _ := h.counts()
		return issues == 1
	}, 5*time.Second, 5*time.Millisecond)
	// The second ingress waits for the semaphore and reports Issuing.
	time.Sleep(50 * time.Millisecond)
	_, issues, _ := h.counts()
	assert.Equal(t, 1, issues)
	assert.Equal(t, CertReasonIssuing, resolve(t, h.m, second).Reason)
	assert.Equal(t, CertReasonIssuing, certCondition(t, "second").Reason)

	close(gate)
	h.waitEnqueued("first")
	h.waitEnqueued("second")
	probes, issues, _ := h.counts()
	assert.Equal(t, 2, probes)
	assert.Equal(t, 2, issues, "repeated Ensure calls are deduplicated")
	h.mu.Lock()
	assert.Equal(t, 1, h.maxActive)
	h.mu.Unlock()
}

func TestForgetDropsInFlightOutcome(t *testing.T) {
	h := newCertHarness(t)
	gate := make(chan struct{})
	h.set(func() { h.issueGate = gate })
	ing := h.addIngress(certIngress("gone", model.IngressTLSModeACME, "gone.example.com"))

	h.m.Ensure(context.Background(), ing)
	require.Eventually(t, func() bool {
		_, issues, _ := h.counts()
		return issues == 1
	}, 5*time.Second, 5*time.Millisecond)

	h.m.Forget("gone")
	close(gate)
	// Wait for the run to release the semaphore.
	require.Eventually(t, func() bool { return len(h.m.sem) == 0 }, 5*time.Second, 5*time.Millisecond)

	_, _, enqueued := h.counts()
	assert.Empty(t, enqueued)
	_, ok := h.state("gone")
	assert.False(t, ok)
}

func TestForgetStopsRetryAndQueuedRun(t *testing.T) {
	h := newCertHarness(t)
	ing := h.addIngress(certIngress("retry", model.IngressTLSModeACME, "retry.example.com"))
	h.set(func() { h.issueErr = errors.New("boom") })

	h.m.Ensure(context.Background(), ing)
	h.waitIdle("retry")
	timer := h.lastTimer()

	h.m.Forget("retry")
	assert.True(t, timer.stopped)
	_, ok := h.state("retry")
	assert.False(t, ok)

	// A run waiting for the semaphore is cancelled as well.
	h.m.sem <- struct{}{}
	h.set(func() { h.issueErr = nil })
	h.m.Ensure(context.Background(), ing)
	h.m.Forget("retry")
	<-h.m.sem
	time.Sleep(50 * time.Millisecond)
	_, issues, enqueued := h.counts()
	assert.Equal(t, 1, issues)
	assert.Empty(t, enqueued)
}

func TestEnsureSkipsCompleteCertificateAndOtherModes(t *testing.T) {
	h := newCertHarness(t)
	addTestCert(t, h.confDir, SiteName("done"), certcrypto.RSA2048, "done.example.com")
	h.m.Ensure(context.Background(), h.addIngress(certIngress("done", model.IngressTLSModeACME, "done.example.com")))
	h.m.Ensure(context.Background(), h.addIngress(certIngress("plain", model.IngressTLSModeNone, "plain.example.com")))
	h.m.Ensure(context.Background(), nil)

	probes, issues, _ := h.counts()
	assert.Zero(t, probes)
	assert.Zero(t, issues)
}

func TestEnsureReissuesForAddedHost(t *testing.T) {
	h := newCertHarness(t)
	addTestCert(t, h.confDir, SiteName("grow"), certcrypto.RSA2048, "a.example.com")
	ing := h.addIngress(certIngress("grow", model.IngressTLSModeACME, "a.example.com", "b.example.com"))

	h.m.Ensure(context.Background(), ing)
	h.waitEnqueued("grow")
	require.Len(t, h.issues, 1)
	assert.Equal(t, []string{"a.example.com", "b.example.com"}, h.issues[0].ServerName)
}

func TestEnsureDNS01SkipsProbe(t *testing.T) {
	h := newCertHarness(t)
	ing := certIngress("dns", model.IngressTLSModeACME, "*.example.com")
	ing.Spec.TLS.Challenge = model.IngressChallengeDNS01
	ing.Spec.TLS.DNSCredentialID = 3
	ing.Spec.TLS.KeyType = "P256"
	h.addIngress(ing)

	h.m.Ensure(context.Background(), ing)
	h.waitEnqueued("dns")
	probes, _, _ := h.counts()
	assert.Zero(t, probes)
	payload := h.issues[0]
	assert.Equal(t, cert.DNS01, payload.ChallengeMethod)
	assert.Equal(t, uint64(3), payload.DNSCredentialID)
	assert.Equal(t, certcrypto.EC256, payload.KeyType)
}

func TestCertBackoff(t *testing.T) {
	assert.Equal(t, time.Minute, certBackoff(1))
	assert.Equal(t, 2*time.Minute, certBackoff(2))
	assert.Equal(t, 16*time.Minute, certBackoff(5))
	assert.Equal(t, 30*time.Minute, certBackoff(6))
	assert.Equal(t, 30*time.Minute, certBackoff(100))
}

// --- probe ---

func stubCertProbe(t *testing.T, diagnostics []acmehint.Diagnostic, local []netip.Addr,
	routes []cert.HTTP01ProbeResult, routeErr error) *int {
	t.Helper()
	routeCalls := 0
	origDiagnose, origRoutes, origLocal, origCADir := diagnoseCertHosts, probeHTTP01Routes, localAddrs, certCADirOf
	diagnoseCertHosts = func(context.Context, []string, acmehint.Options) []acmehint.Diagnostic { return diagnostics }
	probeHTTP01Routes = func(context.Context, []string, ...cert.HTTP01ProbeOption) ([]cert.HTTP01ProbeResult, error) {
		routeCalls++
		return routes, routeErr
	}
	localAddrs = func() []netip.Addr { return local }
	certCADirOf = func(uint64) string { return "https://acme-v02.api.letsencrypt.org/directory" }
	t.Cleanup(func() {
		diagnoseCertHosts, probeHTTP01Routes, localAddrs, certCADirOf = origDiagnose, origRoutes, origLocal, origCADir
	})
	return &routeCalls
}

func TestProbeCertHosts(t *testing.T) {
	req := certRequest{Name: "app", Hosts: []string{"app.example.com"}}
	private := []netip.Addr{netip.MustParseAddr("10.0.0.5"), netip.MustParseAddr("127.0.0.1")}
	public := []netip.Addr{netip.MustParseAddr("203.0.113.10")}
	elsewhere := acmehint.Diagnostic{Code: acmehint.CodeDNSPointsElsewhere, Params: map[string]string{
		"domain": "app.example.com", "resolved": "198.51.100.7", "elsewhere": "198.51.100.7", "local": "10.0.0.5",
	}}
	ok := []cert.HTTP01ProbeResult{{Domain: "app.example.com", Status: cert.HTTP01ProbeStatusSuccess}}

	t.Run("no records", func(t *testing.T) {
		calls := stubCertProbe(t, []acmehint.Diagnostic{{Code: acmehint.CodeDNSNoRecords,
			Params: map[string]string{"domain": "app.example.com"}}}, private, ok, nil)
		_, err := probeCertHosts(context.Background(), req)
		var probeErr *certProbeError
		require.ErrorAs(t, err, &probeErr)
		assert.Equal(t, CertReasonDNSNotReady, probeErr.Reason)
		assert.Contains(t, err.Error(), "app.example.com has no A or AAAA record")
		assert.Zero(t, *calls)
	})

	t.Run("points elsewhere behind NAT", func(t *testing.T) {
		calls := stubCertProbe(t, []acmehint.Diagnostic{elsewhere}, private, ok, nil)
		evidence, err := probeCertHosts(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, 1, *calls)
		assert.Equal(t, []string{"198.51.100.7"}, evidence.Resolved["app.example.com"])
	})

	t.Run("points elsewhere on a public host", func(t *testing.T) {
		stubCertProbe(t, []acmehint.Diagnostic{elsewhere}, public, ok, nil)
		_, err := probeCertHosts(context.Background(), req)
		var probeErr *certProbeError
		require.ErrorAs(t, err, &probeErr)
		assert.Equal(t, CertReasonDNSNotReady, probeErr.Reason)
		assert.Contains(t, err.Error(), "198.51.100.7")
	})

	t.Run("challenge route not served", func(t *testing.T) {
		stubCertProbe(t, nil, private, []cert.HTTP01ProbeResult{{
			Domain: "app.example.com", Status: cert.HTTP01ProbeStatusFailure, StatusCode: 404, Error: "unexpected status 404",
		}}, nil)
		_, err := probeCertHosts(context.Background(), req)
		var probeErr *certProbeError
		require.ErrorAs(t, err, &probeErr)
		assert.Equal(t, CertReasonIssueFailed, probeErr.Reason)
		assert.Contains(t, err.Error(), "challenge route check failed")
	})

	t.Run("route probe cannot run", func(t *testing.T) {
		stubCertProbe(t, nil, private, nil, errors.New("port busy"))
		_, err := probeCertHosts(context.Background(), req)
		var probeErr *certProbeError
		require.ErrorAs(t, err, &probeErr)
		assert.Equal(t, CertReasonIssueFailed, probeErr.Reason)
	})

	t.Run("warnings pass", func(t *testing.T) {
		stubCertProbe(t, []acmehint.Diagnostic{{Code: acmehint.CodeDNSOK}}, private, []cert.HTTP01ProbeResult{{
			Domain: "app.example.com", Status: cert.HTTP01ProbeStatusWarning,
		}}, nil)
		_, err := probeCertHosts(context.Background(), req)
		require.NoError(t, err)
	})
}

func TestHasPublicAddr(t *testing.T) {
	parse := func(addrs ...string) []netip.Addr {
		var out []netip.Addr
		for _, a := range addrs {
			out = append(out, netip.MustParseAddr(a))
		}
		return out
	}
	assert.False(t, hasPublicAddr(parse("127.0.0.1", "10.1.2.3", "172.17.0.2", "192.168.1.4", "100.100.1.1", "fd00::1", "fe80::1", "::1")))
	assert.True(t, hasPublicAddr(parse("10.1.2.3", "203.0.113.4")))
	assert.True(t, hasPublicAddr(parse("2001:db8::1")))
}
