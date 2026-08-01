package demo

import (
	"context"
	"time"

	certdns "github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// Database rows the fabricated providers need something to hang off.
//
// Written through GORM rather than raw SQL on purpose: DnsCredential.Config is
// declared `serializer:json[aes]`, so the value is encrypted on write with the
// instance's crypto secret. Inserting it directly would store plaintext that
// the application then fails to decrypt.

type credentialSpec struct {
	name     string
	provider string
	code     string
	values   map[string]string
	domains  []domainSpec
}

type domainSpec struct {
	domain      string
	description string
	ddns        bool
}

// demoCredentials covers the vendors whose credential forms the UI can render.
// The secrets are obvious placeholders — nothing here is a real key shape that
// someone might mistake for one.
func demoCredentials() []credentialSpec {
	return []credentialSpec{
		{
			name: "Cloudflare (demo)", provider: "Cloudflare", code: "cloudflare",
			values: map[string]string{
				"CF_DNS_API_TOKEN": "demo-token-not-a-real-credential",
			},
			domains: []domainSpec{
				{domain: "ojbk.me", description: "Primary demo domain", ddns: true},
				{domain: "nginxui.demo", description: "Documentation site"},
			},
		},
		{
			name: "Aliyun DNS (demo)", provider: "Alibaba Cloud DNS", code: "alidns",
			values: map[string]string{
				"ALICLOUD_ACCESS_KEY": "demo-access-key",
				"ALICLOUD_SECRET_KEY": "demo-secret-key",
			},
			domains: []domainSpec{
				{domain: "langgood.com", description: "Sponsor site"},
			},
		},
		{
			name: "Tencent Cloud DNS (demo)", provider: "Tencent Cloud DNS", code: "tencentcloud",
			values: map[string]string{
				"TENCENTCLOUD_SECRET_ID":  "demo-secret-id",
				"TENCENTCLOUD_SECRET_KEY": "demo-secret-key",
			},
			domains: []domainSpec{
				{domain: "example-cdn.net", description: "CDN edge zone"},
			},
		},
	}
}

func ddnsConfig() *model.DDNSConfig {
	last := time.Now().Add(-7 * time.Minute)
	return &model.DDNSConfig{
		Enabled:                   true,
		IntervalSeconds:           300,
		IPVersion:                 "ipv4",
		CleanupConflictingRecords: true,
		LastIPv4:                  "203.0.113.42",
		LastRunAt:                 &last,
	}
}

// seedDNS creates the credential and domain rows, unless they already exist.
func seedDNS(ctx context.Context) error {
	credentials := query.DnsCredential
	domains := query.DnsDomain

	existing, err := credentials.WithContext(ctx).Count()
	if err != nil {
		return err
	}
	if existing > 0 {
		logger.Info("demo: DNS credentials already present, skipping seed")
		return nil
	}

	for _, spec := range demoCredentials() {
		credential := &model.DnsCredential{
			Name:         spec.name,
			Provider:     spec.provider,
			ProviderCode: spec.code,
			Config: &certdns.Config{
				Name: spec.provider,
				Code: spec.code,
				Configuration: &certdns.Configuration{
					Credentials: spec.values,
				},
			},
		}
		if err := credentials.WithContext(ctx).Create(credential); err != nil {
			return err
		}

		for _, d := range spec.domains {
			row := &model.DnsDomain{
				Domain:          d.domain,
				Description:     d.description,
				DnsCredentialID: credential.ID,
			}
			if d.ddns {
				row.DDNSConfig = ddnsConfig()
			}
			if err := domains.WithContext(ctx).Create(row); err != nil {
				return err
			}
		}
	}

	logger.Info("demo: seeded DNS credentials and domains")
	return nil
}

// Seed writes the demo's database rows. Runs after the database is ready, and
// is a no-op outside demo mode.
func Seed(ctx context.Context) {
	if !Enabled() {
		return
	}

	if err := seedDNS(ctx); err != nil {
		// Not fatal: an empty DNS page is worse than a crash loop is worse.
		logger.Errorf("demo: DNS seed failed: %v", err)
	}
}
