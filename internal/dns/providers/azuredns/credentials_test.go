package azuredns

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const testTenantID = "00000000-0000-0000-0000-000000000000"

func TestParseAuthConfigPrefersValuesOverAdditional(t *testing.T) {
	cred := &dns.Credential{
		Values: map[string]string{
			"AZURE_TENANT_ID":     " tenant-from-values ",
			"AZURE_CLIENT_ID":     "client-from-values",
			"AZURE_CLIENT_SECRET": "secret",
		},
		Additional: map[string]string{
			"AZURE_TENANT_ID":       "tenant-from-additional",
			"AZURE_SUBSCRIPTION_ID": "subscription",
			"AZURE_RESOURCE_GROUP":  " rg-dns ",
			"AZURE_ZONE_NAME":       "example.com",
			"AZURE_ENVIRONMENT":     "china",
		},
	}

	cfg := parseAuthConfig(cred)

	require.Equal(t, "tenant-from-values", cfg.TenantID)
	require.Equal(t, "client-from-values", cfg.ClientID)
	require.Equal(t, "secret", cfg.ClientSecret)
	require.Equal(t, "subscription", cfg.SubscriptionID)
	require.Equal(t, "rg-dns", cfg.ResourceGroup)
	require.Equal(t, "example.com", cfg.ZoneName)
	require.Equal(t, "china", cfg.Environment)
	require.False(t, cfg.PrivateZone)
	require.Equal(t, defaultTokenTimeout, cfg.TokenTimeout)
}

func TestParseAuthConfigFallsBackToAdditional(t *testing.T) {
	cred := &dns.Credential{
		Values: map[string]string{},
		Additional: map[string]string{
			"AZURE_TENANT_ID": "tenant",
		},
	}

	require.Equal(t, "tenant", parseAuthConfig(cred).TenantID)
}

func TestParseAuthConfigHandlesNilCredential(t *testing.T) {
	cfg := parseAuthConfig(nil)

	require.Empty(t, cfg.TenantID)
	require.Equal(t, defaultTokenTimeout, cfg.TokenTimeout)
}

func TestParseBool(t *testing.T) {
	for _, value := range []string{"1", "t", "T", "true", "TRUE", "True", "yes", "YES", "on", "ON"} {
		require.True(t, parseBool(value), "expected %q to be truthy", value)
	}

	for _, value := range []string{"", "0", "false", "FALSE", "no", "off", "garbage"} {
		require.False(t, parseBool(value), "expected %q to be falsy", value)
	}
}

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{input: "", want: defaultTokenTimeout},
		{input: "3", want: 3 * time.Second},
		{input: " 10 ", want: 10 * time.Second},
		{input: "3s", want: 3 * time.Second},
		{input: "1500ms", want: 1500 * time.Millisecond},
		{input: "0", want: defaultTokenTimeout},
		{input: "-5", want: defaultTokenTimeout},
		{input: "garbage", want: defaultTokenTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.want, parseTimeout(tt.input))
		})
	}
}

