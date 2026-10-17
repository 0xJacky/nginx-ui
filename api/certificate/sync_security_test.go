package certificate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
)

func syncSecurityRequest(t *testing.T, payload cert.SyncCertificatePayload) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/cert_sync", SyncCertificate)
	req := httptest.NewRequest(http.MethodPut, "/cert_sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func setupSyncSecurityPayload(t *testing.T) cert.SyncCertificatePayload {
	t.Helper()
	root := t.TempDir()
	previous := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = root
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	settings.NginxSettings.PIDPath = filepath.Join(root, "absent.pid")
	// Never run a real nginx command, even when an assertion fails.
	settings.NginxSettings.TestConfigCmd = "true"
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	t.Cleanup(func() { *settings.NginxSettings = previous })
	certPEM, keyPEM := newDownloadTestPair(t)
	return cert.SyncCertificatePayload{Name: "sync-example", SSLCertificatePath: filepath.Join(root, "custom", "chain.pem"),
		SSLCertificateKeyPath: filepath.Join(root, "custom", "private.key"), SSLCertificate: string(certPEM), SSLCertificateKey: string(keyPEM), KeyType: certcrypto.EC256}
}

func TestSyncCertificateRejectsInvalidInputBeforeDatabaseOrFiles(t *testing.T) {
	for _, which := range []string{"config payload", "valid pair to config", "trailing directives", "mismatch", "missing key"} {
		t.Run(which, func(t *testing.T) {
			db := setupCertTestDB(t)
			payload := setupSyncSecurityPayload(t)
			switch which {
			case "config payload":
				payload.SSLCertificate = "user root;"
			case "valid pair to config":
				payload.SSLCertificatePath = settings.NginxSettings.ConfigPath
			case "trailing directives":
				payload.SSLCertificateKey += "\njs_import app.js;"
			case "mismatch":
				_, otherKey := newDownloadTestPair(t)
				payload.SSLCertificateKey = string(otherKey)
			case "missing key":
				payload.SSLCertificateKey = ""
			}
			rec := syncSecurityRequest(t, payload)
			if rec.Code == http.StatusOK {
				t.Fatalf("invalid sync succeeded: %s", rec.Body.String())
			}
			var count int64
			if err := db.Model(&model.Cert{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("invalid sync created %d records", count)
			}
			if _, err := os.Stat(payload.SSLCertificatePath); !os.IsNotExist(err) {
				t.Fatalf("invalid sync touched target: %v", err)
			}
		})
	}
}

func TestSyncCertificateValidPairIsIdempotentAndRollsBackFailedTest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses harmless POSIX test commands")
	}
	setupCertTestDB(t)
	payload := setupSyncSecurityPayload(t)
	first := syncSecurityRequest(t, payload)
	if first.Code != http.StatusOK {
		t.Fatalf("first sync failed: %d %s", first.Code, first.Body.String())
	}
	before, err := os.Stat(payload.SSLCertificatePath)
	if err != nil {
		t.Fatal(err)
	}
	// Exact replicas must not test/reload at all.
	settings.NginxSettings.TestConfigCmd = "false"
	repeated := syncSecurityRequest(t, payload)
	if repeated.Code != http.StatusOK {
		t.Fatalf("unchanged sync ran failing test: %s", repeated.Body.String())
	}
	after, err := os.Stat(payload.SSLCertificatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged sync rewrote certificate")
	}
	oldCert, oldKey := payload.SSLCertificate, payload.SSLCertificateKey
	expectedPath := filepath.Join(settings.NginxSettings.ConfigDir, "expected.pem")
	if err := os.WriteFile(expectedPath, []byte(oldCert), 0600); err != nil {
		t.Fatal(err)
	}
	settings.NginxSettings.TestConfigCmd = "cmp -s '" + payload.SSLCertificatePath + "' '" + expectedPath + "'"
	nextCert, nextKey := newDownloadTestPair(t)
	payload.SSLCertificate, payload.SSLCertificateKey = string(nextCert), string(nextKey)
	failed := syncSecurityRequest(t, payload)
	if failed.Code == http.StatusOK {
		t.Fatal("failed config test returned success")
	}
	for path, want := range map[string]string{payload.SSLCertificatePath: oldCert, payload.SSLCertificateKeyPath: oldKey} {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(got) != want {
			t.Fatalf("rollback did not restore %s", path)
		}
	}
}

