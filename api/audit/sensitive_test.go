package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCredentialName(t *testing.T) {
	for _, name := range []string{
		"password", "old_password", "pfx_password", "client_secret", "token", "short_token", "accessToken",
		"private_key", "eab_hmac_key", "s3_secret_access_key", "aes_key", "CLOUDFLARE_DNS_API_TOKEN",
		"NAMESILO_API_KEY", "ALICLOUD_ACCESS_KEY", "X-Install-Secret", "install_secret", "ssl_certificate_key",
		"device_key", "signature",
		// DNS provider settings that do not use the common words.
		"OVH_APPLICATION_KEY", "OVH_CONSUMER_KEY", "VOLC_ACCESSKEY", "VOLC_SECRETKEY", "HURRICANE_TOKENS",
		"EPIK_SIGNATURE", "DNSHOMEDE_CREDENTIALS", "SYSE_CREDENTIALS", "TELE3_KEY", "EASYDNS_KEY", "DDNSS_KEY",
		"GNAME_APP_KEY", "SELFHOSTDEV2_API_KEYS", "REGRU_TLS_KEY", "DNSUPDATE_TSIG_KEY",
		"MYADDR_PRIVATE_KEYS_MAPPING", "GCE_SERVICE_ACCOUNT",
	} {
		assert.True(t, isCredentialName(name), name)
	}
	for _, name := range []string{
		"name", "content", "tokenized", "max_tokens", "tokens", "credentials", "public_key", "key_type", "site_key",
		"cache_key", "keys_zone", "ssl_certificate_key_path", "private_key_path", "Sec-Websocket-Key",
		"Content-Type", "X-Backup-Security", "credential_id",
		"UCLOUD_PUBLIC_KEY", "OCI_PRIVATE_KEY_PATH", "TRANSIP_PRIVATE_KEY_PATH", "AWS_ACCESS_KEY_ID",
		"GCE_SERVICE_ACCOUNT_FILE", "CLOUDRU_KEY_ID",
	} {
		assert.False(t, isCredentialName(name), name)
	}
}
