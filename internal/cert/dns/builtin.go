package dns

import (
	"context"
	"slices"

	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
)

func init() {
	RegisterSource(builtinSource{})
}

// builtinSource describes the vendors the built-in DNS record management
// talks to. Their credential forms stay in the core because the record
// management reads the very same credential rows. Solving DNS-01 is not part
// of it: that belongs to a plugin.
type builtinSource struct{}

func (builtinSource) Providers() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(builtinProviders))
	for _, p := range builtinProviders {
		out = append(out, ProviderInfo{
			Name:             p.name,
			Code:             p.code,
			Links:            &ProviderLinks{API: p.api},
			Form:             p.form(),
			RecordManagement: true,
		})
	}
	return out
}

func (builtinSource) NewChallengeProvider(context.Context, string, Configuration, map[string]any) (challenge.Provider, []dns01.ChallengeOption, func(), error) {
	return nil, nil, func() {}, ErrProviderNotFound
}

// builtinProvider is one record management vendor. The keys mirror lego's
// provider catalog so a credential entered here keeps working when the
// DNS-01 plugin picks it up.
type builtinProvider struct {
	name    string
	code    string
	fields  []FormField
	methods []FormMethod
	api     string
}

// msg marks a string for extraction into the frontend catalog.
func msg(c *translation.Container) string {
	return c.Message
}

// form returns a copy callers may change freely.
func (p builtinProvider) form() *Form {
	form := &Form{Fields: slices.Clone(p.fields)}
	for _, m := range p.methods {
		form.Methods = append(form.Methods, FormMethod{
			Name:        m.Name,
			Recommended: m.Recommended,
			Fields:      slices.Clone(m.Fields),
		})
	}
	return form
}

