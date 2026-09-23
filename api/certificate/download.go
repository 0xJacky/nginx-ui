package certificate

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

type downloadCertRequest struct {
	Format      string `json:"format" binding:"required,oneof=crt key pfx"`
	PFXPassword string `json:"pfx_password"`
}

func DownloadCert(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))
	certModel, err := query.Cert.FirstByID(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	var req downloadCertRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	certPEM, keyPEM, err := readCertificatePair(certModel.SSLCertificatePath, certModel.SSLCertificateKeyPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	filename := downloadCertFileName(certModel.Name)

	switch req.Format {
	case "crt":
		c.Header("Content-Type", "application/x-x509-ca-cert")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.crt\"", filename))
		c.Data(http.StatusOK, "application/x-x509-ca-cert", certPEM)
	case "key":
		c.Header("Content-Type", "application/x-pem-file")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.key\"", filename))
		c.Data(http.StatusOK, "application/x-pem-file", keyPEM)
	case "pfx":
		pfxBytes, pfxErr := encodePFX(certPEM, keyPEM, req.PFXPassword)
		if pfxErr != nil {
			cosy.ErrHandler(c, pfxErr)
			return
		}
		c.Header("Content-Type", "application/x-pkcs12")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.pfx\"", filename))
		c.Data(http.StatusOK, "application/x-pkcs12", pfxBytes)
	default:
		cosy.ErrHandler(c, fmt.Errorf("unsupported download format"))
	}
}

func readCertificatePair(certPath, keyPath string) ([]byte, []byte, error) {
	if certPath == "" || keyPath == "" {
		return nil, nil, fmt.Errorf("certificate path and key path cannot be empty")
	}

	confRoot := nginx.GetConfPath()
	if !helper.IsUnderDirectory(certPath, confRoot) || !helper.IsUnderDirectory(keyPath, confRoot) {
		return nil, nil, fmt.Errorf("certificate path is out of allowed directory")
	}

	certPEM, err := nginx.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	if !cert.IsCertificate(string(certPEM)) {
		return nil, nil, fmt.Errorf("invalid certificate format")
	}

	keyPEM, err := nginx.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	if !cert.IsPrivateKey(string(keyPEM)) {
		return nil, nil, fmt.Errorf("invalid private key format")
	}

	return certPEM, keyPEM, nil
}

func encodePFX(certPEM, keyPEM []byte, password string) ([]byte, error) {
	leaf, chain, err := parseCertificateChain(certPEM)
	if err != nil {
		return nil, err
	}

	privateKey, err := certcrypto.ParsePEMPrivateKey(keyPEM)
	if err != nil {
		return nil, err
	}

	return pkcs12.Encode(rand.Reader, privateKey, leaf, chain, password)
}

func parseCertificateChain(certPEM []byte) (*x509.Certificate, []*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := certPEM
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		parsed, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		certs = append(certs, parsed)
	}

	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("certificate is empty")
	}

	leaf := certs[0]
	chain := make([]*x509.Certificate, 0)
	if len(certs) > 1 {
		chain = certs[1:]
	}

	return leaf, chain, nil
}

func downloadCertFileName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "certificate"
	}

	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(trimmed)
}
