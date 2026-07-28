package cert

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

type Info struct {
	SubjectName string    `json:"subject_name"`
	IssuerName  string    `json:"issuer_name"`
	NotAfter    time.Time `json:"not_after"`
	NotBefore   time.Time `json:"not_before"`
}

func GetCertInfo(sslCertificatePath string) (info *Info, err error) {
	cert, err := getCertificate(sslCertificatePath)
	if err != nil {
		return nil, err
	}

	return certificateInfo(cert), nil
}

func getCertificate(sslCertificatePath string) (*x509.Certificate, error) {
	if !helper.IsUnderDirectory(sslCertificatePath, nginx.GetConfPath()) {
		return nil, ErrCertPathIsNotUnderTheNginxConfDir
	}

	certData, err := os.ReadFile(sslCertificatePath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, ErrCertDecode
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, ErrCertParse
	}
	return cert, nil
}

func certificateInfo(cert *x509.Certificate) *Info {
	if cert == nil {
		return nil
	}

	return &Info{
		SubjectName: certificateSubjectName(cert),
		IssuerName:  cert.Issuer.CommonName,
		NotAfter:    cert.NotAfter,
		NotBefore:   cert.NotBefore,
	}
}

func certificateSubjectName(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}
	for _, name := range cert.DNSNames {
		if name != "" {
			return name
		}
	}
	for _, ip := range cert.IPAddresses {
		if ip != nil {
			return ip.String()
		}
	}
	return ""
}
