package deploy

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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testKind = "plugin:test-cdn"

// fakeSource serves testKind and fails the first failures pushes of every
// zone, so concurrent targets do not share the failures.
type fakeSource struct {
	mu       sync.Mutex
	failures int
	failed   map[string]int
	pushes   []fakePush
}

type fakePush struct {
	config map[string]string
	cert   Certificate
	dryRun bool
}

func (s *fakeSource) Kinds() []TargetKind {
	return []TargetKind{{Kind: testKind, Name: "Test CDN", Fields: []KindField{
		{Key: "zone_id", DisplayName: "Zone ID", Required: true},
	}}}
}

func (s *fakeSource) Validate(_ context.Context, _ string, config map[string]string) error {
	if config["zone_id"] == "bad" {
		return cosy.WrapErrorWithParams(plugin.ErrDeployConfigInvalid, "zone_id", "unknown zone")
	}
	return nil
}

func (s *fakeSource) Push(_ context.Context, _ string, config map[string]string, cert Certificate, dryRun bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushes = append(s.pushes, fakePush{config: config, cert: cert, dryRun: dryRun})
	zone := config["zone_id"]
	if s.failed[zone] < s.failures {
		s.failed[zone]++
		return "", errors.New("cdn is down")
	}
	return "pushed " + cert.Name, nil
}

func (s *fakeSource) reset(failures int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = failures
	s.failed = map[string]int{}
	s.pushes = nil
}

func (s *fakeSource) recorded() []fakePush {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]fakePush(nil), s.pushes...)
}

var registerFakeSource = sync.OnceValue(func() *fakeSource {
	source := &fakeSource{}
	RegisterSource(source)
	return source
})

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// Runs query from their own goroutines; one connection keeps them on the
	// same in-memory database.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Cert{}, &model.CertDeployTarget{}, &model.CertDeployment{}, &model.Notification{}))
	originalDB := model.UseDB()
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() {
		model.Use(originalDB)
		if originalDB != nil {
			query.SetDefault(originalDB)
		}
	})
	return db
}

// writeCertificate writes a leaf signed by an issuer as a full chain file and
// the leaf key, the way nginx serves them.
func writeCertificate(t *testing.T, dir, name string) (certPath, keyPath string) {
	t.Helper()
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	issuerTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Issuer"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(48 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	issuerDER, err := x509.CreateCertificate(rand.Reader, issuerTemplate, issuerTemplate, &issuerKey.PublicKey, issuerKey)
	require.NoError(t, err)
	issuer, err := x509.ParseCertificate(issuerDER)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name, "www." + name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, issuer, &leafKey.PublicKey, issuerKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	require.NoError(t, err)

	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: issuerDER})...)
	certPath = filepath.Join(dir, name+".cer")
	keyPath = filepath.Join(dir, name+".key")
	require.NoError(t, os.WriteFile(certPath, chain, 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certPath, keyPath
}

func createCert(t *testing.T, db *gorm.DB, name string) *model.Cert {
	t.Helper()
	certPath, keyPath := writeCertificate(t, t.TempDir(), name)
	certModel := &model.Cert{Name: name, SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath}
	require.NoError(t, db.Create(certModel).Error)
	return certModel
}

func createTarget(t *testing.T, db *gorm.DB, name string, certID uint64, enabled bool) *model.CertDeployTarget {
	t.Helper()
	target := &model.CertDeployTarget{Name: name, Kind: testKind, Config: map[string]string{"zone_id": name}, CertID: certID, Enabled: enabled}
	require.NoError(t, db.Create(target).Error)
	return target
}

func deploymentsOf(t *testing.T, db *gorm.DB, targetID, certID uint64) []model.CertDeployment {
	t.Helper()
	var list []model.CertDeployment
	require.NoError(t, db.Where("target_id = ? AND cert_id = ?", targetID, certID).Order("id").Find(&list).Error)
	return list
}

func TestLoadCertificateSplitsTheChain(t *testing.T) {
	certPath, keyPath := writeCertificate(t, t.TempDir(), "example.com")

	certificate, err := LoadCertificate(&model.Cert{Filename: "example.com", SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath})
	require.NoError(t, err)
	assert.Equal(t, "example.com", certificate.Name)
	assert.Equal(t, []string{"example.com", "www.example.com"}, certificate.Domains)
	assert.Equal(t, 1, strings.Count(certificate.CertificatePEM, "BEGIN CERTIFICATE"))
	assert.Equal(t, 1, strings.Count(certificate.ChainPEM, "BEGIN CERTIFICATE"))
	assert.Contains(t, certificate.PrivateKeyPEM, "BEGIN EC PRIVATE KEY")
	assert.True(t, certificate.NotAfter.Equal(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)))

	// The leaf alone has no chain.
	leafOnly := filepath.Join(t.TempDir(), "leaf.cer")
	require.NoError(t, os.WriteFile(leafOnly, []byte(certificate.CertificatePEM), 0o600))
	certificate, err = LoadCertificate(&model.Cert{Name: "leaf", SSLCertificatePath: leafOnly, SSLCertificateKeyPath: keyPath})
	require.NoError(t, err)
	assert.Empty(t, certificate.ChainPEM)

	_, err = LoadCertificate(&model.Cert{Name: "no files"})
	assert.Error(t, err)
	_, err = LoadCertificate(&model.Cert{Name: "swapped", SSLCertificatePath: keyPath, SSLCertificateKeyPath: certPath})
	assert.Error(t, err, "a key file is no certificate")
	_, err = LoadCertificate(&model.Cert{Name: "bad key", SSLCertificatePath: certPath, SSLCertificateKeyPath: certPath})
	assert.Error(t, err, "a certificate is no key")
}

