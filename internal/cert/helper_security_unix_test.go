//go:build unix

package cert

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
)

// certPathValidatorTimeout is far longer than a validation that does not
// open the FIFO needs, and far shorter than the test timeout.
const certPathValidatorTimeout = 2 * time.Second

// makeCertPathTestFIFO creates a FIFO that blocks any open for reading until a
// writer appears. The cleanup opens a writer so a validator that did open it
// is released instead of leaking a blocked goroutine.
func makeCertPathTestFIFO(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	t.Cleanup(func() {
		if w, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = w.Close()
		}
	})
}

// runCertPathValidator runs validate and fails the test if it does not return
// promptly, which is what happens when it opens a FIFO.
func runCertPathValidator(t *testing.T, name string, validate func(string) bool, path string) bool {
	t.Helper()
	done := make(chan bool, 1)
	go func() { done <- validate(path) }()
	select {
	case passed := <-done:
		return passed
	case <-time.After(certPathValidatorTimeout):
		t.Fatalf("%s blocked on %s: the validator opened it", name, path)
		return false
	}
}

func TestCertificatePathValidatorsDoNotOpenFIFOOutsideConfDir(t *testing.T) {
	useCertPathTestConfDir(t)
	fifo := filepath.Join(t.TempDir(), "fifo")
	makeCertPathTestFIFO(t, fifo)

	if !runCertPathValidator(t, "IsCertificatePath", IsCertificatePath, fifo) {
		t.Error("external FIFO path was rejected; external paths must be accepted without inspection")
	}
	if !runCertPathValidator(t, "IsPrivateKeyPath", IsPrivateKeyPath, fifo) {
		t.Error("external FIFO path was rejected; external paths must be accepted without inspection")
	}
}

func TestCertificatePathValidatorsRejectFIFOUnderConfDir(t *testing.T) {
	confDir := useCertPathTestConfDir(t)
	fifo := filepath.Join(confDir, "ssl", "fifo")
	makeCertPathTestFIFO(t, fifo)

	if runCertPathValidator(t, "IsCertificatePath", IsCertificatePath, fifo) {
		t.Error("FIFO under the conf dir was accepted as a certificate")
	}
	if runCertPathValidator(t, "IsPrivateKeyPath", IsPrivateKeyPath, fifo) {
		t.Error("FIFO under the conf dir was accepted as a private key")
	}
}

// /dev/null stands in for /dev/zero: it is the same kind of character device
// but returns EOF at once, so the pre-fix behavior can be shown without
// exhausting memory. Reading it yields no PEM, so a validator that reads it
// rejects it, while one that leaves external paths alone accepts it. The conf
// dir symlink resolves to the device, so it is external as well.
func TestCertificatePathValidatorsDoNotReadDevicesOutsideConfDir(t *testing.T) {
	confDir := useCertPathTestConfDir(t)
	info, err := os.Stat("/dev/null")
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		t.Skip("/dev/null is not a character device here")
	}
	link := filepath.Join(confDir, "ssl", "null.pem")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/null", link); err != nil {
		t.Skip("symlink:", err)
	}

	for _, path := range []string{"/dev/null", link} {
		if !IsCertificatePath(path) || !IsPrivateKeyPath(path) {
			t.Errorf("%s: device outside the conf dir was read instead of being left alone", path)
		}
	}
}

// Editing a self-signed certificate reuses the key at the stored path. That
// path can be changed to anything outside the conf dir through the generic
// modify endpoint, so the reuse must not open it: a FIFO there would block and
// /dev/zero would never stop being read. A fresh key is generated instead.
func TestRegenerateSelfSignedDoesNotOpenExternalKeyPath(t *testing.T) {
	useCertPathTestConfDir(t)
	fifo := filepath.Join(t.TempDir(), "fifo")
	makeCertPathTestFIFO(t, fifo)
	certModel := &model.Cert{SSLCertificateKeyPath: fifo}

	done := make(chan error, 1)
	go func() {
		_, keyPEM, err := RegenerateSelfSignedWithOptions(certModel, SelfSignedOptions{
			CommonName:   "fifo.local",
			DNSNames:     []string{"fifo.local"},
			KeyType:      certcrypto.EC256,
			ValidityDays: 30,
		})
		if err == nil && !IsPrivateKey(string(keyPEM)) {
			err = os.ErrInvalid
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RegenerateSelfSignedWithOptions: %v", err)
		}
	case <-time.After(certPathValidatorTimeout):
		t.Fatal("self-signed key reuse blocked: it opened an external FIFO")
	}
}
