package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/certcrypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type certificateMigrationTestEnv struct {
	confDir string
	db      *gorm.DB
}

func setupCertificateMigrationTest(t *testing.T) certificateMigrationTestEnv {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"sites-available", "sites-enabled", "ssl", "conf.d"} {
		if err := os.MkdirAll(filepath.Join(confDir, dir), 0o755); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	originalSettings := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.TestConfigCmd = "true"
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	if err := os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("write nginx pid: %v", err)
	}
	t.Cleanup(func() { *settings.NginxSettings = originalSettings })

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err = db.AutoMigrate(
		&model.Cert{}, &model.Site{}, &model.Namespace{}, &model.ConfigBackup{},
		&model.Notification{}, &model.ExternalNotify{},
	); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	return certificateMigrationTestEnv{confDir: confDir, db: db}
}

func (env certificateMigrationTestEnv) addLegacySite(t *testing.T, name string,
	keyType certcrypto.KeyType) (*model.Cert, string, string, string) {
	t.Helper()

	canonical := string(keyType)
	legacy := string(keyType)
	switch keyType {
	case certcrypto.EC256:
		legacy = "P256"
	case certcrypto.EC384:
		legacy = "P384"
	case certcrypto.RSA2048:
		legacy = "2048"
	case certcrypto.RSA3072:
		legacy = "3072"
	case certcrypto.RSA4096:
		legacy = "4096"
	case certcrypto.RSA8192:
		legacy = "8192"
	}

	managedDir := filepath.Join(env.confDir, "ssl", name+"_"+canonical)
	if err := os.MkdirAll(managedDir, 0o755); err != nil {
		t.Fatalf("create managed cert dir: %v", err)
	}
	managedCertPath := filepath.Join(managedDir, "fullchain.cer")
	managedKeyPath := filepath.Join(managedDir, "private.key")
	certPEM, keyPEM, err := cert.GenerateSelfSigned(cert.SelfSignedOptions{
		CommonName: name,
		DNSNames:   []string{name},
		KeyType:    keyType,
	})
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	if err = os.WriteFile(managedCertPath, certPEM, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err = os.WriteFile(managedKeyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	legacyDir := filepath.Join(env.confDir, "ssl", name+"_"+legacy)
	legacyCertPath := filepath.Join(legacyDir, "fullchain.cer")
	legacyKeyPath := filepath.Join(legacyDir, "private.key")
	configPath := filepath.Join(env.confDir, "sites-available", name)
	content := fmt.Sprintf(`# keep formatting
server {
    listen 443 ssl;
    server_name %s;
    ssl_certificate "%s"; # legacy certificate
	ssl_certificate_key '%s';
}
`, name, legacyCertPath, legacyKeyPath)
	if err = os.WriteFile(configPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write site config: %v", err)
	}
	if err = os.Symlink(configPath, filepath.Join(env.confDir, "sites-enabled", name)); err != nil {
		t.Fatalf("enable site: %v", err)
	}

	certModel := &model.Cert{
		Name:                  name,
		Filename:              name,
		Domains:               []string{name},
		AutoCert:              model.AutoCertEnabled,
		KeyType:               keyType,
		SSLCertificatePath:    managedCertPath,
		SSLCertificateKeyPath: managedKeyPath,
		Status:                model.CertStatusSuccess,
	}
	if err = env.db.Create(certModel).Error; err != nil {
		t.Fatalf("create cert model: %v", err)
	}
	return certModel, configPath, legacyCertPath, legacyKeyPath
}

func TestInspectCertificateDeploymentFindsLegacyAliasDrift(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, _, legacyCertPath, legacyKeyPath := env.addLegacySite(t, "example.com", certcrypto.EC256)

	status := InspectCertificateDeployment(certModel)
	if status.State != CertificateDeploymentLegacyDrift || !status.AutomaticMigrationAvailable {
		t.Fatalf("deployment status = %+v", status)
	}
	if len(status.ConfiguredCertificatePaths) != 1 || status.ConfiguredCertificatePaths[0] != legacyCertPath {
		t.Fatalf("configured certificate paths = %#v", status.ConfiguredCertificatePaths)
	}
	if len(status.ConfiguredCertificateKeys) != 1 || status.ConfiguredCertificateKeys[0] != legacyKeyPath {
		t.Fatalf("configured key paths = %#v", status.ConfiguredCertificateKeys)
	}
}

func TestInspectCertificateDeploymentDoesNotAutoMigrateCustomMismatch(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, _, _ := env.addLegacySite(t, "custom.example.com", certcrypto.EC256)
	customCert := filepath.Join(env.confDir, "ssl", "custom", "cert.pem")
	customKey := filepath.Join(env.confDir, "ssl", "custom", "key.pem")
	content := fmt.Sprintf("server { listen 443 ssl; server_name custom.example.com; ssl_certificate %s; ssl_certificate_key %s; }\n",
		customCert, customKey)
	if err := os.WriteFile(configPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write custom mismatch: %v", err)
	}

	status := InspectCertificateDeployment(certModel)
	if status.State != CertificateDeploymentMismatch || status.AutomaticMigrationAvailable {
		t.Fatalf("deployment status = %+v", status)
	}
}

func TestInspectCertificateDeploymentRefusesUncoveredServerName(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, legacyCertPath, legacyKeyPath := env.addLegacySite(t, "covered.example.com", certcrypto.EC256)
	content := fmt.Sprintf("server { listen 443 ssl; server_name other.example.com; ssl_certificate %s; ssl_certificate_key %s; }\n",
		legacyCertPath, legacyKeyPath)
	if err := os.WriteFile(configPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write uncovered server config: %v", err)
	}

	status := InspectCertificateDeployment(certModel)
	if status.State != CertificateDeploymentMismatch || status.AutomaticMigrationAvailable ||
		!strings.Contains(status.Error, "does not cover server name") {
		t.Fatalf("deployment status = %+v", status)
	}
}

func TestMigrateLegacyCertificatePathsRewritesAndIsIdempotent(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, _, _ := env.addLegacySite(t, "migrate.example.com", certcrypto.EC256)

	result, err := MigrateLegacyCertificatePaths()
	if err != nil {
		t.Fatalf("MigrateLegacyCertificatePaths() error = %v", err)
	}
	if result.MigratedFiles != 1 || result.MigratedSites != 1 || result.SkippedFiles != 0 {
		t.Fatalf("migration result = %+v", result)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if !strings.Contains(string(content), `ssl_certificate "`+certModel.SSLCertificatePath+`"; # legacy certificate`) ||
		!strings.Contains(string(content), `ssl_certificate_key '`+certModel.SSLCertificateKeyPath+`';`) {
		t.Fatalf("migrated config =\n%s", content)
	}
	info, err := os.Stat(configPath)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("migrated config mode = %v, error = %v", info.Mode().Perm(), err)
	}

	var historyCount int64
	if err = env.db.Model(&model.ConfigBackup{}).Count(&historyCount).Error; err != nil || historyCount != 1 {
		t.Fatalf("history count = %d, error = %v", historyCount, err)
	}
	status := InspectCertificateDeployment(certModel)
	if status.State != CertificateDeploymentConsistent {
		t.Fatalf("post-migration status = %+v", status)
	}

	second, err := MigrateLegacyCertificatePaths()
	if err != nil {
		t.Fatalf("second migration error = %v", err)
	}
	if second.MigratedFiles != 0 || second.MigratedSites != 0 {
		t.Fatalf("second migration result = %+v", second)
	}
}

func TestMigrateLegacyCertificatePathsRepairsPartiallyMigratedConfig(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, legacyCertPath, legacyKeyPath := env.addLegacySite(t, "partial.example.com", certcrypto.EC256)
	content := fmt.Sprintf(`server {
    listen 443 ssl;
    server_name partial.example.com;
    ssl_certificate %s;
    ssl_certificate_key %s;
}
server {
    listen 8443 ssl;
    server_name partial.example.com;
    ssl_certificate %s;
    ssl_certificate_key %s;
}
`, legacyCertPath, legacyKeyPath, certModel.SSLCertificatePath, certModel.SSLCertificateKeyPath)
	if err := os.WriteFile(configPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write partially migrated config: %v", err)
	}

	result, err := MigrateLegacyCertificatePaths()
	if err != nil {
		t.Fatalf("MigrateLegacyCertificatePaths() error = %v", err)
	}
	if result.MigratedFiles != 1 || result.MigratedSites != 1 {
		t.Fatalf("migration result = %+v", result)
	}
	migrated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if strings.Contains(string(migrated), legacyCertPath) || strings.Contains(string(migrated), legacyKeyPath) {
		t.Fatalf("legacy paths remain after migration:\n%s", migrated)
	}
	if strings.Count(string(migrated), certModel.SSLCertificatePath) != 2 ||
		strings.Count(string(migrated), certModel.SSLCertificateKeyPath) != 2 {
		t.Fatalf("managed paths were not applied to both servers:\n%s", migrated)
	}
}

func TestMigrateLegacyCertificatePathsRollsBackWholeBatch(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	_, firstPath, _, _ := env.addLegacySite(t, "first.example.com", certcrypto.EC256)
	_, secondPath, _, _ := env.addLegacySite(t, "second.example.com", certcrypto.EC384)
	firstBefore, _ := os.ReadFile(firstPath)
	secondBefore, _ := os.ReadFile(secondPath)
	settings.NginxSettings.TestConfigCmd = "false"

	result, err := MigrateLegacyCertificatePaths()
	if err == nil {
		t.Fatal("MigrateLegacyCertificatePaths() expected error")
	}
	if result.MigratedFiles != 0 || result.MigratedSites != 0 {
		t.Fatalf("failed migration result = %+v", result)
	}
	firstAfter, _ := os.ReadFile(firstPath)
	secondAfter, _ := os.ReadFile(secondPath)
	if string(firstAfter) != string(firstBefore) || string(secondAfter) != string(secondBefore) {
		t.Fatalf("batch rollback did not restore every config")
	}
}

func TestMigrateLegacyCertificatePathsUpdatesMaintenanceConfig(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, _, _ := env.addLegacySite(t, "maintenance.example.com", certcrypto.EC256)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read source config: %v", err)
	}
	if err = os.Remove(filepath.Join(env.confDir, "sites-enabled", certModel.Filename)); err != nil {
		t.Fatalf("disable original site: %v", err)
	}
	maintenancePath := filepath.Join(env.confDir, "sites-enabled", certModel.Filename+MaintenanceSuffix)
	if err = os.WriteFile(maintenancePath, content, 0o644); err != nil {
		t.Fatalf("write maintenance config: %v", err)
	}

	result, err := MigrateLegacyCertificatePaths()
	if err != nil {
		t.Fatalf("MigrateLegacyCertificatePaths() error = %v", err)
	}
	if result.MigratedFiles != 2 || result.MigratedSites != 1 {
		t.Fatalf("migration result = %+v", result)
	}
	for _, path := range []string{configPath, maintenancePath} {
		migrated, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if !strings.Contains(string(migrated), certModel.SSLCertificatePath) ||
			!strings.Contains(string(migrated), certModel.SSLCertificateKeyPath) {
			t.Fatalf("config %s was not migrated:\n%s", path, migrated)
		}
	}
}

func TestMigrateLegacyCertificatePathsHandlesReportedBatchSize(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	for i := 0; i < 16; i++ {
		env.addLegacySite(t, fmt.Sprintf("batch-%02d.example.com", i), certcrypto.EC256)
	}

	result, err := MigrateLegacyCertificatePaths()
	if err != nil {
		t.Fatalf("MigrateLegacyCertificatePaths() error = %v", err)
	}
	if result.MigratedFiles != 16 || result.MigratedSites != 16 || result.SkippedFiles != 0 {
		t.Fatalf("migration result = %+v", result)
	}
}

func TestMigrateLegacyCertificatePathsRestoresBatchWhenReloadFails(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	_, firstPath, _, _ := env.addLegacySite(t, "reload-first.example.com", certcrypto.EC256)
	_, secondPath, _, _ := env.addLegacySite(t, "reload-second.example.com", certcrypto.RSA2048)
	firstBefore, _ := os.ReadFile(firstPath)
	secondBefore, _ := os.ReadFile(secondPath)
	reloadMarker := filepath.Join(t.TempDir(), "reload-attempted")
	settings.NginxSettings.ReloadCmd = fmt.Sprintf(
		"if [ ! -e %q ]; then touch %q; exit 1; fi", reloadMarker, reloadMarker)

	result, err := MigrateLegacyCertificatePaths()
	if err == nil {
		t.Fatal("MigrateLegacyCertificatePaths() expected reload error")
	}
	if result.MigratedFiles != 0 || result.MigratedSites != 0 {
		t.Fatalf("failed migration result = %+v", result)
	}
	firstAfter, _ := os.ReadFile(firstPath)
	secondAfter, _ := os.ReadFile(secondPath)
	if string(firstAfter) != string(firstBefore) || string(secondAfter) != string(secondBefore) {
		t.Fatal("reload failure did not restore every config")
	}
}

func TestMigrateLegacyCertificatePathsDeduplicatesMismatchWarningAndClearsState(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, _, _ := env.addLegacySite(t, "warning.example.com", certcrypto.EC256)
	customCert := filepath.Join(env.confDir, "ssl", "custom", "cert.pem")
	customKey := filepath.Join(env.confDir, "ssl", "custom", "key.pem")
	content := fmt.Sprintf("server { listen 443 ssl; server_name warning.example.com; ssl_certificate %s; ssl_certificate_key %s; }\n",
		customCert, customKey)
	if err := os.WriteFile(configPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write custom mismatch: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := MigrateLegacyCertificatePaths(); err != nil {
			t.Fatalf("audit %d error = %v", i, err)
		}
	}
	var notificationCount int64
	if err := env.db.Model(&model.Notification{}).
		Where("title = ?", "Certificate Configuration Mismatch").Count(&notificationCount).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if notificationCount != 1 {
		t.Fatalf("mismatch notifications = %d, want 1", notificationCount)
	}
	if err := env.db.First(certModel, certModel.ID).Error; err != nil {
		t.Fatalf("reload cert model: %v", err)
	}
	if certModel.LastDeploymentIssueHash == "" || certModel.LastDeploymentIssueNotifyAt == nil {
		t.Fatalf("deployment issue state was not persisted: %+v", certModel)
	}

	consistent := fmt.Sprintf("server { listen 443 ssl; server_name warning.example.com; ssl_certificate %s; ssl_certificate_key %s; }\n",
		certModel.SSLCertificatePath, certModel.SSLCertificateKeyPath)
	if err := os.WriteFile(configPath, []byte(consistent), 0o640); err != nil {
		t.Fatalf("write consistent config: %v", err)
	}
	if _, err := MigrateLegacyCertificatePaths(); err != nil {
		t.Fatalf("consistent audit error = %v", err)
	}
	var cleared model.Cert
	if err := env.db.First(&cleared, certModel.ID).Error; err != nil {
		t.Fatalf("reload cleared cert model: %v", err)
	}
	if cleared.LastDeploymentIssueHash != "" || cleared.LastDeploymentIssueNotifyAt != nil {
		t.Fatalf("deployment issue state was not cleared: %+v", cleared)
	}
}

func TestInspectCertificateDeploymentDoesNotMigrateSharedInclude(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, _, _ := env.addLegacySite(t, "included.example.com", certcrypto.EC256)
	content := "server { listen 443 ssl; server_name included.example.com; include snippets/shared-tls.conf; }\n"
	if err := os.WriteFile(configPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write include config: %v", err)
	}

	status := InspectCertificateDeployment(certModel)
	if status.State != CertificateDeploymentUnreadable || status.AutomaticMigrationAvailable {
		t.Fatalf("deployment status = %+v", status)
	}
}

func TestInspectCertificateDeploymentTreatsSymlinkedManagedFilesAsConsistent(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	certModel, configPath, legacyCertPath, legacyKeyPath := env.addLegacySite(t, "linked.example.com", certcrypto.EC256)
	if err := os.MkdirAll(filepath.Dir(legacyCertPath), 0o755); err != nil {
		t.Fatalf("create legacy dir: %v", err)
	}
	if err := os.Symlink(certModel.SSLCertificatePath, legacyCertPath); err != nil {
		t.Fatalf("link certificate: %v", err)
	}
	if err := os.Symlink(certModel.SSLCertificateKeyPath, legacyKeyPath); err != nil {
		t.Fatalf("link key: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("stat config: %v", err)
	}

	status := InspectCertificateDeployment(certModel)
	if status.State != CertificateDeploymentConsistent {
		t.Fatalf("deployment status = %+v", status)
	}
}
