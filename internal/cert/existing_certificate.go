package cert

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/netip"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// CertificatePair is an installed certificate and private key file pair.
type CertificatePair struct {
	// Leaf is the first certificate of the chain.
	Leaf *x509.Certificate
	// Info summarizes Leaf.
	Info *Info
	// KeyType is the key type of Leaf ("2048", "P256", ...), empty when it
	// is not one of the key types Nginx UI issues.
	KeyType string
}

// LoadCertificatePair reads the PEM certificate chain at certPath and the PEM
// private key at keyPath, both of which must be under the nginx configuration
// directory, and checks that the key belongs to the certificate.
func LoadCertificatePair(certPath, keyPath string) (*CertificatePair, error) {
	certPath = strings.TrimSpace(certPath)
	keyPath = strings.TrimSpace(keyPath)
	if certPath == "" || keyPath == "" {
		return nil, ErrCertPathIsEmpty
	}
	confDir := nginx.GetConfPath()
	if !helper.IsUnderDirectory(certPath, confDir) || !helper.IsUnderDirectory(keyPath, confDir) {
		return nil, ErrCertPathIsNotUnderTheNginxConfDir
	}

	certPEM, err := nginx.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := nginx.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate or key: %w", err)
	}
	leaf := pair.Leaf
	if leaf == nil {
		if leaf, err = x509.ParseCertificate(pair.Certificate[0]); err != nil {
			return nil, ErrCertParse
		}
	}

	keyType, _ := GetKeyType(string(certPEM))
	return &CertificatePair{Leaf: leaf, Info: certificateInfo(leaf), KeyType: keyType}, nil
}

// CertificateCoversIdentifier reports whether leaf is valid for identifier:
// a host name (matched against the DNS SANs, including wildcard SANs), a
// wildcard name (which needs the identical wildcard SAN) or an IP address
// (matched against the IP SANs). The subject common name is not considered,
// like in browsers.
func CertificateCoversIdentifier(leaf *x509.Certificate, identifier string) bool {
	if leaf == nil {
		return false
	}
	identifier = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(identifier)), ".")
	if identifier == "" {
		return false
	}

	ipCandidate := strings.TrimSuffix(strings.TrimPrefix(identifier, "["), "]")
	if addr, err := netip.ParseAddr(ipCandidate); err == nil {
		addr = addr.Unmap()
		for _, ip := range leaf.IPAddresses {
			if candidate, ok := netip.AddrFromSlice(ip); ok && candidate.Unmap() == addr {
				return true
			}
		}
		return false
	}

	if strings.HasPrefix(identifier, "*.") {
		for _, name := range leaf.DNSNames {
			if strings.EqualFold(strings.TrimSuffix(name, "."), identifier) {
				return true
			}
		}
		return false
	}

	return leaf.VerifyHostname(identifier) == nil
}
