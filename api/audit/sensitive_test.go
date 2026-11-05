package audit

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// nonSecretProviderSettingSuffixes cover provider settings that identify an
// account, endpoint or option rather than hold a secret: user names, IDs,
// URLs, regions, file paths and similar.
var nonSecretProviderSettingSuffixes = []string{
	"_USERNAME", "_USER_NAME", "_USER", "_LOGIN", "_EMAIL", "_NICKNAME", "_ID", "_OCID", "_URL", "_HOST",
	"_HOSTNAME", "_ENDPOINT", "_ADDRESS", "_NAMESERVER", "_REGION", "_PROJECT", "_ROLE", "_ARN", "_ZONE",
	"_BUCKET", "_PATH", "_FILE", "_EDGERC", "_SECTION", "_NAME", "_VIEW", "_MODE", "_CONFIG", "_NUMBER",
	"_REFERENCE", "_CODE", "_FINGERPRINT", "_RECORDS_MAPPING", "_API_BASE", "_CHANGED", "_PROFILE",
}

// nonSecretProviderSettings are the remaining non-secret entries.
var nonSecretProviderSettings = map[string]struct{}{
	// A documentation entry in the catalog, not an environment variable.
	"Application Default Credentials": {},
	// UCloud pairs this public key with UCLOUD_PRIVATE_KEY, which is redacted.
	"UCLOUD_PUBLIC_KEY": {},
}

// TestIsCredentialNameCoversDNSProviderCredentials runs the matcher over every
// credential setting in the bundled lego provider catalog, so a provider whose
// secret uses an unusual name cannot slip through redaction unnoticed.
func TestIsCredentialNameCoversDNSProviderCredentials(t *testing.T) {
	providers := dns.GetProvidersList()
	require.NotEmpty(t, providers)
	checked := 0
	for _, listed := range providers {
		provider, ok := dns.GetProvider(listed.Code)
		if !ok || provider.Configuration == nil {
			continue
		}
		for name := range provider.Configuration.Credentials {
			checked++
			if isCredentialName(name) {
				continue
			}
			if _, ok := nonSecretProviderSettings[name]; ok {
				continue
			}
			nonSecret := false
			for _, suffix := range nonSecretProviderSettingSuffixes {
				if strings.HasSuffix(name, suffix) {
					nonSecret = true
					break
				}
			}
			assert.True(t, nonSecret, "%s credential %q is neither redacted nor listed as non-secret", provider.Code, name)
		}
	}
	require.Greater(t, checked, 100)
}
