package dns

import (
	"context"
	"maps"

	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
)

func init() {
	RegisterSource(builtinSource{})
}

// builtinSource describes the vendors the built-in DNS record management
// talks to. Their credential schemas stay in the core because the record
// management reads the very same credential rows. Solving DNS-01 is not part
// of it: that belongs to a plugin.
type builtinSource struct{}

func (builtinSource) Providers() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(builtinProviders))
	for _, p := range builtinProviders {
		out = append(out, ProviderInfo{
			Config: Config{
				Name:          p.name,
				Code:          p.code,
				Configuration: &Configuration{Credentials: maps.Clone(p.credentials), Additional: maps.Clone(p.additional)},
				Links:         &Links{API: p.api, GoClient: p.goClient},
			},
			RecordManagement: true,
		})
	}
	return out
}

func (builtinSource) NewChallengeProvider(context.Context, string, Configuration, map[string]any) (challenge.Provider, []dns01.ChallengeOption, func(), error) {
	return nil, nil, func() {}, ErrProviderNotFound
}

// builtinProvider is one record management vendor. The field names and help
// texts mirror lego's provider catalog so a credential entered here keeps
// working when the DNS-01 plugin picks it up.
type builtinProvider struct {
	name        string
	code        string
	credentials map[string]string
	additional  map[string]string
	api         string
	goClient    string
}

