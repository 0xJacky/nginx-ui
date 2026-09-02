package cert

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

type ConfigPayload struct {
	CertID                            uint64                     `json:"cert_id"`
	ServerName                        []string                   `json:"server_name"`
	ChallengeMethod                   string                     `json:"challenge_method"`
	Profile                           string                     `json:"profile"`
	DNSCredentialID                   uint64                     `json:"dns_credential_id"`
	ACMEUserID                        uint64                     `json:"acme_user_id"`
	KeyType                           certcrypto.KeyType         `json:"key_type"`
	Resource                          *model.CertificateResource `json:"resource,omitempty"`
	MustStaple                        bool                       `json:"must_staple"`
	LegoDisableCNAMESupport           bool                       `json:"lego_disable_cname_support"`
	DisableAuthoritativeNSPropagation bool                       `json:"disable_authoritative_ns_propagation"`
	EnableCommonName                  bool                       `json:"enable_common_name"`
	NotBefore                         time.Time                  `json:"-"`
	CertificateDir                    string                     `json:"-"`
	SSLCertificatePath                string                     `json:"-"`
	SSLCertificateKeyPath             string                     `json:"-"`
	RevokeOld                         bool                       `json:"revoke_old"`
	ReplacesCertID                    string                     `json:"-"`
}

func (c *ConfigPayload) GetACMEUser() (user *model.AcmeUser, err error) {
	u := query.AcmeUser
	// if acme_user_id == 0, use default user
	if c.ACMEUserID == 0 {
		return GetDefaultACMEUser()
	}
	// use the acme_user_id to get the acme user
	user, err = u.Where(u.ID.Eq(c.ACMEUserID)).First()
	// if acme_user not exist, use default user
	if err != nil {
		logger.Error(err)
		return GetDefaultACMEUser()
	}
	return
}

func (c *ConfigPayload) GetKeyType() certcrypto.KeyType {
	return helper.GetKeyType(c.KeyType)
}

// mkCertificateDir creates the certificate directory on the nginx target
// filesystem, which is the remote host in host_via_ssh + sftp mode.
func (c *ConfigPayload) mkCertificateDir() (err error) {
	dir := c.getCertificateDirPath()
	exists, err := nginx.Exists(dir)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = nginx.MkdirAll(dir, 0755)
	if err == nil {
		return nil
	}

	// For windows, replace * with # (issue #403)
	c.CertificateDir = strings.ReplaceAll(c.CertificateDir, "*", "#")
	if _, err = nginx.Stat(c.CertificateDir); os.IsNotExist(err) {
		err = nginx.MkdirAll(c.CertificateDir, 0755)
		if err == nil {
			return nil
		}
	}

	return
}

