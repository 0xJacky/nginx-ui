package certificate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/internal/validation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
)

// setupCertPathAPITest points nginx at a temporary configuration root, never
// runs a real nginx command, and returns that root.
func setupCertPathAPITest(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	selfSignedValidationOnce.Do(validation.Init)
	// AddCert and ModifyCert go through cosy's create and modify chains, which
	// need the model registered with cosy, not only migrated.
	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Cert{}, model.DnsCredential{}, model.AcmeUser{})
	db := cosy.InitDB(sqlite.Open(testdb.DSN(t)))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })

	root := t.TempDir()
	previous := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = root
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	settings.NginxSettings.PIDPath = filepath.Join(root, "absent.pid")
	settings.NginxSettings.TestConfigCmd = "true"
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	t.Cleanup(func() { *settings.NginxSettings = previous })
	return root
}

func certPathAPIRequest(t *testing.T, target string, payload gin.H) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/certs", AddCert)
	router.POST("/certs/:id", ModifyCert)
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// External certificate paths are stored without inspecting the files, so a
// non-PEM file outside the configuration root gives the same answer as a
// missing one and the record keeps saving on add and modify.
func TestCertPathValidationAcceptsExternalPathsWithoutInspection(t *testing.T) {
	setupCertPathAPITest(t)
	outside := t.TempDir()
	plain := filepath.Join(outside, "passwd")
	if err := os.WriteFile(plain, []byte("root:x:0:0:root:/root:/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := certPathAPIRequest(t, "/certs", gin.H{
		"name":                     "external",
		"ssl_certificate_path":     plain,
		"ssl_certificate_key_path": filepath.Join(outside, "missing.key"),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("add with external paths: HTTP %d %s", rec.Code, rec.Body.String())
	}
	var created model.Cert
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("decode created cert: %v %s", err, rec.Body.String())
	}

	rec = certPathAPIRequest(t, fmt.Sprintf("/certs/%d", created.ID), gin.H{
		"name":                     "external",
		"ssl_certificate_path":     filepath.Join(outside, "missing.pem"),
		"ssl_certificate_key_path": plain,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("modify with external paths: HTTP %d %s", rec.Code, rec.Body.String())
	}
}

// Files under the configuration root are still checked, and a rejection
// keeps the response shape the frontend already handles.
func TestCertPathValidationRejectsInvalidContentUnderConfDir(t *testing.T) {
	root := setupCertPathAPITest(t)
	plain := filepath.Join(root, "nginx.conf")
	if err := os.WriteFile(plain, []byte("user nginx;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := certPathAPIRequest(t, "/certs", gin.H{
		"name":                     "invalid",
		"ssl_certificate_path":     plain,
		"ssl_certificate_key_path": plain,
	})
	if rec.Code != http.StatusNotAcceptable {
		t.Fatalf("add with non-PEM files under the conf dir: HTTP %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"ssl_certificate_path":"required,certificate_path"`,
		`"ssl_certificate_key_path":"required,privatekey_path"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("response %s does not contain %s", body, want)
		}
	}
}
