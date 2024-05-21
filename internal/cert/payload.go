package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/pkg/errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ConfigPayload struct {
	CertID          int                        `json:"cert_id"`
	ServerName      []string                   `json:"server_name"`
	ChallengeMethod string                     `json:"challenge_method"`
	DNSCredentialID int                        `json:"dns_credential_id"`
	ACMEUserID      int                        `json:"acme_user_id"`
	KeyType         certcrypto.KeyType         `json:"key_type"`
	Resource        *model.CertificateResource `json:"resource,omitempty"`
	NotBefore       time.Time
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

func (c *ConfigPayload) WriteFile(l *log.Logger, errChan chan error) {
	name := strings.Join(c.ServerName, "_")
	saveDir := nginx.GetConfPath("ssl/" + name + "_" + string(c.KeyType))
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		err = os.MkdirAll(saveDir, 0755)
		if err != nil {
			errChan <- errors.Wrap(err, "mkdir error")
			return
		}
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	l.Println("[INFO] [Nginx UI] Writing certificate to disk")
	err := os.WriteFile(filepath.Join(saveDir, "fullchain.cer"),
		c.Resource.Certificate, 0644)

	if err != nil {
		errChan <- errors.Wrap(err, "write fullchain.cer error")
		return
	}

	l.Println("[INFO] [Nginx UI] Writing certificate private key to disk")
	err = os.WriteFile(filepath.Join(saveDir, "private.key"),
		c.Resource.PrivateKey, 0644)

	if err != nil {
		errChan <- errors.Wrap(err, "write private.key error")
		return
	}

	// update database
	if c.CertID <= 0 {
		return
	}

	db := model.UseDB()
	db.Where("id = ?", c.CertID).Updates(&model.Cert{
		SSLCertificatePath:    filepath.Join(saveDir, "fullchain.cer"),
		SSLCertificateKeyPath: filepath.Join(saveDir, "private.key"),
	})
}
