package certificate

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/site"
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
	SSLCertificate    string                           `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string                           `json:"ssl_certificate_key,omitempty"`
	CertificateInfo   *cert.Info                       `json:"certificate_info,omitempty"`
	DeploymentStatus  site.CertificateDeploymentStatus `json:"deployment_status"`
	UsedBy            []site.CertificateUsage          `json:"used_by,omitempty"`
}

func Transformer(certModel *model.Cert) (certificate *APICertificate) {
	var sslCertificationBytes, sslCertificationKeyBytes []byte
	var certificateInfo *cert.Info
	if certModel.SSLCertificatePath != "" &&
		helper.IsUnderDirectory(certModel.SSLCertificatePath, nginx.GetConfPath()) {
		if _, err := nginx.Stat(certModel.SSLCertificatePath); err == nil {
			sslCertificationBytes, _ = nginx.ReadFile(certModel.SSLCertificatePath)
			if !cert.IsCertificate(string(sslCertificationBytes)) {
				sslCertificationBytes = []byte{}
			}
		}

		certificateInfo, _ = cert.GetCertInfo(certModel.SSLCertificatePath)
	}

	if certModel.SSLCertificateKeyPath != "" &&
		helper.IsUnderDirectory(certModel.SSLCertificateKeyPath, nginx.GetConfPath()) {
		if _, err := nginx.Stat(certModel.SSLCertificateKeyPath); err == nil {
			sslCertificationKeyBytes, _ = nginx.ReadFile(certModel.SSLCertificateKeyPath)
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
		DeploymentStatus:  site.InspectCertificateDeployment(certModel),
	}
}

func GetCertList(c *gin.Context) {
	s := logger.NewSessionLogger(c)
	s.Info("GetCertList")
	usageIndex := site.BuildCertificateUsageIndex()
	cosy.Core[model.Cert](c).SetFussy("name", "domain").
		SetTransformer(func(m *model.Cert) any {
			info, _ := cert.GetCertInfo(m.SSLCertificatePath)
			return APICertificate{
				Cert:             m,
				CertificateInfo:  info,
				DeploymentStatus: site.InspectCertificateDeployment(m),
				UsedBy:           usageIndex.Lookup(m.SSLCertificatePath),
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

	certificate := Transformer(certModel)
	// The private key is a stored secret. Read-only service tokens keep the
	// certificate, its metadata and the key path, while api:write tokens keep
	// the key for export and administrators keep it for the editor.
	if internalmcp.IsReadOnlyServiceTokenRequest(c) && certificate.SSLCertificateKey != "" {
		certificate.SSLCertificateKey = settings.RedactedSensitiveValue
	}

	c.JSON(http.StatusOK, certificate)
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
			"name":                     "omitempty",
			"ssl_certificate_path":     "required,certificate_path",
			"ssl_certificate_key_path": "required,privatekey_path",
			"ssl_certificate":          "omitempty,certificate",
			"ssl_certificate_key":      "omitempty,privatekey",
			"key_type":                 "omitempty,auto_cert_key_type",
			"challenge_method":         "omitempty,oneof=http01 dns01",
			"profile":                  "omitempty",
			"dns_credential_id":        "omitempty",
			"acme_user_id":             "omitempty",
			"sync_node_ids":            "omitempty",
			"must_staple":              "omitempty",
			"enable_common_name":       "omitempty",
			"revoke_old":               "omitempty",
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
			"name":                     "omitempty",
			"ssl_certificate_path":     "required,certificate_path",
			"ssl_certificate_key_path": "required,privatekey_path",
			"ssl_certificate":          "omitempty,certificate",
			"ssl_certificate_key":      "omitempty,privatekey",
			"key_type":                 "omitempty,auto_cert_key_type",
			"challenge_method":         "omitempty,oneof=http01 dns01",
			"profile":                  "omitempty",
			"dns_credential_id":        "omitempty",
			"acme_user_id":             "omitempty",
			"sync_node_ids":            "omitempty",
			"must_staple":              "omitempty",
			"enable_common_name":       "omitempty",
			"revoke_old":               "omitempty",
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

func DiscoverNewCerts(c *gin.Context) {
	var json struct {
		NewOnly *bool `json:"new_only"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	newOnly := true
	if json.NewOnly != nil {
		newOnly = *json.NewOnly
	}

	pairs, err := cert.ScanCertificateSSLDirectory(newOnly)
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
	if err := nginx.RemoveAll(certDir); err != nil {
		logger.Errorf("self-signed cert directory cleanup failed for id %d at %s: %v", certModel.ID, certDir, err)
	}
}

