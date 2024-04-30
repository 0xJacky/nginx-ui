package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v4/certcrypto"
)

type ConfigPayload struct {
	ServerName      []string           `json:"server_name"`
	ChallengeMethod string             `json:"challenge_method"`
	DNSCredentialID int                `json:"dns_credential_id"`
	ACMEUserID      int                `json:"acme_user_id"`
	KeyType         certcrypto.KeyType `json:"key_type"`
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
