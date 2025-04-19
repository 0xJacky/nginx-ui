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
	cosy.Core[model.Cert](c).SetFussy("name", "domain").
		SetTransformer(func(m *model.Cert) any {
			info, _ := cert.GetCertInfo(m.SSLCertificatePath)
			return APICertificate{
				Cert:            m,
				CertificateInfo: info,
			}
		}).PagingList()
}

func GetCert(c *gin.Context) {
	q := query.Cert

	id := cast.ToUint64(c.Param("id"))
	if contextId, ok := c.Get("id"); ok {
		id = cast.ToUint64(contextId)
	}

	certModel, err := q.FirstByID(id)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

func AddCert(c *gin.Context) {
	cosy.Core[model.Cert](c).
		SetValidRules(gin.H{
			"name":                       "omitempty",
			"ssl_certificate_path":       "required,certificate_path",
			"ssl_certificate_key_path":   "required,privatekey_path",
			"ssl_certificate":            "omitempty,certificate",
			"ssl_certificate_key":        "omitempty,privatekey",
			"key_type":                   "omitempty,auto_cert_key_type",
			"challenge_method":           "omitempty,oneof=http01 dns01",
			"dns_credential_id":          "omitempty",
			"acme_user_id":               "omitempty",
			"sync_node_ids":              "omitempty",
			"must_staple":                "omitempty",
			"lego_disable_cname_support": "omitempty",
			"revoke_old":                 "omitempty",
		}).
		BeforeExecuteHook(func(ctx *cosy.Ctx[model.Cert]) {
			sslCertificate := ctx.Payload["ssl_certificate"].(string)
			// Detect and set certificate type
			if sslCertificate != "" {
				keyType, err := cert.GetKeyType(sslCertificate)
				if err == nil && keyType != "" {
					// Set KeyType based on certificate type
					switch keyType {
					case "2048":
						ctx.Model.KeyType = certcrypto.RSA2048
					case "3072":
						ctx.Model.KeyType = certcrypto.RSA3072
					case "4096":
						ctx.Model.KeyType = certcrypto.RSA4096
					case "P256":
						ctx.Model.KeyType = certcrypto.EC256
					case "P384":
						ctx.Model.KeyType = certcrypto.EC384
					}
				}
			}
		}).
		ExecutedHook(func(ctx *cosy.Ctx[model.Cert]) {
			content := &cert.Content{
				SSLCertificatePath:    ctx.Model.SSLCertificatePath,
				SSLCertificateKeyPath: ctx.Model.SSLCertificateKeyPath,
				SSLCertificate:        ctx.Payload["ssl_certificate"].(string),
				SSLCertificateKey:     ctx.Payload["ssl_certificate_key"].(string),
			}
			err := content.WriteFile()
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
			err = cert.SyncToRemoteServer(&ctx.Model)
			if err != nil {
				notification.Error("Sync Certificate Error", err.Error(), nil)
				return
			}
			ctx.Context.Set("id", ctx.Model.ID)
		}).
		SetNextHandler(GetCert).
		Create()
}

func ModifyCert(c *gin.Context) {
	cosy.Core[model.Cert](c).
		SetValidRules(gin.H{
			"name":                       "omitempty",
			"ssl_certificate_path":       "required,certificate_path",
			"ssl_certificate_key_path":   "required,privatekey_path",
			"ssl_certificate":            "omitempty,certificate",
			"ssl_certificate_key":        "omitempty,privatekey",
			"key_type":                   "omitempty,auto_cert_key_type",
			"challenge_method":           "omitempty,oneof=http01 dns01",
			"dns_credential_id":          "omitempty",
			"acme_user_id":               "omitempty",
			"sync_node_ids":              "omitempty",
			"must_staple":                "omitempty",
			"lego_disable_cname_support": "omitempty",
			"revoke_old":                 "omitempty",
		}).
		BeforeExecuteHook(func(ctx *cosy.Ctx[model.Cert]) {
			sslCertificate := ctx.Payload["ssl_certificate"].(string)
			// Detect and set certificate type
			if sslCertificate != "" {
				keyType, err := cert.GetKeyType(sslCertificate)
				if err == nil && keyType != "" {
					// Set KeyType based on certificate type
					switch keyType {
					case "2048":
						ctx.Model.KeyType = certcrypto.RSA2048
					case "3072":
						ctx.Model.KeyType = certcrypto.RSA3072
					case "4096":
						ctx.Model.KeyType = certcrypto.RSA4096
					case "P256":
						ctx.Model.KeyType = certcrypto.EC256
					case "P384":
						ctx.Model.KeyType = certcrypto.EC384
					}
				}
			}
		}).
		ExecutedHook(func(ctx *cosy.Ctx[model.Cert]) {
			content := &cert.Content{
				SSLCertificatePath:    ctx.Model.SSLCertificatePath,
				SSLCertificateKeyPath: ctx.Model.SSLCertificateKeyPath,
				SSLCertificate:        ctx.Payload["ssl_certificate"].(string),
				SSLCertificateKey:     ctx.Payload["ssl_certificate_key"].(string),
			}
			err := content.WriteFile()
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
			err = cert.SyncToRemoteServer(&ctx.Model)
			if err != nil {
				notification.Error("Sync Certificate Error", err.Error(), nil)
				return
			}

		}).
		SetNextHandler(GetCert).
		Modify()
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
