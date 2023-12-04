package cert

import (
	"crypto/x509"
	"encoding/pem"
	"github.com/pkg/errors"
	"os"
	"time"
)

type Info struct {
	SubjectName string    `json:"subject_name"`
	IssuerName  string    `json:"issuer_name"`
	NotAfter    time.Time `json:"not_after"`
	NotBefore   time.Time `json:"not_before"`
}

func GetCertInfo(sslCertificatePath string) (info *Info, err error) {
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

	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		err = errors.Wrap(err, "certificate parsing error")
		return
	}

	info = &Info{
		SubjectName: cert.Subject.CommonName,
		IssuerName:  cert.Issuer.CommonName,
		NotAfter:    cert.NotAfter,
		NotBefore:   cert.NotBefore,
	}

	return
}
