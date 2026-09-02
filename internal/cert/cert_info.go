package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

func CertificateCoversNames(sslCertificatePath string, names []string) error {
	parsedCertificate, err := getCertificate(sslCertificatePath)
	if err != nil {
		return err
	}

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if name == "_" || name == "localhost" || strings.Contains(name, "$") ||
			strings.HasPrefix(name, "~") || strings.HasPrefix(name, ".") {
			return fmt.Errorf("server name %q cannot be matched safely", name)
		}
		if strings.HasPrefix(name, "*.") {
			matched := false
			for _, dnsName := range parsedCertificate.DNSNames {
				if strings.EqualFold(dnsName, name) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("certificate does not cover server name %q", name)
			}
			continue
		}
		if err = parsedCertificate.VerifyHostname(name); err != nil {
			return fmt.Errorf("certificate does not cover server name %q: %w", name, err)
		}
	}

	return nil
}

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
