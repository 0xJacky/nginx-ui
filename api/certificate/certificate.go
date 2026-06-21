package certificate

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
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
	s := logger.NewSessionLogger(c)
	s.Info("GetCertList")
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

func normalizeCertKeyType(ctx *cosy.Ctx[model.Cert]) {
	payloadKeyType := cast.ToString(ctx.Payload["key_type"])
	if payloadKeyType != "" {
		ctx.Model.KeyType = helper.GetKeyType(certcrypto.KeyType(payloadKeyType))
	}

	sslCertificate := cast.ToString(ctx.Payload["ssl_certificate"])
	if sslCertificate == "" {
		return
	}

	keyType, err := cert.GetKeyType(sslCertificate)
	if err == nil && keyType != "" {
		ctx.Model.KeyType = helper.GetKeyType(certcrypto.KeyType(keyType))
	}
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
			normalizeCertKeyType(ctx)
		}).
		ExecutedHook(func(ctx *cosy.Ctx[model.Cert]) {
			sslCertificate := cast.ToString(ctx.Payload["ssl_certificate"])
			sslCertificateKey := cast.ToString(ctx.Payload["ssl_certificate_key"])
			if sslCertificate != "" && sslCertificateKey != "" {
				content := &cert.Content{
					SSLCertificatePath:    ctx.Model.SSLCertificatePath,
					SSLCertificateKeyPath: ctx.Model.SSLCertificateKeyPath,
					SSLCertificate:        sslCertificate,
					SSLCertificateKey:     sslCertificateKey,
				}
				err := content.WriteFile()
				if err != nil {
					ctx.AbortWithError(err)
					return
				}
			}
			persistCertificateFingerprint(&ctx.Model)
			err := cert.SyncToRemoteServer(&ctx.Model)
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
			normalizeCertKeyType(ctx)
		}).
		ExecutedHook(func(ctx *cosy.Ctx[model.Cert]) {
			sslCertificate := cast.ToString(ctx.Payload["ssl_certificate"])
			sslCertificateKey := cast.ToString(ctx.Payload["ssl_certificate_key"])

			content := &cert.Content{
				SSLCertificatePath:    ctx.Model.SSLCertificatePath,
				SSLCertificateKeyPath: ctx.Model.SSLCertificateKeyPath,
				SSLCertificate:        sslCertificate,
				SSLCertificateKey:     sslCertificateKey,
			}
			err := content.WriteFile()
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
			persistCertificateFingerprint(&ctx.Model)
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
	id := cast.ToUint64(c.Param("id"))
	certModel, err := query.Cert.FirstByID(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if err = query.Cert.DeleteByID(id); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	cleanupSelfSignedCertFiles(certModel)
}

func ImportExistingCert(c *gin.Context) {
	var json cert.ImportCertificateOptions

	if !cosy.BindAndValid(c, &json) {
		return
	}

	certModel, err := cert.ImportExistingCertificate(json)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	response := Transformer(certModel)
	if info, err := cert.ValidateCertificateAndKey(certModel.SSLCertificatePath, certModel.SSLCertificateKeyPath); err == nil {
		response.CertificateInfo = info
	}

	c.JSON(http.StatusOK, response)
}

func DiscoverExistingCert(c *gin.Context) {
	var json struct {
		Name string `json:"name"`
		Dir  string `json:"dir"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	pair, err := cert.DiscoverCertificatePair(json.Dir)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if json.Name != "" {
		pair.Name = json.Name
	}

	c.JSON(http.StatusOK, pair)
}

func DiscoverNewCerts(c *gin.Context) {
	var json struct {
		Patterns   []string `json:"patterns"`
		Configured bool     `json:"configured"`
		NewOnly    *bool    `json:"new_only"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	patterns := json.Patterns
	if json.Configured || len(patterns) == 0 {
		patterns = settings.CertSettings.DiscoveryPatterns
	}

	newOnly := true
	if json.NewOnly != nil {
		newOnly = *json.NewOnly
	}

	pairs, err := cert.ScanCertificateDiscoveryPatterns(patterns, newOnly)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"candidates": pairs,
	})
}

func persistCertificateFingerprint(certModel *model.Cert) {
	if certModel == nil || certModel.SSLCertificatePath == "" {
		return
	}

	fingerprint, err := cert.CertificateFingerprintFromPath(certModel.SSLCertificatePath)
	if err != nil {
		logger.Debug("certificate fingerprint unavailable", "path", certModel.SSLCertificatePath, "error", err)
		return
	}

	certModel.Fingerprint = fingerprint
	if certModel.ID == 0 {
		return
	}

	if err = model.UseDB().Model(certModel).Update("fingerprint", fingerprint).Error; err != nil {
		logger.Debug("persist certificate fingerprint failed", "id", certModel.ID, "error", err)
	}
}

func cleanupSelfSignedCertFiles(certModel *model.Cert) {
	if certModel.AutoCert != model.AutoCertSelfSigned {
		return
	}

	certPath := certModel.SSLCertificatePath
	keyPath := certModel.SSLCertificateKeyPath
	sslDir := nginx.GetConfPath("ssl")
	certDir := filepath.Dir(certPath)
	keyDir := filepath.Dir(keyPath)
	if certDir == "." || certDir != keyDir {
		return
	}
	if !helper.IsUnderDirectory(certPath, sslDir) || !helper.IsUnderDirectory(keyPath, sslDir) || !helper.IsUnderDirectory(certDir, sslDir) {
		return
	}
	if err := os.RemoveAll(certDir); err != nil {
		logger.Errorf("self-signed cert directory cleanup failed for id %d at %s: %v", certModel.ID, certDir, err)
	}
}

func SyncCertificate(c *gin.Context) {
	var json cert.SyncCertificatePayload

	if !cosy.BindAndValid(c, &json) {
		return
	}
	normalizedKeyType := helper.GetKeyType(json.KeyType)

	certModel := &model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		KeyType:               normalizedKeyType,
		AutoCert:              model.AutoCertSync,
	}

	db := model.UseDB()

	err := db.Where("name = ? AND ssl_certificate_path = ? AND ssl_certificate_key_path = ? AND key_type IN ?",
		json.Name, json.SSLCertificatePath, json.SSLCertificateKeyPath,
		helper.GetKeyTypeAliasStrings(normalizedKeyType)).
		Assign(&model.Cert{KeyType: normalizedKeyType}).
		FirstOrCreate(certModel).Error
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
	persistCertificateFingerprint(certModel)

	nginx.Reload()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