func TestResolveAuthMethodExplicit(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{method: "secret", want: authMethodSecret},
		{method: "env", want: authMethodSecret},
		{method: "client_secret", want: authMethodSecret},
		{method: "cert", want: authMethodCert},
		{method: "certificate", want: authMethodCert},
		{method: "msi", want: authMethodMSI},
		{method: "managedidentity", want: authMethodMSI},
		{method: "wli", want: authMethodWLI},
		{method: "workload_identity", want: authMethodWLI},
		{method: "cli", want: authMethodCLI},
		{method: "azurecli", want: authMethodCLI},
		{method: "default", want: authMethodDefault},
		{method: "auto", want: authMethodDefault},
		{method: "chain", want: authMethodDefault},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got, err := resolveAuthMethod(authConfig{Method: tt.method})
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolveAuthMethodRejectsUnsupported(t *testing.T) {
	for _, method := range []string{"oidc", "pipeline", "carrier-pigeon"} {
		t.Run(method, func(t *testing.T) {
			_, err := resolveAuthMethod(authConfig{Method: method})
			require.Error(t, err)
		})
	}
}

func TestResolveAuthMethodInference(t *testing.T) {
	tests := []struct {
		name string
		cfg  authConfig
		want string
	}{
		{
			name: "client secret triple",
			cfg:  authConfig{TenantID: "t", ClientID: "c", ClientSecret: "s"},
			want: authMethodSecret,
		},
		{
			name: "client certificate",
			cfg:  authConfig{TenantID: "t", ClientID: "c", CertificatePath: "/tmp/cert.pem"},
			want: authMethodCert,
		},
		{
			name: "secret wins over certificate",
			cfg:  authConfig{TenantID: "t", ClientID: "c", ClientSecret: "s", CertificatePath: "/tmp/cert.pem"},
			want: authMethodSecret,
		},
		{
			name: "nothing configured falls back to the default chain",
			cfg:  authConfig{},
			want: authMethodDefault,
		},
		{
			name: "incomplete secret falls back to the default chain",
			cfg:  authConfig{TenantID: "t", ClientSecret: "s"},
			want: authMethodDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAuthMethod(tt.cfg)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseCloud(t *testing.T) {
	tests := []struct {
		input   string
		want    cloud.Configuration
		wantErr bool
	}{
		{input: "", want: cloud.AzurePublic},
		{input: "public", want: cloud.AzurePublic},
		{input: "Public", want: cloud.AzurePublic},
		{input: " AzureCloud ", want: cloud.AzurePublic},
		{input: "usgovernment", want: cloud.AzureGovernment},
		{input: "USGov", want: cloud.AzureGovernment},
		{input: "china", want: cloud.AzureChina},
		{input: "AzureChinaCloud", want: cloud.AzureChina},
		{input: "mars", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseCloud(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCredentialFingerprint(t *testing.T) {
	base := authConfig{
		TenantID:       "tenant",
		ClientID:       "client",
		ClientSecret:   "super-secret",
		SubscriptionID: "subscription",
		TokenTimeout:   defaultTokenTimeout,
	}

	fingerprint := credentialFingerprint(base, authMethodSecret, "public")

	require.Equal(t, fingerprint, credentialFingerprint(base, authMethodSecret, "public"))
	require.NotContains(t, fingerprint, "super-secret")

	rotated := base
	rotated.ClientSecret = "rotated-secret"
	require.NotEqual(t, fingerprint, credentialFingerprint(rotated, authMethodSecret, "public"))

	otherTenant := base
	otherTenant.TenantID = "other"
	require.NotEqual(t, fingerprint, credentialFingerprint(otherTenant, authMethodSecret, "public"))

	otherSubscription := base
	otherSubscription.SubscriptionID = "other"
	require.NotEqual(t, fingerprint, credentialFingerprint(otherSubscription, authMethodSecret, "public"))

	require.NotEqual(t, fingerprint, credentialFingerprint(base, authMethodSecret, "china"))
	require.NotEqual(t, fingerprint, credentialFingerprint(base, authMethodCLI, "public"))
}

func TestNewProviderRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{
			name: "missing subscription",
			values: map[string]string{
				"AZURE_TENANT_ID":     testTenantID,
				"AZURE_CLIENT_ID":     "client",
				"AZURE_CLIENT_SECRET": "secret",
			},
		},
		{
			name: "private zone",
			values: map[string]string{
				"AZURE_SUBSCRIPTION_ID": "subscription",
				"AZURE_PRIVATE_ZONE":    "true",
			},
		},
		{
			name: "unknown environment",
			values: map[string]string{
				"AZURE_SUBSCRIPTION_ID": "subscription",
				"AZURE_ENVIRONMENT":     "mars",
			},
		},
		{
			name: "unsupported auth method",
			values: map[string]string{
				"AZURE_SUBSCRIPTION_ID": "subscription",
				"AZURE_AUTH_METHOD":     "oidc",
			},
		},
		{
			name: "client secret auth without a secret",
			values: map[string]string{
				"AZURE_SUBSCRIPTION_ID": "subscription",
				"AZURE_AUTH_METHOD":     "secret",
				"AZURE_TENANT_ID":       testTenantID,
				"AZURE_CLIENT_ID":       "client",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newProvider(&dns.Credential{Code: providerCode, Values: tt.values})
			require.Error(t, err)
		})
	}
}

func TestNewProviderBuildsClientSecretProvider(t *testing.T) {
	created, err := newProvider(&dns.Credential{
		Code: providerCode,
		Values: map[string]string{
			"AZURE_TENANT_ID":     testTenantID,
			"AZURE_CLIENT_ID":     "client-id",
			"AZURE_CLIENT_SECRET": "client-secret",
		},
		Additional: map[string]string{
			"AZURE_SUBSCRIPTION_ID": "subscription-id",
			"AZURE_RESOURCE_GROUP":  "rg-dns",
		},
	})
	require.NoError(t, err)

	azureProvider, ok := created.(*provider)
	require.True(t, ok)
	require.Equal(t, "subscription-id", azureProvider.subscriptionID)
	require.Equal(t, "rg-dns", azureProvider.resourceGroup)
	require.NotEmpty(t, azureProvider.fingerprint)
	require.NotNil(t, azureProvider.credential)
	require.Equal(t, cloud.AzurePublic, azureProvider.cloudConfig)
}

func TestProviderIsRegistered(t *testing.T) {
	created, err := dns.NewProvider(providerCode, &dns.Credential{
		Code: providerCode,
		Values: map[string]string{
			"AZURE_TENANT_ID":       testTenantID,
			"AZURE_CLIENT_ID":       "client-id",
			"AZURE_CLIENT_SECRET":   "client-secret",
			"AZURE_SUBSCRIPTION_ID": "subscription-id",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, created)
}

// stubCredential blocks until the caller's context is done so that the timeout
// wrapper can be observed without any network access.
type stubCredential struct {
	calls int
}

func (c *stubCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.calls++
	<-ctx.Done()

	return azcore.AccessToken{}, ctx.Err()
}

func TestTimeoutCredentialEnforcesDeadlineOnEveryCall(t *testing.T) {
	stub := &stubCredential{}
	credential := withTokenTimeout(stub, 20*time.Millisecond)

	for i := range 2 {
		start := time.Now()
		_, err := credential.GetToken(context.Background(), policy.TokenRequestOptions{})
		require.ErrorIs(t, err, context.DeadlineExceeded, "call %d should time out", i+1)
		require.Less(t, time.Since(start), time.Second)
	}

	require.Equal(t, 2, stub.calls)
}

func TestWithTokenTimeoutPassesThroughWhenDisabled(t *testing.T) {
	stub := &stubCredential{}

	require.Same(t, stub, withTokenTimeout(stub, 0))
	require.Same(t, stub, withTokenTimeout(stub, -time.Second))
}
