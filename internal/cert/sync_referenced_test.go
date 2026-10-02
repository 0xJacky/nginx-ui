package cert

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func withSyncConfDir(t *testing.T, files map[string]string) string {
	t.Helper()

	originalConfigDir := settings.NginxSettings.ConfigDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	for relativePath, content := range files {
		path := filepath.Join(confDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return confDir
}

func withSyncDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalModelDB := model.UseDB()
	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err = db.AutoMigrate(&model.Cert{}, &model.Namespace{}, &model.Site{}, &model.Stream{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	model.Use(db)
	t.Cleanup(func() {
		model.Use(originalModelDB)
	})
	return db
}

func TestReferencedSyncPayloadsCollectsLoadablePairs(t *testing.T) {
	withSyncDB(t)
	confDir := withSyncConfDir(t, map[string]string{
		"ssl/example.com_EC256/fullchain.cer": "ec cert",
		"ssl/example.com_EC256/private.key":   "ec key",
		"ssl/example.com_2048/fullchain.cer":  "rsa cert",
		"ssl/example.com_2048/private.key":    "rsa key",
	})
	outside := t.TempDir()

	content := fmt.Sprintf(`server {
    listen 443 ssl;
    ssl_certificate "%[1]s/ssl/example.com_EC256/fullchain.cer";
    ssl_certificate_key "%[1]s/ssl/example.com_EC256/private.key";
    ssl_certificate ssl/example.com_2048/fullchain.cer;
    ssl_certificate_key ssl/example.com_2048/private.key;
    ssl_certificate $ssl_server_name.crt;
    ssl_certificate_key $ssl_server_name.key;
    ssl_certificate %[2]s/outside.cer;
    ssl_certificate_key %[2]s/outside.key;
    proxy_ssl_certificate %[1]s/ssl/upstream.cer;
}
server {
    listen 8443 ssl;
    ssl_certificate %[1]s/ssl/example.com_EC256/fullchain.cer;
    ssl_certificate_key %[1]s/ssl/example.com_EC256/private.key;
}
`, confDir, outside)

	payloads := ReferencedSyncPayloads(content)
	if len(payloads) != 2 {
		t.Fatalf("expected the two pairs below the conf dir once each, got %+v", payloads)
	}

	ec := payloads[0]
	if ec.SSLCertificatePath != filepath.Join(confDir, "ssl/example.com_EC256/fullchain.cer") ||
		ec.SSLCertificateKeyPath != filepath.Join(confDir, "ssl/example.com_EC256/private.key") ||
		ec.SSLCertificate != "ec cert" || ec.SSLCertificateKey != "ec key" || ec.Name != "example.com_EC256" {
		t.Fatalf("unexpected quoted pair: %+v", ec)
	}

	rsa := payloads[1]
	if rsa.SSLCertificatePath != filepath.Join(confDir, "ssl/example.com_2048/fullchain.cer") ||
		rsa.SSLCertificateKeyPath != filepath.Join(confDir, "ssl/example.com_2048/private.key") ||
		rsa.SSLCertificate != "rsa cert" || rsa.SSLCertificateKey != "rsa key" {
		t.Fatalf("relative pair must resolve against the conf dir: %+v", rsa)
	}
}

func TestReferencedSyncPayloadsUsesCertificateRecord(t *testing.T) {
	db := withSyncDB(t)
	confDir := withSyncConfDir(t, map[string]string{
		"ssl/example.com_EC256/fullchain.cer": "cert",
		"ssl/example.com_EC256/private.key":   "key",
	})
	certPath := filepath.Join(confDir, "ssl/example.com_EC256/fullchain.cer")
	keyPath := filepath.Join(confDir, "ssl/example.com_EC256/private.key")
	if err := db.Create(&model.Cert{
		Name:                  "example.com",
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		KeyType:               "P256",
	}).Error; err != nil {
		t.Fatalf("create cert: %v", err)
	}

	// The key directive is missing, so only the record can pair the files.
	payloads := ReferencedSyncPayloads(fmt.Sprintf("server { ssl_certificate %s; }", certPath))
	if len(payloads) != 1 {
		t.Fatalf("expected one pair, got %+v", payloads)
	}
	if payloads[0].Name != "example.com" || payloads[0].SSLCertificateKeyPath != keyPath ||
		payloads[0].SSLCertificateKey != "key" {
		t.Fatalf("pair must come from the certificate record: %+v", payloads[0])
	}
}

func TestReferencedSyncPayloadsSkipsUnreadablePair(t *testing.T) {
	withSyncDB(t)
	confDir := withSyncConfDir(t, nil)

	content := fmt.Sprintf("server { ssl_certificate %[1]s/ssl/missing.cer; ssl_certificate_key %[1]s/ssl/missing.key; }", confDir)
	if payloads := ReferencedSyncPayloads(content); len(payloads) != 0 {
		t.Fatalf("a missing pair cannot be pushed, got %+v", payloads)
	}
}

func TestReferencingNodeIDsCollectsSitesAndStreamsLoadingTheCertificate(t *testing.T) {
	db := withSyncDB(t)
	confDir := withSyncConfDir(t, nil)
	certPath := filepath.Join(confDir, "ssl/example.com_EC256/fullchain.cer")
	loads := fmt.Sprintf("server { ssl_certificate %s; ssl_certificate_key %s; }",
		certPath, filepath.Join(confDir, "ssl/example.com_EC256/private.key"))

	files := map[string]string{
		"sites-available/direct":     loads,
		"sites-available/namespaced": loads,
		"sites-available/unrelated":  "server { listen 80; }",
		"sites-available/local-only": loads,
		"streams-available/tls":      loads,
	}
	for relativePath, content := range files {
		path := filepath.Join(confDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	namespace := &model.Namespace{Name: "edge", SyncNodeIds: []uint64{3, 4}}
	if err := db.Create(namespace).Error; err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	records := []any{
		&model.Site{Path: filepath.Join(confDir, "sites-available/direct"), SyncNodeIDs: []uint64{1, 2}},
		&model.Site{Path: filepath.Join(confDir, "sites-available/namespaced"), NamespaceID: namespace.ID, SyncNodeIDs: []uint64{2}},
		&model.Site{Path: filepath.Join(confDir, "sites-available/unrelated"), SyncNodeIDs: []uint64{9}},
		&model.Site{Path: filepath.Join(confDir, "sites-available/local-only")},
		&model.Stream{Path: filepath.Join(confDir, "streams-available/tls"), SyncNodeIDs: []uint64{5}},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}

	got := referencingNodeIDs(&model.Cert{SSLCertificatePath: certPath})
	slices.Sort(got)
	if want := []uint64{1, 2, 3, 4, 5}; !slices.Equal(got, want) {
		t.Fatalf("referencing nodes = %v, want %v", got, want)
	}
}

func TestSyncToRemoteServerIgnoresOutsidePathWithoutExplicitNodes(t *testing.T) {
	db := withSyncDB(t)
	confDir := withSyncConfDir(t, nil)
	outside := t.TempDir()
	certPath := filepath.Join(outside, "fullchain.cer")
	keyPath := filepath.Join(outside, "private.key")

	sitePath := filepath.Join(confDir, "sites-available/example")
	if err := os.MkdirAll(filepath.Dir(sitePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sitePath,
		[]byte(fmt.Sprintf("server { ssl_certificate %s; ssl_certificate_key %s; }", certPath, keyPath)), 0o644); err != nil {
		t.Fatalf("write site: %v", err)
	}
	if err := db.Create(&model.Site{Path: sitePath, SyncNodeIDs: []uint64{1}}).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}

	c := &model.Cert{SSLCertificatePath: certPath, SSLCertificateKeyPath: keyPath}
	if err := SyncToRemoteServer(c); err != nil {
		t.Fatalf("a certificate outside the conf dir is managed per node, got %v", err)
	}

	c.SyncNodeIds = []uint64{1}
	if err := SyncToRemoteServer(c); err == nil {
		t.Fatal("an explicit sync target for a certificate outside the conf dir must be reported")
	}
}

func TestContentMatchesFiles(t *testing.T) {
	confDir := withSyncConfDir(t, map[string]string{
		"ssl/example/fullchain.cer": "cert",
		"ssl/example/private.key":   "key",
	})
	content := &Content{
		SSLCertificatePath:    filepath.Join(confDir, "ssl/example/fullchain.cer"),
		SSLCertificateKeyPath: filepath.Join(confDir, "ssl/example/private.key"),
		SSLCertificate:        "cert",
		SSLCertificateKey:     "key",
	}
	if !content.MatchesFiles() {
		t.Fatal("identical files must match")
	}

	content.SSLCertificate = "renewed cert"
	if content.MatchesFiles() {
		t.Fatal("a renewed certificate must not match")
	}

	content.SSLCertificate = "cert"
	content.SSLCertificateKeyPath = filepath.Join(confDir, "ssl/example/missing.key")
	if content.MatchesFiles() {
		t.Fatal("a missing key must not match")
	}
}
