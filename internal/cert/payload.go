package cert

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/go-acme/lego/v4/certcrypto"
)

type ConfigPayload struct {
	ServerName      []string           `json:"server_name"`
	ChallengeMethod string             `json:"challenge_method"`
	DNSCredentialID int                `json:"dns_credential_id"`
	KeyType         certcrypto.KeyType `json:"key_type"`
}

func (c *ConfigPayload) GetKeyType() certcrypto.KeyType {
	return helper.GetKeyType(c.KeyType)
}
