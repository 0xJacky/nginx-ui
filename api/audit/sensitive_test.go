package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCredentialName(t *testing.T) {
	for _, name := range []string{"password", "old_password", "pfx_password", "client_secret", "token", "short_token", "accessToken", "private_key", "eab_hmac_key", "s3_secret_access_key", "aes_key", "CLOUDFLARE_DNS_API_TOKEN", "NAMESILO_API_KEY", "ALICLOUD_ACCESS_KEY", "X-Install-Secret", "install_secret", "ssl_certificate_key", "device_key"} {
		assert.True(t, isCredentialName(name), name)
	}
	for _, name := range []string{"name", "content", "tokenized", "max_tokens", "public_key", "key_type", "site_key", "cache_key", "keys_zone", "ssl_certificate_key_path", "private_key_path", "Sec-Websocket-Key", "Content-Type", "X-Backup-Security", "credential_id"} {
		assert.False(t, isCredentialName(name), name)
	}
}