func TestRunnerRetriesAndRecordsOneOutcome(t *testing.T) {
	db := setupDB(t)
	source := registerFakeSource()
	certModel := createCert(t, db, "example.com")
	other := createCert(t, db, "other.com")
	bound := createTarget(t, db, "bound", certModel.ID, true)
	everyCert := createTarget(t, db, "every", 0, true)
	createTarget(t, db, "disabled", certModel.ID, false)
	createTarget(t, db, "elsewhere", other.ID, true)

	// Every target fails twice, then succeeds.
	source.reset(2)
	runner := NewRunner(t.Context(), WithRetrySchedule([]time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}))
	runner.HandleEvent(event.Event{Type: event.TypeCertIssued, Data: map[string]any{"cert_id": certModel.ID, "renewed": true}})
	runner.Wait()

	pushes := source.recorded()
	require.Len(t, pushes, 6)
	for _, push := range pushes {
		assert.False(t, push.dryRun)
		assert.Equal(t, "example.com", push.cert.Name)
		assert.Contains(t, push.cert.PrivateKeyPEM, "PRIVATE KEY")
	}

	for _, target := range []*model.CertDeployTarget{bound, everyCert} {
		deployments := deploymentsOf(t, db, target.ID, certModel.ID)
		require.Len(t, deployments, 1, target.Name)
		assert.Equal(t, model.CertDeploymentOK, deployments[0].Status)
		assert.Equal(t, "pushed example.com", deployments[0].Message)
		assert.Equal(t, 3, deployments[0].Attempts)
	}
	var total int64
	require.NoError(t, db.Model(&model.CertDeployment{}).Count(&total).Error)
	assert.Equal(t, int64(2), total, "only the bound and enabled targets receive the certificate")
}

func TestRunnerGivesUpAfterTheSchedule(t *testing.T) {
	db := setupDB(t)
	source := registerFakeSource()
	certModel := createCert(t, db, "example.com")
	target := createTarget(t, db, "cdn", certModel.ID, true)

	source.reset(100)
	runner := NewRunner(t.Context(), WithRetrySchedule([]time.Duration{time.Millisecond, time.Millisecond}))
	runner.HandleEvent(event.Event{Type: event.TypeCertRenewed, Data: map[string]any{"cert_id": float64(certModel.ID)}})
	runner.Wait()

	assert.Len(t, source.recorded(), 3, "one attempt and two retries")
	deployments := deploymentsOf(t, db, target.ID, certModel.ID)
	require.Len(t, deployments, 1)
	assert.Equal(t, model.CertDeploymentFailed, deployments[0].Status)
	assert.Equal(t, 3, deployments[0].Attempts)
	assert.Contains(t, deployments[0].Message, "cdn is down")

	// Other events are ignored.
	source.reset(0)
	runner.HandleEvent(event.Event{Type: event.TypeCertExpiring, Data: map[string]any{"cert_id": certModel.ID}})
	runner.HandleEvent(event.Event{Type: event.TypeCertIssued, Data: "not a map"})
	runner.Wait()
	assert.Empty(t, source.recorded())
}

func TestRunnerStopsWithItsContext(t *testing.T) {
	db := setupDB(t)
	source := registerFakeSource()
	certModel := createCert(t, db, "example.com")
	target := createTarget(t, db, "cdn", certModel.ID, true)

	source.reset(100)
	ctx, cancel := context.WithCancel(t.Context())
	runner := NewRunner(ctx, WithRetrySchedule([]time.Duration{time.Hour}))
	runner.TriggerCertificate(certModel.ID)
	require.Eventually(t, func() bool { return len(source.recorded()) == 1 }, 5*time.Second, 5*time.Millisecond)
	cancel()
	runner.Wait()

	assert.Empty(t, deploymentsOf(t, db, target.ID, certModel.ID), "an interrupted run records nothing")
}

