package certificate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

func postCertRecommendation(t *testing.T, domains []string) RecommendCertResponse {
	t.Helper()

	body, err := json.Marshal(RecommendCertRequest{Domains: domains})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	router := gin.New()
	router.POST("/cert_recommendation", RecommendCert)
	req := httptest.NewRequest(http.MethodPost, "/cert_recommendation", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response RecommendCertResponse
	if err = json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestRecommendCertPicksTheCoveringCertificate(t *testing.T) {
	db := setupSelfSignedAPITest(t)
	confDir := t.TempDir()
	previousConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = previousConfigDir })

	addCertificate := func(name string, dnsNames ...string) *model.Cert {
		t.Helper()
		certPEM, keyPEM, err := cert.GenerateSelfSigned(cert.SelfSignedOptions{DNSNames: dnsNames, KeyType: "P256"})
		if err != nil {
			t.Fatalf("generate certificate: %v", err)
		}
		dir := filepath.Join(confDir, "ssl", name)
		if err = os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create directory: %v", err)
		}
		record := &model.Cert{
			Name:                  name,
			Domains:               dnsNames,
			SSLCertificatePath:    filepath.Join(dir, "fullchain.cer"),
			SSLCertificateKeyPath: filepath.Join(dir, "private.key"),
		}
		if err = os.WriteFile(record.SSLCertificatePath, certPEM, 0o644); err != nil {
			t.Fatalf("write certificate: %v", err)
		}
		if err = os.WriteFile(record.SSLCertificateKeyPath, keyPEM, 0o600); err != nil {
			t.Fatalf("write key: %v", err)
		}
		if err = db.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
		return record
	}

	addCertificate("def", "*.def.cn")
	abc := addCertificate("abc", "*.abc.cn")

	response := postCertRecommendation(t, []string{"WWW.abc.cn."})
	if response.Certificate == nil {
		t.Fatal("no certificate recommended for www.abc.cn")
	}
	if response.Certificate.ID != abc.ID {
		t.Fatalf("recommended certificate = %d, want %d", response.Certificate.ID, abc.ID)
	}
	if response.Certificate.CertificateInfo == nil || response.Certificate.CertificateInfo.NotAfter.IsZero() {
		t.Fatal("the recommendation carries no certificate info")
	}
	if response.Certificate.SSLCertificateKey != "" {
		t.Fatal("the recommendation must not carry the private key")
	}

	if response = postCertRecommendation(t, []string{"www.abc.cn", "www.def.cn"}); response.Certificate != nil {
		t.Fatalf("recommended %d although no certificate covers both domains", response.Certificate.ID)
	}
	if response = postCertRecommendation(t, []string{"_"}); response.Certificate != nil {
		t.Fatal("recommended a certificate for an invalid domain")
	}
	if response = postCertRecommendation(t, nil); response.Certificate != nil {
		t.Fatal("recommended a certificate without domains")
	}
}
