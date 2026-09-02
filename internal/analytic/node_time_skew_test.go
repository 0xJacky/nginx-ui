package analytic

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

const fourSecondNodeClockSkew = 4 * time.Second

func TestNodeTLSHandshakeFailsWhileCertificateIsFourSecondsInFuture(t *testing.T) {
	const secret = "time-skew-node-secret"

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Node-Secret") != secret {
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(NodeInfo{Version: "time-skew-test"}); err != nil {
			t.Errorf("encode node info: %v", err)
		}
	}))

	serverIP := server.Listener.Addr().(*net.TCPAddr).IP
	serverCertificate, rootCertificate, notBefore := newFutureNodeCertificate(t, serverIP, fourSecondNodeClockSkew)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		MinVersion:   tls.VersionTLS12,
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	node := &model.Node{
		Model: model.Model{ID: 49},
		Name:  "four-second-clock-skew",
		URL:   server.URL,
	}
	setupLegacyNodeAuthForTest(t, node, secret)

	roots := x509.NewCertPool()
	roots.AddCert(rootCertificate)
	client := &http.Client{
		Transport: nodeauth.NewTransport(node, &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
		}}),
		Timeout: 2 * time.Second,
	}
	t.Cleanup(client.CloseIdleConnections)

	requestURL := server.URL + "/api/node"
	response, err := client.Get(requestURL)
	require.Nil(t, response)
	require.ErrorContains(t, err, "is not yet valid")

	wait := time.Until(notBefore.Add(100 * time.Millisecond))
	require.Positive(t, wait, "test setup took longer than the four-second skew window")
	timer := time.NewTimer(wait)
	t.Cleanup(func() { timer.Stop() })
	<-timer.C

	response, err = client.Get(requestURL)
	require.NoError(t, err)
	t.Cleanup(func() { response.Body.Close() })
	require.Equal(t, http.StatusOK, response.StatusCode)
}

func newFutureNodeCertificate(t *testing.T, serverIP net.IP, futureOffset time.Duration) (
	tls.Certificate,
	*x509.Certificate,
	time.Time,
) {
	t.Helper()

	rootPublicKey, rootPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Nginx UI node time-skew test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, rootPublicKey, rootPrivateKey)
	require.NoError(t, err)
	rootCertificate, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	leafPublicKey, leafPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	notBefore := time.Now().Add(futureOffset).Truncate(time.Second)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Nginx UI child node"},
		IPAddresses:  []net.IP{serverIP},
		NotBefore:    notBefore,
		NotAfter:     notBefore.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, rootTemplate, leafPublicKey, rootPrivateKey)
	require.NoError(t, err)

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(leafPrivateKey)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	serverCertificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	require.NoError(t, err)

	return serverCertificate, rootCertificate, notBefore
}