var builtinProviders = []builtinProvider{
	{
		name: "Alibaba Cloud DNS",
		code: "alidns",
		fields: []FormField{
			{Key: "ALICLOUD_ACCESS_KEY", Group: FieldGroupCredential, Label: msg(translation.C("AccessKey ID"))},
			{Key: "ALICLOUD_SECRET_KEY", Group: FieldGroupCredential, Label: msg(translation.C("AccessKey secret")), Secret: true},
			{Key: "ALICLOUD_SECURITY_TOKEN", Group: FieldGroupCredential, Label: msg(translation.C("STS security token")), Optional: true, Secret: true},
			{Key: "ALICLOUD_RAM_ROLE", Group: FieldGroupCredential, Label: msg(translation.C("Instance RAM role")), Link: "https://www.alibabacloud.com/help/en/ecs/user-guide/attach-an-instance-ram-role-to-an-ecs-instance"},
			{Key: "ALICLOUD_REGION_ID", Group: FieldGroupSetting, Label: msg(translation.C("Region ID")), Default: "cn-hangzhou"},
			{Key: "ALICLOUD_LINE", Group: FieldGroupSetting, Label: msg(translation.C("DNS line")), Default: "default"},
			{Key: "ALICLOUD_TTL", Group: FieldGroupSetting, Label: msg(translation.C("TXT record TTL")), Default: "600", Unit: FieldUnitSeconds},
			{Key: "ALICLOUD_HTTP_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("API timeout")), Default: "10", Unit: FieldUnitSeconds},
			{Key: "ALICLOUD_POLLING_INTERVAL", Group: FieldGroupSetting, Label: msg(translation.C("Check interval")), Default: "2", Unit: FieldUnitSeconds},
			{Key: "ALICLOUD_PROPAGATION_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("Maximum wait")), Default: "60", Unit: FieldUnitSeconds},
		},
		methods: []FormMethod{
			{Name: msg(translation.C("AccessKey")), Recommended: true, Fields: []string{"ALICLOUD_ACCESS_KEY", "ALICLOUD_SECRET_KEY", "ALICLOUD_SECURITY_TOKEN"}},
			{Name: msg(translation.C("Instance RAM role")), Fields: []string{"ALICLOUD_RAM_ROLE"}},
		},
		api: "https://www.alibabacloud.com/help/en/alibaba-cloud-dns/latest/api-alidns-2015-01-09-dir-parsing-records",
	},
	{
		name: "Tencent Cloud DNS",
		code: "tencentcloud",
		fields: []FormField{
			{Key: "TENCENTCLOUD_SECRET_ID", Group: FieldGroupCredential, Label: msg(translation.C("SecretId"))},
			{Key: "TENCENTCLOUD_SECRET_KEY", Group: FieldGroupCredential, Label: msg(translation.C("SecretKey")), Secret: true},
			{Key: "TENCENTCLOUD_SESSION_TOKEN", Group: FieldGroupSetting, Label: msg(translation.C("Session token")), Secret: true},
			{Key: "TENCENTCLOUD_REGION", Group: FieldGroupSetting, Label: msg(translation.C("Region"))},
			{Key: "TENCENTCLOUD_TTL", Group: FieldGroupSetting, Label: msg(translation.C("TXT record TTL")), Default: "600", Unit: FieldUnitSeconds},
			{Key: "TENCENTCLOUD_HTTP_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("API timeout")), Default: "30", Unit: FieldUnitSeconds},
			{Key: "TENCENTCLOUD_POLLING_INTERVAL", Group: FieldGroupSetting, Label: msg(translation.C("Check interval")), Default: "2", Unit: FieldUnitSeconds},
			{Key: "TENCENTCLOUD_PROPAGATION_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("Maximum wait")), Default: "60", Unit: FieldUnitSeconds},
		},
		api: "https://cloud.tencent.com/document/product/1427/56153",
	},
	{
		name: "Cloudflare",
		code: "cloudflare",
		fields: []FormField{
			{Key: "CF_DNS_API_TOKEN", Group: FieldGroupCredential, Label: msg(translation.C("API token")), Help: msg(translation.C("Needs the DNS edit permission.")), Secret: true},
			{Key: "CF_ZONE_API_TOKEN", Group: FieldGroupCredential, Label: msg(translation.C("Zone read token")), Help: msg(translation.C("Needs the zone read permission. Leave empty to use the API token.")), Optional: true, Secret: true},
			{Key: "CF_API_EMAIL", Group: FieldGroupCredential, Label: msg(translation.C("Account email"))},
			{Key: "CF_API_KEY", Group: FieldGroupCredential, Label: msg(translation.C("Global API key")), Secret: true},
			{Key: "CLOUDFLARE_BASE_URL", Group: FieldGroupSetting, Label: msg(translation.C("API base URL")), Default: "https://api.cloudflare.com/client/v4"},
			{Key: "CLOUDFLARE_TTL", Group: FieldGroupSetting, Label: msg(translation.C("TXT record TTL")), Default: "120", Unit: FieldUnitSeconds},
			{Key: "CLOUDFLARE_HTTP_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("API timeout")), Unit: FieldUnitSeconds},
			{Key: "CLOUDFLARE_POLLING_INTERVAL", Group: FieldGroupSetting, Label: msg(translation.C("Check interval")), Default: "2", Unit: FieldUnitSeconds},
			{Key: "CLOUDFLARE_PROPAGATION_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("Maximum wait")), Default: "120", Unit: FieldUnitSeconds},
		},
		methods: []FormMethod{
			{Name: msg(translation.C("API token")), Recommended: true, Fields: []string{"CF_DNS_API_TOKEN", "CF_ZONE_API_TOKEN"}},
			{Name: msg(translation.C("Global API key")), Fields: []string{"CF_API_EMAIL", "CF_API_KEY"}},
		},
		api: "https://api.cloudflare.com/",
	},
	{
		name: "Azure DNS",
		code: "azuredns",
		fields: []FormField{
			{Key: "AZURE_TENANT_ID", Group: FieldGroupCredential, Label: msg(translation.C("Tenant ID"))},
			{Key: "AZURE_CLIENT_ID", Group: FieldGroupCredential, Label: msg(translation.C("Client ID"))},
			{Key: "AZURE_CLIENT_SECRET", Group: FieldGroupCredential, Label: msg(translation.C("Client secret")), Secret: true},
			{Key: "AZURE_CLIENT_CERTIFICATE_PATH", Group: FieldGroupCredential, Label: msg(translation.C("Client certificate path"))},
			{Key: "AZURE_SUBSCRIPTION_ID", Group: FieldGroupSetting, Label: msg(translation.C("Subscription ID"))},
			{Key: "AZURE_RESOURCE_GROUP", Group: FieldGroupSetting, Label: msg(translation.C("Resource group"))},
			{Key: "AZURE_ZONE_NAME", Group: FieldGroupSetting, Label: msg(translation.C("Zone name"))},
			{Key: "AZURE_ENVIRONMENT", Group: FieldGroupSetting, Label: msg(translation.C("Azure environment")), Help: msg(translation.C("One of public, usgovernment and china."))},
			{Key: "AZURE_PRIVATE_ZONE", Group: FieldGroupSetting, Label: msg(translation.C("Private zone")), Help: msg(translation.C("Set to true to use a private DNS zone."))},
			{Key: "AZURE_AUTH_METHOD", Group: FieldGroupSetting, Label: msg(translation.C("Authentication method"))},
			{Key: "AZURE_AUTH_MSI_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("Managed identity timeout"))},
			{Key: "AZURE_SERVICEDISCOVERY_FILTER", Group: FieldGroupSetting, Label: msg(translation.C("Service discovery filter"))},
			{Key: "AZURE_TTL", Group: FieldGroupSetting, Label: msg(translation.C("TXT record TTL")), Default: "60", Unit: FieldUnitSeconds},
			{Key: "AZURE_POLLING_INTERVAL", Group: FieldGroupSetting, Label: msg(translation.C("Check interval")), Default: "2", Unit: FieldUnitSeconds},
			{Key: "AZURE_PROPAGATION_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("Maximum wait")), Default: "120", Unit: FieldUnitSeconds},
		},
		methods: []FormMethod{
			{Name: msg(translation.C("Client secret")), Recommended: true, Fields: []string{"AZURE_TENANT_ID", "AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET"}},
			{Name: msg(translation.C("Client certificate")), Fields: []string{"AZURE_TENANT_ID", "AZURE_CLIENT_ID", "AZURE_CLIENT_CERTIFICATE_PATH"}},
		},
		api: "https://docs.microsoft.com/en-us/go/azure/",
	},
	{
		name: "Huawei Cloud",
		code: "huaweicloud",
		fields: []FormField{
			{Key: "HUAWEICLOUD_ACCESS_KEY_ID", Group: FieldGroupCredential, Label: msg(translation.C("Access key ID"))},
			{Key: "HUAWEICLOUD_SECRET_ACCESS_KEY", Group: FieldGroupCredential, Label: msg(translation.C("Secret access key")), Secret: true},
			{Key: "HUAWEICLOUD_REGION", Group: FieldGroupCredential, Label: msg(translation.C("Region"))},
			{Key: "HUAWEICLOUD_TTL", Group: FieldGroupSetting, Label: msg(translation.C("TXT record TTL")), Default: "300", Unit: FieldUnitSeconds},
			{Key: "HUAWEICLOUD_HTTP_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("API timeout")), Default: "30", Unit: FieldUnitSeconds},
			{Key: "HUAWEICLOUD_POLLING_INTERVAL", Group: FieldGroupSetting, Label: msg(translation.C("Check interval")), Default: "2", Unit: FieldUnitSeconds},
			{Key: "HUAWEICLOUD_PROPAGATION_TIMEOUT", Group: FieldGroupSetting, Label: msg(translation.C("Maximum wait")), Default: "60", Unit: FieldUnitSeconds},
		},
		api: "https://console-intl.huaweicloud.com/apiexplorer/#/openapi/DNS/doc?locale=en-us",
	},
}
