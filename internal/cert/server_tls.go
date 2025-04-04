package cert

import (
	"crypto/tls"
	"errors"
	"sync/atomic"

	cSettings "github.com/uozi-tech/cosy/settings"
)

var tlsCert atomic.Value

// LoadServerTLSCertificate loads the TLS certificate
func LoadServerTLSCertificate() error {
	return ReloadServerTLSCertificate()
}

// ReloadServerTLSCertificate reloads the TLS certificate
func ReloadServerTLSCertificate() error {
	newCert, err := tls.LoadX509KeyPair(cSettings.ServerSettings.SSLCert, cSettings.ServerSettings.SSLKey)
	if err != nil {
		return err
	}

	tlsCert.Store(newCert)
	return nil
}

// GetServerTLSCertificate returns the current TLS certificate
func GetServerTLSCertificate() (*tls.Certificate, error) {
	cert, ok := tlsCert.Load().(*tls.Certificate)
	if !ok {
		return nil, errors.New("no certificate available")
	}
	return cert, nil
}