func TestSyncCertificateStagesPairsWhileExistingConfigIsUnhealthy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses harmless POSIX test commands")
	}
	db := setupCertTestDB(t)
	first := setupSyncSecurityPayload(t)
	// Model an offline node or a config referencing multiple missing pairs.
	// Neither condition may prevent the first pair from being staged.
	settings.NginxSettings.TestConfigCmd = "false"
	settings.NginxSettings.RestartCmd = "false"
	for _, name := range []string{"first", "second"} {
		payload := first
		payload.Name = name
		payload.SSLCertificatePath = filepath.Join(settings.NginxSettings.ConfigDir, name, "chain.pem")
		payload.SSLCertificateKeyPath = filepath.Join(settings.NginxSettings.ConfigDir, name, "private.key")
		rec := syncSecurityRequest(t, payload)
		if rec.Code != http.StatusOK {
			t.Fatalf("valid staged pair rejected: %d %s", rec.Code, rec.Body.String())
		}
		var result struct {
			ReloadPending bool `json:"reload_pending"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !result.ReloadPending {
			t.Fatal("deferred reload was not reported")
		}
		if _, err := os.Stat(payload.SSLCertificatePath); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(payload.SSLCertificateKeyPath); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(&model.Cert{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("accepted staged pairs retained %d records, want 2", count)
	}

}

func TestSyncCertificateRejectedNewPairRollsBackMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses harmless POSIX test commands")
	}
	db := setupCertTestDB(t)
	payload := setupSyncSecurityPayload(t)
	// The baseline succeeds before the new certificate exists; the same test
	// fails after the write, reproducing healthy-to-rejected config application.
	settings.NginxSettings.TestConfigCmd = "test ! -f '" + payload.SSLCertificatePath + "'"
	rejected := syncSecurityRequest(t, payload)
	if rejected.Code == http.StatusOK {
		t.Fatal("rejected new pair returned success")
	}
	var count int64
	if err := db.Model(&model.Cert{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rejected sync left %d certificate records", count)
	}
	for _, path := range []string{payload.SSLCertificatePath, payload.SSLCertificateKeyPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("rejected sync left file %s: %v", path, err)
		}
	}
	// A retry must create exactly one record, and an unchanged repeat must keep it.
	settings.NginxSettings.TestConfigCmd = "true"
	for i := 0; i < 2; i++ {
		rec := syncSecurityRequest(t, payload)
		if rec.Code != http.StatusOK {
			t.Fatalf("accepted retry failed: %d %s", rec.Code, rec.Body.String())
		}
	}
	if err := db.Model(&model.Cert{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("retry/idempotent sync created %d records", count)
	}
}

func TestSyncCertificateReloadFailureRestoresPairWithoutFalseRollbackError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses harmless POSIX test commands")
	}
	setupCertTestDB(t)
	payload := setupSyncSecurityPayload(t)
	if response := syncSecurityRequest(t, payload); response.Code != http.StatusOK {
		t.Fatalf("initial pair failed: %s", response.Body.String())
	}
	oldCertificate, oldKey := payload.SSLCertificate, payload.SSLCertificateKey
	certificate, key := newDownloadTestPair(t)
	payload.SSLCertificate, payload.SSLCertificateKey = string(certificate), string(key)
	marker := filepath.Join(t.TempDir(), "reload-attempt")
	quotedMarker := "'" + strings.ReplaceAll(marker, "'", "'\\''") + "'"
	settings.NginxSettings.ReloadCmd = "if test -f " + quotedMarker + "; then true; else touch " + quotedMarker + "; false; fi"
	// Reload falls back to the configured start command when no nginx PID exists.
	settings.NginxSettings.RestartCmd = settings.NginxSettings.ReloadCmd
	response := syncSecurityRequest(t, payload)
	if response.Code == http.StatusOK {
		t.Fatal("failed reload returned success")
	}
	if strings.Contains(response.Body.String(), "rollback failed") {
		t.Fatalf("successful restore/reload was reported as failed: %s", response.Body.String())
	}
	for path, want := range map[string]string{payload.SSLCertificatePath: oldCertificate, payload.SSLCertificateKeyPath: oldKey} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("reload failure did not restore %s: %v", path, err)
		}
	}
}
