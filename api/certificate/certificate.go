package certificate

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/gorm"
)

type APICertificate struct {
	*model.Cert
	SSLCertificate    string                           `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string                           `json:"ssl_certificate_key,omitempty"`
	CertificateInfo   *cert.Info                       `json:"certificate_info,omitempty"`
	DeploymentStatus  site.CertificateDeploymentStatus `json:"deployment_status"`
	UsedBy            []site.CertificateUsage          `json:"used_by,omitempty"`
	cert.Overview
	// DNSProvider is the provider name of the DNS credential used for DNS-01.
	DNSProvider string `json:"dns_provider,omitempty"`
	// DelegatedNodeName names the node a delegated certificate was issued for.
	DelegatedNodeName string `json:"delegated_node_name,omitempty"`
}

// delegatedNodeNames maps the ids of the nodes certificates were issued for
// to their names.
func delegatedNodeNames(models []*model.Cert) map[uint64]string {
	ids := make([]uint64, 0)
	for _, m := range models {
		if m.IsDelegated() {
			ids = append(ids, m.DelegatedNodeID)
		}
	}
	names := map[uint64]string{}
	if len(ids) == 0 {
		return names
	}
	nodes, err := query.Node.Where(query.Node.ID.In(ids...)).Find()
	if err != nil {
		return names
	}
	for _, node := range nodes {
		names[node.ID] = node.Name
	}
	return names
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
		Overview:          buildOverview(certModel, certificateInfo),
		DNSProvider:       dnsProviderNames([]*model.Cert{certModel})[certModel.DnsCredentialID],
		DelegatedNodeName: delegatedNodeNames([]*model.Cert{certModel})[certModel.DelegatedNodeID],
	}
}

func GetCertList(c *gin.Context) {
	s := logger.NewSessionLogger(c)
	s.Info("GetCertList")

	usageIndex := site.BuildCertificateUsageIndex()
	keyword := strings.TrimSpace(c.Query("keyword"))
	filter := cert.ParseListFilter(c.Query("state"))
	withCounts := cast.ToBool(c.Query("with_counts"))

	var summary *certListSummary
	if filter != cert.FilterAll || withCounts {
		var err error
		summary, err = summarizeCertList(keyword, filter)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	var providers map[uint64]string
	core := cosy.Core[model.Cert](c).SetFussy("name", "domain").
		GormScope(func(tx *gorm.DB) *gorm.DB {
			return applyCertKeyword(tx, keyword)
		}).
		SetScan(func(tx *gorm.DB) any {
			models := make([]*model.Cert, 0)
			tx.Find(&models)
			providers = dnsProviderNames(models)
			nodeNames := delegatedNodeNames(models)

			rows := make([]any, 0, len(models))
			for _, m := range models {
				var info *cert.Info
				if summary != nil {
					info = summary.infos[m.ID]
				} else {
					info, _ = cert.GetCertInfo(m.SSLCertificatePath)
				}
				rows = append(rows, APICertificate{
					Cert:              m,
					CertificateInfo:   info,
					DeploymentStatus:  site.InspectCertificateDeployment(m),
					UsedBy:            usageIndex.Lookup(m.SSLCertificatePath),
					Overview:          buildOverview(m, info),
					DNSProvider:       providers[m.DnsCredentialID],
					DelegatedNodeName: nodeNames[m.DelegatedNodeID],
				})
			}
			return rows
		})

	if summary != nil {
		core.GormScope(func(tx *gorm.DB) *gorm.DB {
			if filter == cert.FilterAll {
				return tx
			}
			if len(summary.matched) == 0 {
				return tx.Where("1 = 0")
			}
			return tx.Where("certs.id IN ?", summary.matched)
		})
		if withCounts {
			core.SetResponseBuilder(func(ctx *cosy.Ctx[model.Cert]) {
				list, _ := ctx.GetDefaultResponseData().(cosyModel.DataList)
				ctx.JSON(http.StatusOK, certListResponse{DataList: list, Counts: summary.counts})
			})
		}
	}

	core.PagingList()
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

	// A certificate issued for a node keeps its files there unless asked.
	if certModel.IsDelegated() {
		if err = cert.RemoveDelegated(c.Request.Context(), certModel, cast.ToBool(c.Query("remove_remote"))); err != nil {
			cosy.ErrHandler(c, err)
		}
		return
	}

	if err = query.Cert.DeleteByID(id); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	cleanupSelfSignedCertFiles(certModel)
}

// SyncCertificatePaths tells another instance where this node keeps the
// certificate of one of its configurations.
func SyncCertificatePaths(c *gin.Context) {
	var json cert.SyncPathsRequest
	if !cosy.BindAndValid(c, &json) {
		return
	}
	c.JSON(http.StatusOK, cert.SyncPathsFor(json))
}

// RemoveSyncedCertificate deletes certificate files another instance sent to
// this node, unless the Nginx configuration still loads them.
func RemoveSyncedCertificate(c *gin.Context) {
	var json cert.SyncPaths
	if !cosy.BindAndValid(c, &json) {
		return
	}
	if err := cert.RemoveSynced(json); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
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
	normalizedKeyType := helper.GetKeyType(json.KeyType)

	// The sender keeps renewing a certificate it issued for this node.
	if json.Delegated {
		if err := cert.StopRenewingDelegated(json.SSLCertificatePath, json.SSLCertificateKeyPath); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

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

	// Every save of a site replicates the certificates it loads, so an
	// unchanged pair must not cost the node a reload each time.
	if content.MatchesFiles() {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
			"id":      certModel.ID,
		})
		return
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
		"id":      certModel.ID,
	})
}

// SetCertAutoRenewal switches automatic renewal of an ACME certificate.
func SetCertAutoRenewal(c *gin.Context) {
	var json struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	certModel, err := query.Cert.FirstByID(cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if err = cert.SetAutoRenewal(certModel, *json.Enabled); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}
