package certificate

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/api/cosy"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"net/http"
	"os"
)

type APICertificate struct {
	*model.Cert
	SSLCertificate    string     `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string     `json:"ssl_certificate_key,omitempty"`
	CertificateInfo   *cert.Info `json:"certificate_info,omitempty"`
}

func Transformer(certModel *model.Cert) (certificate *APICertificate) {
	var sslCertificationBytes, sslCertificationKeyBytes []byte
	var certificateInfo *cert.Info
	if certModel.SSLCertificatePath != "" {
		if _, err := os.Stat(certModel.SSLCertificatePath); err == nil {
			sslCertificationBytes, _ = os.ReadFile(certModel.SSLCertificatePath)
		}

		certificateInfo, _ = cert.GetCertInfo(certModel.SSLCertificatePath)
	}

	if certModel.SSLCertificateKeyPath != "" {
		if _, err := os.Stat(certModel.SSLCertificateKeyPath); err == nil {
			sslCertificationKeyBytes, _ = os.ReadFile(certModel.SSLCertificateKeyPath)
		}
	}

	return &APICertificate{
		Cert:              certModel,
		SSLCertificate:    string(sslCertificationBytes),
		SSLCertificateKey: string(sslCertificationKeyBytes),
		CertificateInfo:   certificateInfo,
	}
}

func GetCertList(c *gin.Context) {
	cosy.Core[model.Cert](c).SetFussy("name", "domain").SetTransformer(func(m *model.Cert) any {

		info, _ := cert.GetCertInfo(m.SSLCertificatePath)

		return APICertificate{
			Cert:            m,
			CertificateInfo: info,
		}
	}).PagingList()
}

func GetCert(c *gin.Context) {
	q := query.Cert

	certModel, err := q.FirstByID(cast.ToInt(c.Param("id")))

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

func AddCert(c *gin.Context) {
	var json struct {
		Name                  string `json:"name"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
		SSLCertificate        string `json:"ssl_certificate"`
		SSLCertificateKey     string `json:"ssl_certificate_key"`
		ChallengeMethod       string `json:"challenge_method"`
		DnsCredentialID       int    `json:"dns_credential_id"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}
	certModel := &model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		ChallengeMethod:       json.ChallengeMethod,
		DnsCredentialID:       json.DnsCredentialID,
	}

	err := certModel.Insert()

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	content := &cert.Content{
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		SSLCertificate:        json.SSLCertificate,
		SSLCertificateKey:     json.SSLCertificateKey,
	}

	err = content.WriteFile()

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

func ModifyCert(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	var json struct {
		Name                  string `json:"name"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
		SSLCertificate        string `json:"ssl_certificate"`
		SSLCertificateKey     string `json:"ssl_certificate_key"`
		ChallengeMethod       string `json:"challenge_method"`
		DnsCredentialID       int    `json:"dns_credential_id"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	q := query.Cert

	certModel, err := q.FirstByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		ChallengeMethod:       json.ChallengeMethod,
		DnsCredentialID:       json.DnsCredentialID,
	})

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	content := &cert.Content{
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		SSLCertificate:        json.SSLCertificate,
		SSLCertificateKey:     json.SSLCertificateKey,
	}

	err = content.WriteFile()

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	GetCert(c)
}

func RemoveCert(c *gin.Context) {
	cosy.Core[model.Cert](c).Destroy()
}
