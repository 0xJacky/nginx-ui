package azuredns

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

// Authentication methods supported when talking to the Azure DNS management API.
const (
	authMethodSecret  = "secret"
	authMethodCert    = "cert"
	authMethodMSI     = "msi"
	authMethodWLI     = "wli"
	authMethodCLI     = "cli"
	authMethodDefault = "default"
)

// defaultTokenTimeout bounds a single token acquisition. The DNS service gives every
// provider call a 10s budget, so an unbounded IMDS probe or `az` invocation would
// consume the whole request on its own.
const defaultTokenTimeout = 5 * time.Second

// credentialCacheLimit bounds the process-wide token credential cache. Providers are
// constructed per API call, so credentials must be reused to avoid a fresh OAuth
// round trip on every request.
const credentialCacheLimit = 64

// authConfig holds the Azure settings extracted from a DNS credential.
type authConfig struct {
	Method             string
	Environment        string
	TenantID           string
	ClientID           string
	ClientSecret       string
	CertificatePath    string
	CertificatePwd     string
	FederatedTokenFile string
	SubscriptionID     string
	ResourceGroup      string
	ZoneName           string
	PrivateZone        bool
	TokenTimeout       time.Duration
}

// parseAuthConfig reads the Azure settings from a credential, preferring the
// credential values and falling back to the additional configuration map.
func parseAuthConfig(cred *dns.Credential) authConfig {
	cfg := authConfig{
		Method:             strings.ToLower(lookupCredential(cred, "AZURE_AUTH_METHOD")),
		Environment:        lookupCredential(cred, "AZURE_ENVIRONMENT"),
		TenantID:           lookupCredential(cred, "AZURE_TENANT_ID"),
		ClientID:           lookupCredential(cred, "AZURE_CLIENT_ID"),
		ClientSecret:       lookupCredential(cred, "AZURE_CLIENT_SECRET"),
		CertificatePath:    lookupCredential(cred, "AZURE_CLIENT_CERTIFICATE_PATH"),
		CertificatePwd:     lookupCredential(cred, "AZURE_CLIENT_CERTIFICATE_PASSWORD"),
		FederatedTokenFile: lookupCredential(cred, "AZURE_FEDERATED_TOKEN_FILE"),
		SubscriptionID:     lookupCredential(cred, "AZURE_SUBSCRIPTION_ID"),
		ResourceGroup:      lookupCredential(cred, "AZURE_RESOURCE_GROUP"),
		ZoneName:           lookupCredential(cred, "AZURE_ZONE_NAME"),
		PrivateZone:        parseBool(lookupCredential(cred, "AZURE_PRIVATE_ZONE")),
		TokenTimeout:       parseTimeout(lookupCredential(cred, "AZURE_AUTH_MSI_TIMEOUT")),
	}

	return cfg
}

// resolveAuthMethod normalizes the configured authentication method, inferring one
// from the supplied secrets when the user did not pick explicitly.
func resolveAuthMethod(cfg authConfig) (string, error) {
	switch cfg.Method {
	case "secret", "env", "client_secret", "clientsecret":
		return authMethodSecret, nil
	case "cert", "certificate", "client_certificate", "clientcertificate":
		return authMethodCert, nil
	case "msi", "managedidentity", "managed_identity":
		return authMethodMSI, nil
	case "wli", "workloadidentity", "workload_identity":
		return authMethodWLI, nil
	case "cli", "azurecli", "azure_cli":
		return authMethodCLI, nil
	case "oidc", "pipeline":
		// Both exchange a short lived CI token that cannot be stored in a
		// long lived credential, so refuse instead of silently degrading.
		return "", fmt.Errorf("azuredns: auth method %q is not supported for DNS record management", cfg.Method)
	case "", "default", "auto", "chain":
		break
	default:
		return "", fmt.Errorf("azuredns: unknown auth method %q", cfg.Method)
	}

	switch {
	case cfg.TenantID != "" && cfg.ClientID != "" && cfg.ClientSecret != "":
		return authMethodSecret, nil
	case cfg.TenantID != "" && cfg.ClientID != "" && cfg.CertificatePath != "":
		return authMethodCert, nil
	default:
		return authMethodDefault, nil
	}
}

// parseCloud maps the configured environment name onto an Azure cloud configuration.
func parseCloud(name string) (cloud.Configuration, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "public", "azurecloud", "azurepublic":
		return cloud.AzurePublic, nil
	case "usgovernment", "usgov", "azureusgovernment", "azureusgovernmentcloud":
		return cloud.AzureGovernment, nil
	case "china", "azurechina", "azurechinacloud":
		return cloud.AzureChina, nil
	default:
		return cloud.Configuration{}, fmt.Errorf("azuredns: unknown environment %q", name)
	}
}

var (
	credentialCacheMu sync.Mutex
	credentialCache   = map[string]azcore.TokenCredential{}
)