func TestManualDeployments(t *testing.T) {
	db := setupDB(t)
	source := registerFakeSource()
	first := createCert(t, db, "a.example")
	second := createCert(t, db, "b.example")
	require.NoError(t, db.Create(&model.Cert{Name: "no files"}).Error)
	everyCert := createTarget(t, db, "every", 0, false)
	bound := createTarget(t, db, "bound", second.ID, true)

	// A target bound to every certificate pushes each one served from files,
	// even while it is disabled.
	source.reset(1)
	results, err := DeployTarget(t.Context(), everyCert.ID)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, first.ID, results[0].CertID)
	assert.Equal(t, model.CertDeploymentFailed, results[0].Status)
	assert.Equal(t, model.CertDeploymentOK, results[1].Status)
	assert.Len(t, source.recorded(), 2, "a manual push is not retried")

	// A certificate goes to its enabled targets only.
	source.reset(0)
	results, err = DeployCertificate(t.Context(), second.ID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, bound.ID, results[0].TargetID)
	assert.Equal(t, "pushed b.example", results[0].Message)

	last := LatestDeployment(bound.ID, second.ID)
	require.NotNil(t, last)
	assert.Equal(t, model.CertDeploymentOK, last.Status)
	assert.Equal(t, 1, last.Attempts)
	assert.Nil(t, LatestDeployment(bound.ID, first.ID))

	targets, err := TargetsOf(second.ID)
	require.NoError(t, err)
	assert.Len(t, targets, 2)

	_, err = DeployCertificate(t.Context(), 999)
	assert.Error(t, err)
}

func TestDryRunChangesNothing(t *testing.T) {
	db := setupDB(t)
	source := registerFakeSource()

	source.reset(0)
	_, err := DryRun(t.Context(), testKind, map[string]string{"zone_id": "z"}, 0)
	assert.Error(t, err, "without a certificate there is nothing to test with")

	certModel := createCert(t, db, "example.com")
	message, err := DryRun(t.Context(), testKind, map[string]string{"zone_id": "z"}, 0)
	require.NoError(t, err)
	assert.Equal(t, "pushed example.com", message)
	pushes := source.recorded()
	require.Len(t, pushes, 1)
	assert.True(t, pushes[0].dryRun)

	var total int64
	require.NoError(t, db.Model(&model.CertDeployment{}).Count(&total).Error)
	assert.Zero(t, total, "a dry run is not recorded")

	_, err = DryRun(t.Context(), testKind, nil, certModel.ID)
	require.NoError(t, err)
}

func TestValidateConfigAndKinds(t *testing.T) {
	registerFakeSource()

	kinds := Kinds()
	require.NotEmpty(t, kinds)
	assert.Equal(t, testKind, kinds[0].Kind)

	assert.NoError(t, ValidateConfig(t.Context(), testKind, map[string]string{"zone_id": "z"}))
	for _, tc := range []struct {
		kind   string
		config map[string]string
	}{
		{testKind, map[string]string{}},
		{testKind, map[string]string{"zone_id": "bad"}},
		{"plugin:missing", map[string]string{"zone_id": "z"}},
		{"Not A Kind", nil},
	} {
		assert.Error(t, ValidateConfig(t.Context(), tc.kind, tc.config), "%s %v", tc.kind, tc.config)
	}
}

func TestRecordKeepsTheLatestOutcomes(t *testing.T) {
	db := setupDB(t)
	for i := range keptDeployments + 5 {
		record(1, 2, model.CertDeploymentOK, "run", i+1)
	}
	record(1, 3, model.CertDeploymentOK, "other certificate", 1)

	deployments := deploymentsOf(t, db, 1, 2)
	require.Len(t, deployments, keptDeployments)
	assert.Equal(t, 6, deployments[0].Attempts, "the oldest outcomes are pruned")
	assert.Len(t, deploymentsOf(t, db, 1, 3), 1)

	list, err := Deployments(1, 0, 5)
	require.NoError(t, err)
	require.Len(t, list, 5)
	assert.Equal(t, "other certificate", list[0].Message)
}

func TestCertIDOf(t *testing.T) {
	for _, data := range []any{
		map[string]any{"cert_id": uint64(7)},
		map[string]any{"cert_id": 7},
		map[string]any{"cert_id": int64(7)},
		map[string]any{"cert_id": float64(7)},
	} {
		id, ok := certIDOf(data)
		assert.True(t, ok)
		assert.Equal(t, uint64(7), id)
	}
	for _, data := range []any{nil, "x", map[string]any{}, map[string]any{"cert_id": 0}, map[string]any{"cert_id": "7"}} {
		_, ok := certIDOf(data)
		assert.False(t, ok, "%v", data)
	}
}