func SyncCertificate(c *gin.Context) {
	var json cert.SyncCertificatePayload
	if !cosy.BindAndValid(c, &json) {
		return
	}
	content := &cert.Content{
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		SSLCertificate:        json.SSLCertificate,
		SSLCertificateKey:     json.SSLCertificateKey,
	}
	// Serialize target validation and replacement with ordinary config saves.
	release := config.LockApply()
	defer release()
	if err := content.ValidateSync(); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	normalizedKeyType := helper.GetKeyType(json.KeyType)
	certModel := &model.Cert{
		Name: json.Name, SSLCertificatePath: json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
		KeyType:               normalizedKeyType, AutoCert: model.AutoCertSync,
	}
	db := model.UseDB()
	tx := db.Begin()
	if tx.Error != nil {
		cosy.ErrHandler(c, tx.Error)
		return
	}
	defer tx.Rollback()
	err := tx.Where("name = ? AND ssl_certificate_path = ? AND ssl_certificate_key_path = ? AND key_type IN ?",
		json.Name, json.SSLCertificatePath, json.SSLCertificateKeyPath,
		helper.GetKeyTypeAliasStrings(normalizedKeyType)).
		Assign(&model.Cert{KeyType: normalizedKeyType}).FirstOrCreate(certModel).Error
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	// Every save of a site replicates its certificates. Unchanged pairs retain
	// the existing no-write/no-reload behavior, after validating the input.
	if content.MatchesFiles() {
		if err = tx.Commit().Error; err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
		return
	}
	snapshots := make(map[string]config.FileSnapshot, 2)
	for _, path := range []string{content.SSLCertificatePath, content.SSLCertificateKeyPath} {
		snapshot, captureErr := config.CaptureFile(path)
		if captureErr != nil {
			cosy.ErrHandler(c, captureErr)
			return
		}
		snapshots[path] = snapshot
	}
	rollback := func() error {
		var failures []error
		for path, snapshot := range snapshots {
			if restoreErr := snapshot.Restore(path); restoreErr != nil {
				failures = append(failures, restoreErr)
			}
		}
		return errors.Join(failures...)
	}
	// Commit metadata only for accepted file state. Fingerprint persistence
	// uses the shared database and must run after the transaction releases locks.
	finish := func(reloadPending, reloaded bool) {
		if commitErr := tx.Commit().Error; commitErr != nil {
			cosy.ErrHandler(c, config.RollbackError(commitErr, func() error {
				if restoreErr := rollback(); restoreErr != nil {
					return restoreErr
				}
				if reloaded {
					if restored := nginx.Control(nginx.TestConfig); restored.IsError() {
						return restored.GetError()
					}
					if restored := nginx.Control(nginx.Reload); restored.IsError() {
						return restored.GetError()
					}
				}
				return nil
			}))
			return
		}
		persistCertificateFingerprint(certModel)
		result := gin.H{"message": "ok"}
		if reloadPending {
			result["reload_pending"] = true
		}
		c.JSON(http.StatusOK, result)
	}
	wasHealthy := !nginx.Control(nginx.TestConfig).IsError()
	if err = content.WriteFile(); err != nil {
		cosy.ErrHandler(c, config.RollbackError(err, rollback))
		return
	}
	// Certificates are also staged ahead of site/config deployments. An
	// already broken configuration may reference several missing pairs; rolling
	// back each valid pair would prevent that node from ever recovering.
	// Previously healthy configurations still get transactional rollback.
	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		if wasHealthy {
			cosy.ErrHandler(c, config.RollbackError(result.GetError(), rollback))
			return
		}
		logger.Warnf("Certificate sync stored a valid pair; nginx reload deferred until configuration is healthy: %v", result.GetError())
		finish(true, false)
		return
	}
	if result := nginx.Control(nginx.Reload); result.IsError() {
		if !wasHealthy {
			logger.Warnf("Certificate sync stored a valid pair; nginx reload is pending: %v", result.GetError())
			finish(true, false)
			return
		}
		cosy.ErrHandler(c, config.RollbackError(result.GetError(), func() error {
			if restoreErr := rollback(); restoreErr != nil {
				return restoreErr
			}
			if restored := nginx.Control(nginx.TestConfig); restored.IsError() {
				return restored.GetError()
			}
			if restored := nginx.Control(nginx.Reload); restored.IsError() {
				return restored.GetError()
			}
			return nil
		}))
		return
	}
	finish(false, true)
}