// cachedTokenCredential returns a shared token credential for the given fingerprint,
// building one on first use. Token credentials are goroutine safe and cache access
// tokens internally, so reusing them avoids re-authenticating on every request.
func cachedTokenCredential(cfg authConfig, method string, cloudCfg cloud.Configuration, fingerprint string) (azcore.TokenCredential, error) {
	credentialCacheMu.Lock()
	defer credentialCacheMu.Unlock()

	if cached, ok := credentialCache[fingerprint]; ok {
		return cached, nil
	}

	credential, err := newTokenCredential(cfg, method, cloudCfg)
	if err != nil {
		return nil, err
	}

	if len(credentialCache) >= credentialCacheLimit {
		clear(credentialCache)
	}
	credentialCache[fingerprint] = credential

	return credential, nil
}

// newTokenCredential builds an Azure token credential for the resolved auth method.
func newTokenCredential(cfg authConfig, method string, cloudCfg cloud.Configuration) (azcore.TokenCredential, error) {
	clientOptions := azcore.ClientOptions{Cloud: cloudCfg}

	switch method {
	case authMethodSecret:
		if cfg.TenantID == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
			return nil, fmt.Errorf("azuredns: client secret auth requires AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_SECRET")
		}
		return azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret,
			&azidentity.ClientSecretCredentialOptions{ClientOptions: clientOptions})

	case authMethodCert:
		if cfg.TenantID == "" || cfg.ClientID == "" || cfg.CertificatePath == "" {
			return nil, fmt.Errorf("azuredns: certificate auth requires AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_CERTIFICATE_PATH")
		}
		data, err := os.ReadFile(cfg.CertificatePath)
		if err != nil {
			return nil, fmt.Errorf("azuredns: read client certificate: %w", err)
		}
		certs, key, err := azidentity.ParseCertificates(data, []byte(cfg.CertificatePwd))
		if err != nil {
			return nil, fmt.Errorf("azuredns: parse client certificate: %w", err)
		}
		return azidentity.NewClientCertificateCredential(cfg.TenantID, cfg.ClientID, certs, key,
			&azidentity.ClientCertificateCredentialOptions{ClientOptions: clientOptions})

	case authMethodMSI:
		options := &azidentity.ManagedIdentityCredentialOptions{ClientOptions: clientOptions}
		if cfg.ClientID != "" {
			options.ID = azidentity.ClientID(cfg.ClientID)
		}
		credential, err := azidentity.NewManagedIdentityCredential(options)
		if err != nil {
			return nil, err
		}
		return withTokenTimeout(credential, cfg.TokenTimeout), nil

	case authMethodWLI:
		credential, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientOptions: clientOptions,
			ClientID:      cfg.ClientID,
			TenantID:      cfg.TenantID,
			TokenFilePath: cfg.FederatedTokenFile,
		})
		if err != nil {
			return nil, err
		}
		return withTokenTimeout(credential, cfg.TokenTimeout), nil

	case authMethodCLI:
		credential, err := azidentity.NewAzureCLICredential(&azidentity.AzureCLICredentialOptions{TenantID: cfg.TenantID})
		if err != nil {
			return nil, err
		}
		return withTokenTimeout(credential, cfg.TokenTimeout), nil

	default:
		credential, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
			ClientOptions: clientOptions,
			TenantID:      cfg.TenantID,
		})
		if err != nil {
			return nil, err
		}
		return withTokenTimeout(credential, cfg.TokenTimeout), nil
	}
}

// credentialFingerprint derives a stable cache key from every auth relevant field so
// that editing a credential never reuses the previous token credential.
func credentialFingerprint(cfg authConfig, method string, environment string) string {
	parts := []string{
		method,
		environment,
		cfg.TenantID,
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.CertificatePath,
		cfg.CertificatePwd,
		cfg.FederatedTokenFile,
		cfg.SubscriptionID,
		cfg.TokenTimeout.String(),
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))

	return hex.EncodeToString(sum[:])
}

// timeoutCredential bounds every token acquisition performed by the wrapped credential.
type timeoutCredential struct {
	inner   azcore.TokenCredential
	timeout time.Duration
}

// withTokenTimeout wraps a credential when a positive timeout is configured.
func withTokenTimeout(inner azcore.TokenCredential, timeout time.Duration) azcore.TokenCredential {
	if timeout <= 0 {
		return inner
	}

	return &timeoutCredential{inner: inner, timeout: timeout}
}

// GetToken applies the configured deadline to the wrapped credential.
func (c *timeoutCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.inner.GetToken(ctxWithTimeout, opts)
}

// lookupCredential reads a key from the credential values, falling back to the
// additional configuration map.
func lookupCredential(cred *dns.Credential, key string) string {
	if cred == nil {
		return ""
	}

	if value := strings.TrimSpace(cred.Values[key]); value != "" {
		return value
	}

	return strings.TrimSpace(cred.Additional[key])
}

// parseBool accepts the usual truthy spellings plus the ones the Azure docs use.
func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "on":
		return true
	default:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		return err == nil && parsed
	}
}

// parseTimeout accepts a bare number of seconds or a Go duration string.
func parseTimeout(value string) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultTokenTimeout
	}

	if seconds, err := strconv.Atoi(trimmed); err == nil {
		if seconds <= 0 {
			return defaultTokenTimeout
		}
		return time.Duration(seconds) * time.Second
	}

	if duration, err := time.ParseDuration(trimmed); err == nil && duration > 0 {
		return duration
	}

	return defaultTokenTimeout
}
