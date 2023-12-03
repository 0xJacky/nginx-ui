package certificate

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/api/cosy"
	"github.com/0xJacky/Nginx-UI/api/sites"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"net/http"
	"os"
	"path/filepath"
)

func GetCertList(c *gin.Context) {
	cosy.Core[model.Cert](c).SetFussy("name", "domain").PagingList()
}

func getCert(c *gin.Context, certModel *model.Cert) {
	type resp struct {
		*model.Cert
		SSLCertificate    string                 `json:"ssl_certificate"`
		SSLCertificateKey string                 `json:"ssl_certificate_key"`
		CertificateInfo   *sites.CertificateInfo `json:"certificate_info,omitempty"`
	}

	var sslCertificationBytes, sslCertificationKeyBytes []byte
	var certificateInfo *sites.CertificateInfo
	if certModel.SSLCertificatePath != "" {
		if _, err := os.Stat(certModel.SSLCertificatePath); err == nil {
			sslCertificationBytes, _ = os.ReadFile(certModel.SSLCertificatePath)
		}

		pubKey, err := cert.GetCertInfo(certModel.SSLCertificatePath)

		if err != nil {
			api.ErrHandler(c, err)
			return
		}

		certificateInfo = &sites.CertificateInfo{
			SubjectName: pubKey.Subject.CommonName,
			IssuerName:  pubKey.Issuer.CommonName,
			NotAfter:    pubKey.NotAfter,
			NotBefore:   pubKey.NotBefore,
		}
	}

	if certModel.SSLCertificateKeyPath != "" {
		if _, err := os.Stat(certModel.SSLCertificateKeyPath); err == nil {
			sslCertificationKeyBytes, _ = os.ReadFile(certModel.SSLCertificateKeyPath)
		}
	}

	c.JSON(http.StatusOK, resp{
		certModel,
		string(sslCertificationBytes),
		string(sslCertificationKeyBytes),
		certificateInfo,
	})
}

func GetCert(c *gin.Context) {
	certModel, err := model.FirstCertByID(cast.ToInt(c.Param("id")))

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	getCert(c, &certModel)
}

func AddCert(c *gin.Context) {
	var json struct {
		Name                  string `json:"name"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
		SSLCertification      string `json:"ssl_certification"`
		SSLCertificationKey   string `json:"ssl_certification_key"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}
	certModel := &model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
	}

	err := certModel.Insert()

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificatePath), 0644)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificateKeyPath), 0644)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if json.SSLCertification != "" {
		err = os.WriteFile(json.SSLCertificatePath, []byte(json.SSLCertification), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	if json.SSLCertificationKey != "" {
		err = os.WriteFile(json.SSLCertificateKeyPath, []byte(json.SSLCertificationKey), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	getCert(c, certModel)
}

func ModifyCert(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	var json struct {
		Name                  string `json:"name"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
		SSLCertificate        string `json:"ssl_certificate"`
		SSLCertificateKey     string `json:"ssl_certificate_key"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	certModel, err := model.FirstCertByID(id)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
	})

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificatePath), 0644)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificateKeyPath), 0644)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if json.SSLCertificate != "" {
		err = os.WriteFile(json.SSLCertificatePath, []byte(json.SSLCertificate), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	if json.SSLCertificateKeyPath != "" {
		err = os.WriteFile(json.SSLCertificateKeyPath, []byte(json.SSLCertificateKey), 0644)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	GetCert(c)
}

func RemoveCert(c *gin.Context) {
	cosy.Core[model.Cert](c).Destroy()
}
