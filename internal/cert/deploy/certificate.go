package deploy

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

// LoadCertificate reads the certificate a push carries from the files nginx
// serves: the leaf and its chain from the certificate file, the key from the
// key file. It is read afresh for every push, so a retry never sends
// material that was replaced in the meantime.
func LoadCertificate(certModel *model.Cert) (Certificate, error) {
	if certModel.SSLCertificatePath == "" || certModel.SSLCertificateKeyPath == "" {
		return Certificate{}, cert.ErrCertPathIsEmpty
	}

	chainFile, err := os.ReadFile(certModel.SSLCertificatePath)
	if err != nil {
		return Certificate{}, cosy.WrapErrorWithParams(cert.ErrReadCertificate, certModel.SSLCertificatePath, err.Error())
	}
	leafPEM, chainPEM, leaf, err := splitChain(chainFile)
	if err != nil {
		return Certificate{}, cosy.WrapErrorWithParams(cert.ErrInvalidCertificate, certModel.SSLCertificatePath, err.Error())
	}

	keyFile, err := os.ReadFile(certModel.SSLCertificateKeyPath)
	if err != nil {
		return Certificate{}, cosy.WrapErrorWithParams(cert.ErrReadPrivateKey, certModel.SSLCertificateKeyPath, err.Error())
	}
	keyPEM, ok := privateKeyBlock(keyFile)
	if !ok {
		return Certificate{}, cosy.WrapErrorWithParams(cert.ErrInvalidPrivateKey, certModel.SSLCertificateKeyPath)
	}

	name := certModel.Name
	if name == "" {
		name = certModel.Filename
	}
	domains := append([]string(nil), certModel.Domains...)
	if len(domains) == 0 {
		domains = leafIdentifiers(leaf)
	}
	if name == "" && len(domains) > 0 {
		name = domains[0]
	}

	return Certificate{
		Name:           name,
		Domains:        domains,
		CertificatePEM: leafPEM,
		PrivateKeyPEM:  keyPEM,
		ChainPEM:       chainPEM,
		NotAfter:       leaf.NotAfter,
	}, nil
}

// errNoCertificateBlock reports a certificate file without a certificate.
var errNoCertificateBlock = errors.New("no PEM encoded certificate found")

// splitChain splits a certificate file into the leaf, the rest of the chain
// and the parsed leaf. The leaf is the first certificate, as nginx expects.
func splitChain(data []byte) (leafPEM, chainPEM string, leaf *x509.Certificate, err error) {
	var chain bytes.Buffer
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		encoded := pem.EncodeToMemory(&pem.Block{Type: block.Type, Bytes: block.Bytes})
		if leaf == nil {
			if leaf, err = x509.ParseCertificate(block.Bytes); err != nil {
				return "", "", nil, err
			}
			leafPEM = string(encoded)
			continue
		}
		chain.Write(encoded)
	}
	if leaf == nil {
		return "", "", nil, errNoCertificateBlock
	}
	return leafPEM, chain.String(), leaf, nil
}

// privateKeyBlock returns the first private key block of a key file, as PEM.
func privateKeyBlock(data []byte) (string, bool) {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return "", false
		}
		if strings.HasSuffix(block.Type, "PRIVATE KEY") {
			return string(pem.EncodeToMemory(block)), true
		}
	}
}

// leafIdentifiers lists the names and addresses a certificate covers.
func leafIdentifiers(leaf *x509.Certificate) []string {
	identifiers := append([]string(nil), leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		identifiers = append(identifiers, ip.String())
	}
	if len(identifiers) == 0 && leaf.Subject.CommonName != "" {
		identifiers = append(identifiers, leaf.Subject.CommonName)
	}
	return identifiers
}
