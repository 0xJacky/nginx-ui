package certificate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/gin-gonic/gin"
)

func postCertificateArchive(t *testing.T, archive []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "example.com_nginx.zip")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write(archive); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err = form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/cert_parse_archive", ParseCertificateArchive)

	req := httptest.NewRequest(http.MethodPost, "/cert_parse_archive", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestParseCertificateArchiveRoundTripsDownloadedZip(t *testing.T) {
	certPEM, keyPEM := newDownloadTestPair(t)

	// What the download dialog produces must import back unchanged.
	archive, err := buildCertificateZip(downloadCertFileName("*.example.com"), []string{"crt", "key", "pfx"}, certPEM, keyPEM, "")
	if err != nil {
		t.Fatalf("buildCertificateZip() error = %v", err)
	}

	rec := postCertificateArchive(t, archive)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var result cert.ArchiveCertificate
	if err = json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.SSLCertificate != string(certPEM) || result.SSLCertificateKey != string(keyPEM) {
		t.Fatal("imported pair differs from the exported pair")
	}
	if result.Name != "*.example.com" {
		t.Fatalf("name = %q, want *.example.com", result.Name)
	}
}

func TestParseCertificateArchiveRejectsArchiveWithoutKey(t *testing.T) {
	certPEM, _ := newDownloadTestPair(t)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	entry, err := w.Create("only.crt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err = entry.Write(certPEM); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	rec := postCertificateArchive(t, buf.Bytes())
	if rec.Code == http.StatusOK {
		t.Fatalf("status = %d, want an error", rec.Code)
	}

	var resp struct {
		Code  int32  `json:"code"`
		Scope string `json:"scope"`
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (%s)", err, rec.Body.String())
	}
	if resp.Scope != "cert" || resp.Code != 50063 {
		t.Fatalf("error = %s, want cert/50063", rec.Body.String())
	}
}
