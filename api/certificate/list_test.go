package certificate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	cosySettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
)

type certListTestRow struct {
	ID               uint64 `json:"id"`
	Name             string `json:"name"`
	State            string `json:"state"`
	RenewalMethod    string `json:"renewal_method"`
	DNSProvider      string `json:"dns_provider"`
	LastRenewalError string `json:"last_renewal_error"`
}

type certListTestResponse struct {
	Data       []certListTestRow `json:"data"`
	Counts     *CertListCounts   `json:"counts"`
	Pagination struct {
		Total int64 `json:"total"`
	} `json:"pagination"`
}

func getCertList(t *testing.T, query string) certListTestResponse {
	t.Helper()

	router := gin.New()
	router.GET("/certs", GetCertList)
	req := httptest.NewRequest(http.MethodGet, "/certs?"+query, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response certListTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestGetCertListFiltersByStateAndCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Cert{}, model.DnsCredential{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "cert-list.db")))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })

	confDir := t.TempDir()
	previousConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	previousPageSize := cosySettings.AppSettings.PageSize
	cosySettings.AppSettings.PageSize = 20
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = previousConfigDir
		cosySettings.AppSettings.PageSize = previousPageSize
	})

	// Freeze the clock so the self-signed certificates below are ten years
	// away from expiry, then move it to make some of them expire.
	now := time.Now()
	previousNow := overviewNow
	overviewNow = func() time.Time { return now }
	t.Cleanup(func() { overviewNow = previousNow })

	credential := &model.DnsCredential{Name: "main", Provider: "Cloudflare", ProviderCode: "cloudflare"}
	if err := db.Create(credential).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	addCertificate := func(record *model.Cert, withFile bool) *model.Cert {
		t.Helper()
		if withFile {
			certPEM, keyPEM, err := cert.GenerateSelfSigned(cert.SelfSignedOptions{DNSNames: record.Domains, KeyType: "P256", ValidityDays: 20})
			if err != nil {
				t.Fatalf("generate certificate: %v", err)
			}
			dir := filepath.Join(confDir, "ssl", record.Name)
			if err = os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("create directory: %v", err)
			}
			record.SSLCertificatePath = filepath.Join(dir, "fullchain.cer")
			record.SSLCertificateKeyPath = filepath.Join(dir, "private.key")
			if err = os.WriteFile(record.SSLCertificatePath, certPEM, 0o644); err != nil {
				t.Fatalf("write certificate: %v", err)
			}
			if err = os.WriteFile(record.SSLCertificateKeyPath, keyPEM, 0o600); err != nil {
				t.Fatalf("write key: %v", err)
			}
		}
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
		return record
	}

	addCertificate(&model.Cert{
		Name: "dns.example.com", Domains: []string{"dns.example.com"}, AutoCert: model.AutoCertEnabled,
		ChallengeMethod: cert.DNS01, DnsCredentialID: credential.ID,
	}, true)
	failedAt := now.Add(-time.Hour)
	addCertificate(&model.Cert{
		Name: "shop.example.org", Domains: []string{"shop.example.org"}, AutoCert: model.AutoCertEnabled,
		ChallengeMethod: cert.HTTP01, Status: model.CertStatusFailure, LastError: "challenge failed", LastAttemptAt: &failedAt,
	}, true)
	addCertificate(&model.Cert{
		Name: "draft.example.net", Domains: []string{"draft.example.net"}, AutoCert: model.AutoCertEnabled,
		ChallengeMethod: cert.HTTP01,
	}, false)

	all := getCertList(t, "with_counts=true")
	if all.Counts == nil {
		t.Fatal("counts are missing")
	}
	if got := *all.Counts; got != (CertListCounts{All: 3, Expiring: 2, Failed: 1, Expired: 0}) {
		t.Fatalf("counts = %+v", got)
	}
	states := map[string]certListTestRow{}
	for _, row := range all.Data {
		states[row.Name] = row
	}
	if states["draft.example.net"].State != string(cert.StateNotIssued) {
		t.Fatalf("draft state = %q, want not_issued", states["draft.example.net"].State)
	}
	if row := states["dns.example.com"]; row.RenewalMethod != string(cert.RenewalMethodDNS01) || row.DNSProvider != "Cloudflare" {
		t.Fatalf("dns row = %+v", row)
	}
	if row := states["shop.example.org"]; row.State != string(cert.StateFailed) || row.LastRenewalError != "challenge failed" {
		t.Fatalf("failed row = %+v", row)
	}

	failed := getCertList(t, "state=failed")
	if len(failed.Data) != 1 || failed.Data[0].Name != "shop.example.org" || failed.Pagination.Total != 1 {
		t.Fatalf("failed filter = %+v", failed)
	}
	if failed.Counts != nil {
		t.Fatal("counts are only sent when asked for")
	}

	search := getCertList(t, "keyword=SHOP&with_counts=true")
	if len(search.Data) != 1 || search.Counts.All != 1 {
		t.Fatalf("keyword search = %+v", search)
	}

	now = now.Add(30 * 24 * time.Hour)
	expired := getCertList(t, "state=expired&with_counts=true")
	if len(expired.Data) != 2 || expired.Counts.Expired != 2 || expired.Counts.Expiring != 0 {
		t.Fatalf("expired filter = %+v", expired)
	}
}

func TestSetCertAutoRenewal(t *testing.T) {
	db := setupSelfSignedAPITest(t)

	acme := &model.Cert{Name: "acme", Domains: []string{"acme.example.com"}, ChallengeMethod: cert.HTTP01, AutoCert: model.AutoCertEnabled}
	selfSigned := &model.Cert{Name: "self", Domains: []string{"self.example.com"}, AutoCert: model.AutoCertSelfSigned}
	for _, record := range []*model.Cert{acme, selfSigned} {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}

	router := gin.New()
	router.POST("/certs/:id/auto_renewal", SetCertAutoRenewal)
	post := func(id uint64, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/certs/"+strconv.FormatUint(id, 10)+"/auto_renewal", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(acme.ID, `{"enabled":false}`); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	var stored model.Cert
	if err := db.First(&stored, acme.ID).Error; err != nil {
		t.Fatalf("load record: %v", err)
	}
	if stored.AutoCert != model.AutoCertDisabled {
		t.Fatalf("auto_cert = %d, want disabled", stored.AutoCert)
	}

	if code := post(selfSigned.ID, `{"enabled":false}`); code == http.StatusOK {
		t.Fatal("a self-signed certificate must not switch through this endpoint")
	}
	stored = model.Cert{}
	if err := db.First(&stored, selfSigned.ID).Error; err != nil {
		t.Fatalf("load record: %v", err)
	}
	if stored.AutoCert != model.AutoCertSelfSigned {
		t.Fatalf("auto_cert = %d, want self-signed", stored.AutoCert)
	}

	if code := post(acme.ID, `{}`); code == http.StatusOK {
		t.Fatal("the enabled flag is required")
	}
}