var builtinProviders = []builtinProvider{
	{
		name: "Alibaba Cloud DNS",
		code: "alidns",
		credentials: map[string]string{
			"ALICLOUD_RAM_ROLE":       "Your instance RAM role (https://www.alibabacloud.com/help/en/ecs/user-guide/attach-an-instance-ram-role-to-an-ecs-instance)",
			"ALICLOUD_ACCESS_KEY":     "Access key ID",
			"ALICLOUD_SECRET_KEY":     "Access Key secret",
			"ALICLOUD_SECURITY_TOKEN": "STS Security Token (optional)",
		},
		additional: map[string]string{
			"ALICLOUD_REGION_ID":           "Region ID (Default: cn-hangzhou)",
			"ALICLOUD_LINE":                "Line (Default: default)",
			"ALICLOUD_POLLING_INTERVAL":    "Time between DNS propagation check in seconds (Default: 2)",
			"ALICLOUD_PROPAGATION_TIMEOUT": "Maximum waiting time for DNS propagation in seconds (Default: 60)",
			"ALICLOUD_TTL":                 "The TTL of the TXT record used for the DNS challenge in seconds (Default: 600)",
			"ALICLOUD_HTTP_TIMEOUT":        "API request timeout in seconds (Default: 10)",
		},
		api:      "https://www.alibabacloud.com/help/en/alibaba-cloud-dns/latest/api-alidns-2015-01-09-dir-parsing-records",
		goClient: "https://github.com/alibabacloud-go/alidns-20150109",
	},
	{
		name: "Tencent Cloud DNS",
		code: "tencentcloud",
		credentials: map[string]string{
			"TENCENTCLOUD_SECRET_ID":  "Access key ID",
			"TENCENTCLOUD_SECRET_KEY": "Access Key secret",
		},
		additional: map[string]string{
			"TENCENTCLOUD_SESSION_TOKEN":       "Access Key token",
			"TENCENTCLOUD_REGION":              "Region",
			"TENCENTCLOUD_POLLING_INTERVAL":    "Time between DNS propagation check in seconds (Default: 2)",
			"TENCENTCLOUD_PROPAGATION_TIMEOUT": "Maximum waiting time for DNS propagation in seconds (Default: 60)",
			"TENCENTCLOUD_TTL":                 "The TTL of the TXT record used for the DNS challenge in seconds (Default: 600)",
			"TENCENTCLOUD_HTTP_TIMEOUT":        "API request timeout in seconds (Default: 30)",
		},
		api:      "https://cloud.tencent.com/document/product/1427/56153",
		goClient: "https://github.com/tencentcloud/tencentcloud-sdk-go",
	},
	{
		name: "Cloudflare",
		code: "cloudflare",
		credentials: map[string]string{
			"CF_API_EMAIL":              "Account email",
			"CF_API_KEY":                "API key",
			"CF_DNS_API_TOKEN":          "API token with DNS:Edit permission (since v3.1.0)",
			"CF_ZONE_API_TOKEN":         "API token with Zone:Read permission (since v3.1.0)",
			"CLOUDFLARE_EMAIL":          "Alias to CF_API_EMAIL",
			"CLOUDFLARE_API_KEY":        "Alias to CF_API_KEY",
			"CLOUDFLARE_DNS_API_TOKEN":  "Alias to CF_DNS_API_TOKEN",
			"CLOUDFLARE_ZONE_API_TOKEN": "Alias to CF_ZONE_API_TOKEN",
		},
		additional: map[string]string{
			"CLOUDFLARE_POLLING_INTERVAL":    "Time between DNS propagation check in seconds (Default: 2)",
			"CLOUDFLARE_PROPAGATION_TIMEOUT": "Maximum waiting time for DNS propagation in seconds (Default: 120)",
			"CLOUDFLARE_TTL":                 "The TTL of the TXT record used for the DNS challenge in seconds (Default: 120)",
			"CLOUDFLARE_HTTP_TIMEOUT":        "API request timeout in seconds (Default: )",
			"CLOUDFLARE_BASE_URL":            "API base URL (Default: https://api.cloudflare.com/client/v4)",
		},
		api:      "https://api.cloudflare.com/",
		goClient: "https://github.com/cloudflare/cloudflare-go",
	},
	{
		name: "Azure DNS",
		code: "azuredns",
		credentials: map[string]string{
			"AZURE_CLIENT_ID":               "Client ID",
			"AZURE_CLIENT_SECRET":           "Client secret",
			"AZURE_TENANT_ID":               "Tenant ID",
			"AZURE_CLIENT_CERTIFICATE_PATH": "Client certificate path",
		},
		additional: map[string]string{
			"AZURE_ENVIRONMENT":             "Azure environment, one of: public, usgovernment, and china",
			"AZURE_SUBSCRIPTION_ID":         "DNS zone subscription ID",
			"AZURE_RESOURCE_GROUP":          "DNS zone resource group",
			"AZURE_SERVICEDISCOVERY_FILTER": "Advanced ServiceDiscovery filter using Kusto query condition",
			"AZURE_PRIVATE_ZONE":            "Set to true to use Azure Private DNS Zones and not public",
			"AZURE_ZONE_NAME":               "Zone name to use inside Azure DNS service to add the TXT record in",
			"AZURE_AUTH_METHOD":             "Specify which authentication method to use",
			"AZURE_AUTH_MSI_TIMEOUT":        "Managed Identity timeout duration",
			"AZURE_TTL":                     "The TTL of the TXT record used for the DNS challenge in seconds (Default: 60)",
			"AZURE_POLLING_INTERVAL":        "Time between DNS propagation check in seconds (Default: 2)",
			"AZURE_PROPAGATION_TIMEOUT":     "Maximum waiting time for DNS propagation in seconds (Default: 120)",
		},
		api:      "https://docs.microsoft.com/en-us/go/azure/",
		goClient: "https://github.com/Azure/azure-sdk-for-go",
	},
	{
		name: "Huawei Cloud",
		code: "huaweicloud",
		credentials: map[string]string{
			"HUAWEICLOUD_ACCESS_KEY_ID":     "Access key ID",
			"HUAWEICLOUD_SECRET_ACCESS_KEY": "Access Key secret",
			"HUAWEICLOUD_REGION":            "Region",
		},
		additional: map[string]string{
			"HUAWEICLOUD_POLLING_INTERVAL":    "Time between DNS propagation check in seconds (Default: 2)",
			"HUAWEICLOUD_PROPAGATION_TIMEOUT": "Maximum waiting time for DNS propagation in seconds (Default: 60)",
			"HUAWEICLOUD_TTL":                 "The TTL of the TXT record used for the DNS challenge in seconds (Default: 300)",
			"HUAWEICLOUD_HTTP_TIMEOUT":        "API request timeout in seconds (Default: 30)",
		},
		api:      "https://console-intl.huaweicloud.com/apiexplorer/#/openapi/DNS/doc?locale=en-us",
		goClient: "https://github.com/huaweicloud/huaweicloud-sdk-go-v3",
	},
}
