package certificate

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// defaultSelfSignedSlug is the fallback certificate-directory slug.
const defaultSelfSignedSlug = "self_signed"

// SelfSignedCertRequest is the payload for generating or modifying a
// self-signed certificate.
type SelfSignedCertRequest struct {
	Name         string   `json:"name"`
	Domains      []string `json:"domains" binding:"omitempty"`
	IPAddresses  []string `json:"ip_addresses" binding:"omitempty,dive,ip"`
	KeyType      string   `json:"key_type" binding:"omitempty,auto_cert_key_type"`
	ValidityDays int      `json:"validity_days" binding:"omitempty,min=1,max=3650"`
	SyncNodeIds  []uint64 `json:"sync_node_ids" binding:"omitempty"`
}

// GenerateSelfSignedCert creates a new self-signed certificate.
func GenerateSelfSignedCert(c *gin.Context) {
	var req SelfSignedCertRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	opts, err := buildSelfSignedOptions(&req)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	db := model.UseDB()
	certModel := &model.Cert{
		Name:     req.Name,
		Domains:  opts.DNSNames,
		AutoCert: model.AutoCertSelfSigned,
		KeyType:  opts.KeyType,
		SelfSignedConfig: &model.SelfSignedCertConfig{
			IPAddresses:  opts.IPAddresses,
			ValidityDays: opts.ValidityDays,
		},
		SyncNodeIds: req.SyncNodeIds,
	}
	if err = db.Create(certModel).Error; err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// derive a unique, filesystem-safe certificate directory using the row ID
	slug := selfSignedSlug(req.Name)
	if slug == defaultSelfSignedSlug {
		slug = selfSignedSlug(opts.CommonName)
	}
	dir := nginx.GetConfPath("ssl", slug+"_"+cast.ToString(certModel.ID))
	certPath := filepath.Join(dir, "fullchain.cer")
	keyPath := filepath.Join(dir, "private.key")

	if err = writeSelfSignedFiles(certPath, keyPath, opts); err != nil {
		// remove the partial directory so a failed generation leaves no orphan files
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			logger.Errorf("self-signed cert directory cleanup failed for id %d at %s: %v",
				certModel.ID, dir, rmErr)
		}
		// roll back the row so a failed generation leaves no orphan record
		if rollbackErr := db.Delete(certModel).Error; rollbackErr != nil {
			logger.Errorf("self-signed cert rollback failed for id %d: %v", certModel.ID, rollbackErr)
		}
		cosy.ErrHandler(c, err)
		return
	}

	certModel.SSLCertificatePath = certPath
	certModel.SSLCertificateKeyPath = keyPath
	if err = db.Model(certModel).Updates(map[string]any{
		"ssl_certificate_path":     certPath,
		"ssl_certificate_key_path": keyPath,
	}).Error; err != nil {
		logger.Errorf("self-signed cert id %d generated at %s but persisting paths failed: %v",
			certModel.ID, dir, err)
		cosy.ErrHandler(c, err)
		return
	}

	if err = cert.SyncToRemoteServer(certModel); err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

// ModifySelfSignedCert re-issues an existing self-signed certificate.
func ModifySelfSignedCert(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))

	var req SelfSignedCertRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	opts, err := buildSelfSignedOptions(&req)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	certModel, err := query.Cert.FirstByID(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if certModel.AutoCert != model.AutoCertSelfSigned {
		cosy.ErrHandler(c, cert.ErrCertIsNotSelfSigned)
		return
	}
	if certModel.SSLCertificatePath == "" || certModel.SSLCertificateKeyPath == "" {
		cosy.ErrHandler(c, cert.ErrCertPathIsEmpty)
		return
	}

	// reuse the existing file paths so sites referencing them keep working
	if err = writeSelfSignedFiles(certModel.SSLCertificatePath,
		certModel.SSLCertificateKeyPath, opts); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	certModel.Name = req.Name
	certModel.Domains = opts.DNSNames
	certModel.KeyType = opts.KeyType
	certModel.SelfSignedConfig = &model.SelfSignedCertConfig{
		IPAddresses:  opts.IPAddresses,
		ValidityDays: opts.ValidityDays,
	}
	certModel.SyncNodeIds = req.SyncNodeIds
	if err = model.UseDB().Save(certModel).Error; err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	nginx.Reload()

	if err = cert.SyncToRemoteServer(certModel); err != nil {
		notification.Error("Sync Certificate Error", err.Error(), nil)
	}

	c.JSON(http.StatusOK, Transformer(certModel))
}

// buildSelfSignedOptions validates and normalizes the request into
// cert.SelfSignedOptions.
func buildSelfSignedOptions(req *SelfSignedCertRequest) (cert.SelfSignedOptions, error) {
	domains := normalizeStringSlice(req.Domains)
	ips := normalizeStringSlice(req.IPAddresses)

	if len(domains) == 0 && len(ips) == 0 {
		return cert.SelfSignedOptions{}, cert.ErrSelfSignedNoSAN
	}

	validityDays := req.ValidityDays
	if validityDays <= 0 {
		validityDays = cert.SelfSignedDefaultValidityDays
	}

	commonName := ""
	if len(domains) > 0 {
		commonName = domains[0]
	} else {
		commonName = ips[0]
	}

	return cert.SelfSignedOptions{
		CommonName:   commonName,
		DNSNames:     domains,
		IPAddresses:  ips,
		KeyType:      helper.GetKeyType(certcrypto.KeyType(req.KeyType)),
		ValidityDays: validityDays,
	}, nil
}

// writeSelfSignedFiles generates a self-signed certificate and writes it to
// the given paths.
func writeSelfSignedFiles(certPath, keyPath string, opts cert.SelfSignedOptions) error {
	certPEM, keyPEM, err := cert.GenerateSelfSigned(opts)
	if err != nil {
		return err
	}
	content := &cert.Content{
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		SSLCertificate:        string(certPEM),
		SSLCertificateKey:     string(keyPEM),
	}
	return content.WriteFile()
}

// normalizeStringSlice trims entries and drops empty strings.
func normalizeStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// selfSignedSlug builds a filesystem-safe directory slug from a name.
func selfSignedSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '.':
			b.WriteRune(r)
		case r == ' ' || r == '_':
			b.WriteRune('_')
		}
	}
	slug := strings.Trim(b.String(), "._-")
	if slug == "" {
		slug = defaultSelfSignedSlug
	}
	return slug
}
