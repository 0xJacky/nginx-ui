package certificate

import (
	"net/http"
	"os"

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
		cosy.ErrHandler(c, err)
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
	RevokeOld             bool               `json:"revoke_old"`
}

func AddCert(c *gin.Context) {
	var json certJson

	if !cosy.BindAndValid(c, &json) {
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
		cosy.ErrHandler(c, err)
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
		cosy.ErrHandler(c, err)
		return
	}

	// Detect and set certificate type
	if len(json.SSLCertificate) > 0 {
		keyType, err := cert.GetKeyType(json.SSLCertificate)
		if err == nil && keyType != "" {
			// Set KeyType based on certificate type
			switch keyType {
			case "2048":
				certModel.KeyType = certcrypto.RSA2048
			case "3072":
				certModel.KeyType = certcrypto.RSA3072
			case "4096":
				certModel.KeyType = certcrypto.RSA4096
			case "P256":
				certModel.KeyType = certcrypto.EC256
			case "P384":
				certModel.KeyType = certcrypto.EC384
			}
			// Update certificate model
			err = certModel.Updates(&model.Cert{KeyType: certModel.KeyType})
			if err != nil {
				notification.Error("Update Certificate Type Error", err.Error(), nil)
			}
		}
	}

	err = cert.SyncToRemoteServer(certModel)
	if err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
		return
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

func ModifyCert(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var json certJson

	if !cosy.BindAndValid(c, &json) {
		return
	}

	q := query.Cert

	certModel, err := q.FirstByID(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Create update data object
	updateData := &model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		ChallengeMethod:       json.ChallengeMethod,
		KeyType:               json.KeyType,
		DnsCredentialID:       json.DnsCredentialID,
		ACMEUserID:            json.ACMEUserID,
		SyncNodeIds:           json.SyncNodeIds,
		RevokeOld:             json.RevokeOld,
	}

	content := &cert.Content{
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		SSLCertificate:        json.SSLCertificate,
		SSLCertificateKey:     json.SSLCertificateKey,
	}

	err = content.WriteFile()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Detect and set certificate type
	if len(json.SSLCertificate) > 0 {
		keyType, err := cert.GetKeyType(json.SSLCertificate)
		if err == nil && keyType != "" {
			// Set KeyType based on certificate type
			switch keyType {
			case "2048":
				updateData.KeyType = certcrypto.RSA2048
			case "3072":
				updateData.KeyType = certcrypto.RSA3072
			case "4096":
				updateData.KeyType = certcrypto.RSA4096
			case "P256":
				updateData.KeyType = certcrypto.EC256
			case "P384":
				updateData.KeyType = certcrypto.EC384
			}
		}
	}

	err = certModel.Updates(updateData)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	err = cert.SyncToRemoteServer(certModel)
	if err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
		return
	}

	GetCert(c)
}

func RemoveCert(c *gin.Context) {
	cosy.Core[model.Cert](c).Destroy()
}

func SyncCertificate(c *gin.Context) {
	var json cert.SyncCertificatePayload

	if !cosy.BindAndValid(c, &json) {
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
		cosy.ErrHandler(c, err)
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
		cosy.ErrHandler(c, err)
		return
	}

	nginx.Reload()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