// UseExistingCertificatePaths pins the payload to the certificate files that
// the nginx configuration already references, so a renewal replaces them in
// place.
//
// The default certificate directory is derived from the identifier list and the
// key type. Whenever one of those inputs drifts the derived directory changes,
// the renewed material is written to a brand-new directory, and the database row
// is repointed at it while nginx keeps loading the stale files. Known sources of
// drift are identifier normalization (trimming, de-duplication, IP
// canonicalization), editing the domain list of an existing certificate, and the
// lego v4 -> v5 key type rename ("P256" -> "EC256", "2048" -> "RSA2048").
//
// Paths outside the nginx configuration directory are ignored so a corrupted or
// hand-edited record cannot redirect the write to an arbitrary location.
func (c *ConfigPayload) UseExistingCertificatePaths(certificatePath, certificateKeyPath string) {
	if certificatePath == "" || certificateKeyPath == "" {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	if !helper.IsUnderDirectory(certificatePath, nginxConfPath) ||
		!helper.IsUnderDirectory(certificateKeyPath, nginxConfPath) {
		return
	}

	c.SSLCertificatePath = certificatePath
	c.SSLCertificateKeyPath = certificateKeyPath
	c.CertificateDir = filepath.Dir(certificatePath)
}

func (c *ConfigPayload) WriteFile(l *Logger) error {
	err := c.mkCertificateDir()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrMakeCertificateDir, err.Error())
	}

	// The private key does not necessarily live next to the certificate when the
	// paths are reused from an existing record.
	if err = nginx.MkdirAll(filepath.Dir(c.GetCertificateKeyPath()), 0755); err != nil {
		return cosy.WrapErrorWithParams(ErrMakeCertificateDir, err.Error())
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	l.Info(translation.C("[Nginx UI] Writing certificate to disk"))
	err = writeFileWithMode(c.GetCertificatePath(),
		c.Resource.Certificate, 0644)

	if err != nil {
		return cosy.WrapErrorWithParams(ErrWriteFullchainCer, err.Error())
	}

	// The private key must stay owner-only; the certificate above is public.
	l.Info(translation.C("[Nginx UI] Writing certificate private key to disk"))
	err = writeFileWithMode(c.GetCertificateKeyPath(),
		c.Resource.PrivateKey, 0600)

	if err != nil {
		return cosy.WrapErrorWithParams(ErrWritePrivateKey, err.Error())
	}

	// update database
	if c.CertID <= 0 {
		return nil
	}

	fingerprint, _ := CertificateFingerprintFromPath(c.GetCertificatePath())
	db := model.UseDB()
	if db == nil {
		return nil
	}

	// Struct + Select, never a map: Resource carries a `serializer:json[aes]`
	// tag that GORM only applies to struct updates. A map hands the struct
	// straight to the SQL driver, which rejects it and aborts the whole
	// statement, so the record would keep describing the previous certificate.
	// Select is what makes the zero values (cleared ARI schedule) actually be
	// written instead of skipped.
	updates := &model.Cert{
		SSLCertificatePath:           c.GetCertificatePath(),
		SSLCertificateKeyPath:        c.GetCertificateKeyPath(),
		Fingerprint:                  fingerprint,
		Resource:                     c.Resource,
		Profile:                      c.Profile,
		NextAutoRenewAt:              nil,
		LastRenewalInfoCheckAt:       nil,
		AutoRenewScheduleFingerprint: "",
	}
	if err = db.Model(&model.Cert{}).Where("id = ?", c.CertID).
		Select(
			"ssl_certificate_path", "ssl_certificate_key_path", "fingerprint", "resource",
			"profile", "next_auto_renew_at", "last_renewal_info_check_at",
			"auto_renew_schedule_fingerprint",
		).
		Updates(updates).Error; err != nil {
		return cosy.WrapErrorWithParams(ErrPersistCertificateRecord, err.Error())
	}

	return nil
}

func (c *ConfigPayload) getCertificateDirPath() string {
	if c.CertificateDir != "" {
		return c.CertificateDir
	}
	// An explicit certificate path wins over the derived directory so a renewal
	// keeps replacing the file the nginx configuration points at.
	if c.SSLCertificatePath != "" {
		c.CertificateDir = filepath.Dir(c.SSLCertificatePath)
		return c.CertificateDir
	}
	c.CertificateDir = nginx.GetConfPath("ssl", strings.Join(c.ServerName, "_")+"_"+string(c.GetKeyType()))
	return c.CertificateDir
}

func (c *ConfigPayload) GetCertificatePath() string {
	if c.SSLCertificatePath != "" {
		return c.SSLCertificatePath
	}
	c.SSLCertificatePath = filepath.Join(c.getCertificateDirPath(), "fullchain.cer")
	return c.SSLCertificatePath
}

func (c *ConfigPayload) GetCertificateKeyPath() string {
	if c.SSLCertificateKeyPath != "" {
		return c.SSLCertificateKeyPath
	}
	c.SSLCertificateKeyPath = filepath.Join(c.getCertificateDirPath(), "private.key")
	return c.SSLCertificateKeyPath
}
