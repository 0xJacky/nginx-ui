package certificate

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/validation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var selfSignedValidationOnce sync.Once

func setupSelfSignedAPITest(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	selfSignedValidationOnce.Do(validation.Init)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Cert{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	return db
}

func TestSelfSignedSlugSanitizesPathTraversal(t *testing.T) {
	if got := selfSignedSlug("../../etc"); got != "etc" {
		t.Fatalf("selfSignedSlug() = %q, want %q", got, "etc")
	}
}

func TestSelfSignedSlugFallsBackForEmptyInput(t *testing.T) {
	if got := selfSignedSlug(""); got != defaultSelfSignedSlug {
		t.Fatalf("selfSignedSlug() = %q, want %q", got, defaultSelfSignedSlug)
	}
}

func TestSelfSignedSlugConvertsIDNToPunycode(t *testing.T) {
	if got := selfSignedSlug("例如.test"); got != "xn--fsqu6v.test" {
		t.Fatalf("selfSignedSlug() = %q, want %q", got, "xn--fsqu6v.test")
	}
}

func TestBuildSelfSignedOptionsRejectsEmptySAN(t *testing.T) {
	_, err := buildSelfSignedOptions(&SelfSignedCertRequest{
		Domains:     []string{" ", "\t"},
		IPAddresses: []string{""},
	})
	if !errors.Is(err, cert.ErrSelfSignedNoSAN) {
		t.Fatalf("buildSelfSignedOptions() error = %v, want %v", err, cert.ErrSelfSignedNoSAN)
	}
}

func TestBuildSelfSignedOptionsNormalizesValues(t *testing.T) {
	opts, err := buildSelfSignedOptions(&SelfSignedCertRequest{
		Domains:      []string{" example.com ", "", "www.example.com"},
		IPAddresses:  []string{" 127.0.0.1 "},
		KeyType:      string(certcrypto.EC256),
		ValidityDays: 30,
	})
	if err != nil {
		t.Fatalf("buildSelfSignedOptions() error = %v", err)
	}
	if opts.CommonName != "example.com" {
		t.Fatalf("CommonName = %q, want %q", opts.CommonName, "example.com")
	}
	if len(opts.DNSNames) != 2 || opts.DNSNames[0] != "example.com" || opts.DNSNames[1] != "www.example.com" {
		t.Fatalf("DNSNames = %#v, want normalized domains", opts.DNSNames)
	}
	if len(opts.IPAddresses) != 1 || opts.IPAddresses[0] != "127.0.0.1" {
		t.Fatalf("IPAddresses = %#v, want normalized IP", opts.IPAddresses)
	}
}

func TestGenerateSelfSignedCertRollsBackDBOnFileWriteFailure(t *testing.T) {
	db := setupSelfSignedAPITest(t)

	originalConfigDir := settings.NginxSettings.ConfigDir
	blockedBase := filepath.Join(t.TempDir(), "nginx.conf.d")
	if err := os.WriteFile(blockedBase, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}
	settings.NginxSettings.ConfigDir = blockedBase
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	router := gin.New()
	router.POST("/self_signed_cert", GenerateSelfSignedCert)

	body, err := json.Marshal(SelfSignedCertRequest{
		Name:         "rollback-test",
		Domains:      []string{"rollback.example"},
		KeyType:      string(certcrypto.EC256),
		ValidityDays: 30,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/self_signed_cert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusOK {
		t.Fatalf("expected write failure response, got HTTP %d", recorder.Code)
	}

	var count int64
	if err := db.Model(&model.Cert{}).Count(&count).Error; err != nil {
		t.Fatalf("count cert rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected DB rollback to remove cert row, got %d rows", count)
	}
}

func TestCleanupSelfSignedCertFilesRemovesManagedDirectory(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	certDir := filepath.Join(confDir, "ssl", "example_1")
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		t.Fatalf("create cert dir: %v", err)
	}
	certPath := filepath.Join(certDir, "fullchain.cer")
	keyPath := filepath.Join(certDir, "private.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	cleanupSelfSignedCertFiles(&model.Cert{
		AutoCert:              model.AutoCertSelfSigned,
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
	})

	if _, err := os.Stat(certDir); !os.IsNotExist(err) {
		t.Fatalf("expected managed cert directory to be removed, stat err: %v", err)
	}
}
