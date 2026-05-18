package cert

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
)

const (
	// SelfSignedDefaultValidityDays is used when no validity period is given.
	SelfSignedDefaultValidityDays = 365
	// SelfSignedMaxValidityDays caps the validity period.
	SelfSignedMaxValidityDays = 3650
	// selfSignedClockSkewBackdate backdates NotBefore to tolerate clock skew.
	selfSignedClockSkewBackdate = 5 * time.Minute
)

// SelfSignedOptions describes the parameters for generating a self-signed
// leaf certificate.
type SelfSignedOptions struct {
	CommonName   string
	DNSNames     []string
	IPAddresses  []string
	KeyType      certcrypto.KeyType
	ValidityDays int
}

// GenerateSelfSigned builds a self-signed leaf certificate and returns the
// PEM-encoded certificate and private key.
func GenerateSelfSigned(opts SelfSignedOptions) (certPEM, keyPEM []byte, err error) {
	signer, err := certcrypto.GeneratePrivateKey(helper.GetKeyType(opts.KeyType))
	if err != nil {
		return nil, nil, cosy.WrapErrorWithParams(ErrSelfSignedGenerateKey, err.Error())
	}
	return signSelfSigned(opts, signer)
}

// signSelfSigned creates a self-signed certificate from the given options and
// signer, returning the PEM-encoded certificate and private key.
func signSelfSigned(opts SelfSignedOptions, signer crypto.Signer) (certPEM, keyPEM []byte, err error) {
	ipAddresses := make([]net.IP, 0, len(opts.IPAddresses))
	for _, raw := range opts.IPAddresses {
		ip := net.ParseIP(raw)
		if ip == nil {
			return nil, nil, cosy.WrapErrorWithParams(ErrSelfSignedInvalidIP, raw)
		}
		ipAddresses = append(ipAddresses, ip)
	}

	if len(opts.DNSNames) == 0 && len(ipAddresses) == 0 {
		return nil, nil, ErrSelfSignedNoSAN
	}

	validityDays := opts.ValidityDays
	if validityDays <= 0 {
		validityDays = SelfSignedDefaultValidityDays
	}
	if validityDays > SelfSignedMaxValidityDays {
		validityDays = SelfSignedMaxValidityDays
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, cosy.WrapErrorWithParams(ErrSelfSignedCreateCert, err.Error())
	}

	now := time.Now()
	// ECDSA keys do not perform key encipherment; only RSA keys get that bit.
	keyUsage := x509.KeyUsageDigitalSignature
	if _, isRSA := signer.Public().(*rsa.PublicKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: opts.CommonName},
		DNSNames:              opts.DNSNames,
		IPAddresses:           ipAddresses,
		NotBefore:             now.Add(-selfSignedClockSkewBackdate),
		NotAfter:              now.AddDate(0, 0, validityDays),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, signer.Public(), signer)
	if err != nil {
		return nil, nil, cosy.WrapErrorWithParams(ErrSelfSignedCreateCert, err.Error())
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(signer)
	if err != nil {
		return nil, nil, cosy.WrapErrorWithParams(ErrSelfSignedCreateCert, err.Error())
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}
