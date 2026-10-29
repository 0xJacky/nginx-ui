//go:build unix

package certificate

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// A FIFO outside the configuration root blocks any reader until a writer
// appears. The request must finish promptly, which proves the validators do
// not open the path at all.
func TestCertPathValidationDoesNotOpenExternalFIFO(t *testing.T) {
	setupCertPathAPITest(t)
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	// Release a validator that did open the FIFO instead of leaking it.
	t.Cleanup(func() {
		if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = w.Close()
		}
	})

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- certPathAPIRequest(t, "/certs", gin.H{
			"name":                     "fifo",
			"ssl_certificate_path":     fifo,
			"ssl_certificate_key_path": fifo,
		})
	}()
	select {
	case rec := <-done:
		if rec.Code != http.StatusOK {
			t.Fatalf("add with an external FIFO path: HTTP %d %s", rec.Code, rec.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request blocked: the certificate path validators opened an external FIFO")
	}
}
