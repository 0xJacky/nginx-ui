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
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

type ConfigPayload struct {
	CertID                  uint64                     `json:"cert_id"`
	ServerName              []string                   `json:"server_name"`
	ChallengeMethod         string                     `json:"challenge_method"`
	DNSCredentialID         uint64                     `json:"dns_credential_id"`
	ACMEUserID              uint64                     `json:"acme_user_id"`
	KeyType                 certcrypto.KeyType         `json:"key_type"`
	Resource                *model.CertificateResource `json:"resource,omitempty"`
	MustStaple              bool                       `json:"must_staple"`
	LegoDisableCNAMESupport bool                       `json:"lego_disable_cname_support"`
	NotBefore               time.Time                  `json:"-"`
	CertificateDir          string                     `json:"-"`
	SSLCertificatePath      string                     `json:"-"`
	SSLCertificateKeyPath   string                     `json:"-"`
	RevokeOld               bool                       `json:"revoke_old"`
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

func (c *ConfigPayload) mkCertificateDir() (err error) {
	dir := c.getCertificateDirPath()
	if !helper.FileExists(dir) {
		err = os.MkdirAll(dir, 0755)
		if err == nil {
			return nil
		}
	} else {
		return nil
	}

	// For windows, replace * with # (issue #403)
	c.CertificateDir = strings.ReplaceAll(c.CertificateDir, "*", "#")
	if _, err = os.Stat(c.CertificateDir); os.IsNotExist(err) {
		err = os.MkdirAll(c.CertificateDir, 0755)
		if err == nil {
			return nil
		}
	}

	return
}

func (c *ConfigPayload) WriteFile(l *Logger) error {
	err := c.mkCertificateDir()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrMakeCertificateDir, err.Error())
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	l.Info(translation.C("[Nginx UI] Writing certificate to disk"))
	err = os.WriteFile(c.GetCertificatePath(),
		c.Resource.Certificate, 0644)

	if err != nil {
		return cosy.WrapErrorWithParams(ErrWriteFullchainCer, err.Error())
	}

	l.Info(translation.C("[Nginx UI] Writing certificate private key to disk"))
	err = os.WriteFile(c.GetCertificateKeyPath(),
		c.Resource.PrivateKey, 0644)

	if err != nil {
		return cosy.WrapErrorWithParams(ErrWritePrivateKey, err.Error())
	}

	// update database
	if c.CertID <= 0 {
		return nil
	}

	db := model.UseDB()
	db.Where("id = ?", c.CertID).Updates(&model.Cert{
		SSLCertificatePath:    c.GetCertificatePath(),
		SSLCertificateKeyPath: c.GetCertificateKeyPath(),
		Resource:              c.Resource,
	})

	return nil
}

func (c *ConfigPayload) getCertificateDirPath() string {
	if c.CertificateDir != "" {
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
