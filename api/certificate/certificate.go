package certificate

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
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
	if certModel.SSLCertificatePath != "" &&
		helper.IsUnderDirectory(certModel.SSLCertificatePath, nginx.GetConfPath()) {
		if _, err := os.Stat(certModel.SSLCertificatePath); err == nil {
			sslCertificationBytes, _ = os.ReadFile(certModel.SSLCertificatePath)
			if !cert.IsCertificate(string(sslCertificationBytes)) {
				sslCertificationBytes = []byte{}
			}
		}

		certificateInfo, _ = cert.GetCertInfo(certModel.SSLCertificatePath)
	}

	if certModel.SSLCertificateKeyPath != "" &&
		helper.IsUnderDirectory(certModel.SSLCertificateKeyPath, nginx.GetConfPath()) {
		if _, err := os.Stat(certModel.SSLCertificateKeyPath); err == nil {
			sslCertificationKeyBytes, _ = os.ReadFile(certModel.SSLCertificateKeyPath)
			if !cert.IsPrivateKey(string(sslCertificationKeyBytes)) {
				sslCertificationKeyBytes = []byte{}
			}
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

	certModel, err := q.FirstByID(cast.ToUint64(c.Param("id")))

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

type certJson struct {
	Name                  string             `json:"name" binding:"required"`
	SSLCertificatePath    string             `json:"ssl_certificate_path" binding:"required,certificate_path"`
	SSLCertificateKeyPath string             `json:"ssl_certificate_key_path" binding:"required,privatekey_path"`
	SSLCertificate        string             `json:"ssl_certificate" binding:"omitempty,certificate"`
	SSLCertificateKey     string             `json:"ssl_certificate_key" binding:"omitempty,privatekey"`
	KeyType               certcrypto.KeyType `json:"key_type" binding:"omitempty,auto_cert_key_type"`
	ChallengeMethod       string             `json:"challenge_method"`
	DnsCredentialID       uint64             `json:"dns_credential_id"`
	ACMEUserID            uint64             `json:"acme_user_id"`
	SyncNodeIds           []uint64           `json:"sync_node_ids"`
}

func AddCert(c *gin.Context) {
	var json certJson

	if !api.BindAndValid(c, &json) {
		return
	}

	certModel := &model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		KeyType:               json.KeyType,
		ChallengeMethod:       json.ChallengeMethod,
		DnsCredentialID:       json.DnsCredentialID,
		ACMEUserID:            json.ACMEUserID,
		SyncNodeIds:           json.SyncNodeIds,
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

	err = cert.SyncToRemoteServer(certModel)
	if err != nil {
		notification.Error("Sync Certificate Error", err.Error())
		return
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

func ModifyCert(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var json certJson

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
		KeyType:               json.KeyType,
		DnsCredentialID:       json.DnsCredentialID,
		ACMEUserID:            json.ACMEUserID,
		SyncNodeIds:           json.SyncNodeIds,
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

	err = cert.SyncToRemoteServer(certModel)
	if err != nil {
		notification.Error("Sync Certificate Error", err.Error())
		return
	}

	GetCert(c)
}

func RemoveCert(c *gin.Context) {
	cosy.Core[model.Cert](c).Destroy()
}

func SyncCertificate(c *gin.Context) {
	var json cert.SyncCertificatePayload

	if !api.BindAndValid(c, &json) {
		return
	}

	certModel := &model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		KeyType:               json.KeyType,
		AutoCert:              model.AutoCertSync,
	}

	db := model.UseDB()

	err := db.Where(certModel).FirstOrCreate(certModel).Error
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

	nginx.Reload()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
