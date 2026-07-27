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
	if !helper.IsUnderDirectory(sslCertificatePath, nginx.GetConfPath()) {
		err = ErrCertPathIsNotUnderTheNginxConfDir
		return
	}

	certData, err := os.ReadFile(sslCertificatePath)
	if err != nil {
		return
	}

	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		err = ErrCertDecode
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		err = ErrCertParse
		return
	}

	info = &Info{
		SubjectName: certificateSubjectName(cert),
		IssuerName:  cert.Issuer.CommonName,
		NotAfter:    cert.NotAfter,
		NotBefore:   cert.NotBefore,
	}
	return
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
