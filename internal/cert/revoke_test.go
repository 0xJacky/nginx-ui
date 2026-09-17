package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/acme"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newRefusingACMEServer serves just enough of an ACME directory for lego to
// build a client, and answers every revocation request with a problem document
// of the given type. The returned counter tracks the revocation requests.
func newRefusingACMEServer(t *testing.T, problemType string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var revokeCalls atomic.Int32

	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", "nonce-"+time.Now().Format(time.RFC3339Nano))
		switch r.URL.Path {
		case "/directory":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"newNonce":   srv.URL + "/nonce",
				"newAccount": srv.URL + "/account",
				"newOrder":   srv.URL + "/order",
				"revokeCert": srv.URL + "/revoke",
			})
		case "/nonce":
			w.WriteHeader(http.StatusOK)
		case "/revoke":
			revokeCalls.Add(1)
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"type":   problemType,
				"detail": "revocation refused",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	// lego insists on HTTPS; the test server certificate is self-signed.
	original := settings.HTTPSettings.InsecureSkipVerify
	settings.HTTPSettings.InsecureSkipVerify = true
	t.Cleanup(func() { settings.HTTPSettings.InsecureSkipVerify = original })

	return srv, &revokeCalls
}

func selfSignedLeafPEM(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.com"},
		DNSNames:     []string{"example.com"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// revokeAgainstRefusingCA runs RevokeCert for certPEM against a CA that
// answers every revocation with problemType, and returns the errors reported
// to the caller together with the number of revocation requests the CA saw.
func revokeAgainstRefusingCA(t *testing.T, problemType string, certPEM []byte) ([]error, int32) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AcmeUser{}))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })

	srv, revokeCalls := newRefusingACMEServer(t, problemType)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	user := &model.AcmeUser{
		Name:  "test",
		Email: "test@example.com",
		CADir: srv.URL + "/directory",
		Registration: model.AcmeRegistration{
			URI: srv.URL + "/account/1",
		},
		Key: model.PrivateKey{X: key.X, Y: key.Y, D: key.D},
	}
	require.NoError(t, db.Create(user).Error)

	payload := &ConfigPayload{
		ACMEUserID: user.ID,
		Resource:   &model.CertificateResource{Certificate: certPEM},
	}

	logChan := make(chan string, 1)
	errChan := make(chan error, 1)
	go func() {
		for range logChan {
		}
	}()

	certLogger := NewLogger()
	defer certLogger.Close()

	RevokeCert(payload, certLogger, logChan, errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errs, revokeCalls.Load()
}

func validLeafPEM(t *testing.T) []byte {
	t.Helper()
	return selfSignedLeafPEM(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
}

func TestRevokeCertReportsCARefusal(t *testing.T) {
	errs, _ := revokeAgainstRefusingCA(t, acme.UnauthorizedErrorType, validLeafPEM(t))

	require.Len(t, errs, 1, "a revocation the CA refused must reach the caller as an error")
	require.ErrorContains(t, errs[0], "revocation refused")
}

func TestRevokeCertTreatsAlreadyRevokedAsRevoked(t *testing.T) {
	errs, revokeCalls := revokeAgainstRefusingCA(t, acme.AlreadyRevokedErrorType, validLeafPEM(t))

	require.EqualValues(t, 1, revokeCalls)
	require.Empty(t, errs, "a certificate the CA has already revoked needs no further revocation")
}

func TestRevokeCertSkipsExpiredCertificate(t *testing.T) {
	expired := selfSignedLeafPEM(t, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	errs, revokeCalls := revokeAgainstRefusingCA(t, acme.UnauthorizedErrorType, expired)

	require.Empty(t, errs, "an expired certificate needs no revocation")
	require.Zero(t, revokeCalls, "an expired certificate should not be sent to the CA")
}
