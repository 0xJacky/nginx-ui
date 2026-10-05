package site

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
)

func writeUsageTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func usageServerBlock(certPath string) string {
	return "server {\n    listen 443 ssl;\n    server_name example.com;\n" +
		"    ssl_certificate " + certPath + ";\n    ssl_certificate_key " + certPath + ".key;\n}\n"
}

func TestCertificateUsageIndexFindsSitesAndStreams(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	if err := env.db.AutoMigrate(&model.Stream{}); err != nil {
		t.Fatalf("migrate streams: %v", err)
	}

	certPath := filepath.Join(env.confDir, "ssl", "shared", "fullchain.cer")
	otherCertPath := filepath.Join(env.confDir, "ssl", "other", "fullchain.cer")
	unusedCertPath := filepath.Join(env.confDir, "ssl", "unused", "fullchain.cer")
	for _, path := range []string{certPath, otherCertPath, unusedCertPath} {
		writeUsageTestFile(t, path, "certificate")
	}
	aliasDir := filepath.Join(env.confDir, "ssl", "shared_alias")
	if err := os.Symlink(filepath.Dir(certPath), aliasDir); err != nil {
		t.Fatalf("symlink alias dir: %v", err)
	}

	available := filepath.Join(env.confDir, "sites-available")
	enabled := filepath.Join(env.confDir, "sites-enabled")
	// Enabled, and the same certificate loaded by two server blocks.
	writeUsageTestFile(t, filepath.Join(available, "enabled.conf"),
		usageServerBlock(certPath)+usageServerBlock(certPath))
	if err := os.Symlink(filepath.Join(available, "enabled.conf"),
		filepath.Join(enabled, "enabled.conf")); err != nil {
		t.Fatalf("enable site: %v", err)
	}
	// Disabled, with a path relative to the Nginx configuration directory.
	writeUsageTestFile(t, filepath.Join(available, "disabled.conf"),
		usageServerBlock("ssl/shared/fullchain.cer"))
	// In maintenance, loading two certificates.
	writeUsageTestFile(t, filepath.Join(available, "maintenance.conf"),
		usageServerBlock(certPath)+usageServerBlock(otherCertPath))
	writeUsageTestFile(t, filepath.Join(enabled, "maintenance.conf"+MaintenanceSuffix), "")
	// Deployed to a remote namespace, so its status lives in the database.
	writeUsageTestFile(t, filepath.Join(available, "remote.conf"), usageServerBlock(certPath))
	namespace := &model.Namespace{Name: "remote", DeployMode: model.DeployModeRemote}
	if err := env.db.Create(namespace).Error; err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	if err := env.db.Create(&model.Site{Path: filepath.Join(available, "remote.conf"),
		NamespaceID: namespace.ID, RemoteEnabled: true}).Error; err != nil {
		t.Fatalf("create remote site: %v", err)
	}
	// Variables are resolved at request time and cannot be matched.
	writeUsageTestFile(t, filepath.Join(available, "variable.conf"),
		usageServerBlock("$ssl_server_name.crt"))
	// A directory is not a configuration file.
	if err := os.MkdirAll(filepath.Join(available, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	// A stream reaching the same file through a symlinked directory.
	writeUsageTestFile(t, filepath.Join(env.confDir, "streams-available", "tls.conf"),
		usageServerBlock(filepath.Join(aliasDir, "fullchain.cer")))

	index := BuildCertificateUsageIndex()

	got := index.Lookup(certPath)
	want := []CertificateUsage{
		{Kind: CertificateUsageSite, Name: "enabled.conf", Status: config.StatusEnabled},
		{Kind: CertificateUsageSite, Name: "remote.conf", Status: config.StatusEnabled},
		{Kind: CertificateUsageSite, Name: "maintenance.conf", Status: config.StatusMaintenance},
		{Kind: CertificateUsageSite, Name: "disabled.conf", Status: config.StatusDisabled},
		{Kind: CertificateUsageStream, Name: "tls.conf", Status: config.StatusDisabled},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("usages of shared certificate = %#v, want %#v", got, want)
	}

	got = index.Lookup(otherCertPath)
	want = []CertificateUsage{
		{Kind: CertificateUsageSite, Name: "maintenance.conf", Status: config.StatusMaintenance},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("usages of other certificate = %#v, want %#v", got, want)
	}

	if got = index.Lookup(unusedCertPath); len(got) != 0 {
		t.Fatalf("usages of unused certificate = %#v, want none", got)
	}
	if got = index.Lookup(""); len(got) != 0 {
		t.Fatalf("usages of empty path = %#v, want none", got)
	}
}

func TestCertificateUsageIndexWithoutConfigDirectories(t *testing.T) {
	env := setupCertificateMigrationTest(t)
	if err := os.RemoveAll(filepath.Join(env.confDir, "sites-available")); err != nil {
		t.Fatalf("remove sites-available: %v", err)
	}

	index := BuildCertificateUsageIndex()
	if got := index.Lookup(filepath.Join(env.confDir, "ssl", "a", "fullchain.cer")); len(got) != 0 {
		t.Fatalf("usages = %#v, want none", got)
	}

	var nilIndex *CertificateUsageIndex
	if got := nilIndex.Lookup("/etc/nginx/ssl/a/fullchain.cer"); len(got) != 0 {
		t.Fatalf("usages from nil index = %#v, want none", got)
	}
}
