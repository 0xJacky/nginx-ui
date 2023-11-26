package cert

import (
	"crypto/x509"
	"encoding/pem"
	"github.com/pkg/errors"
	"os"
)

func GetCertInfo(sslCertificatePath string) (cert *x509.Certificate, err error) {
	certData, err := os.ReadFile(sslCertificatePath)

	if err != nil {
		err = errors.Wrap(err, "error read certificate")
		return
	}

	block, _ := pem.Decode(certData)

	if block == nil || block.Type != "CERTIFICATE" {
		err = errors.New("certificate decoding error")
		return
	}

	cert, err = x509.ParseCertificate(block.Bytes)

	if err != nil {
		err = errors.Wrap(err, "certificate parsing error")
		return
	}

	return
}
