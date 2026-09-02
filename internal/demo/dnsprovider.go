package demo

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

// In-memory DNS provider.
//
// Registered over the real vendor codes rather than under a synthetic "demo"
// one. That matters: the frontend validates against a fixed allowlist
// (app/src/constants/dns_providers.ts) and the site pickers silently drop
// domains whose provider code is unknown, so inventing a code would leave the
// DNS screens looking broken in a subtler way than leaving them empty.
//
// The registry is a plain map and RegisterProvider is last-write-wins, so
// installing these after the real providers' init() has run replaces them.

// demoProviderCodes are the vendors the UI knows how to render credential
// forms for.
var demoProviderCodes = []string{
	"cloudflare", "alidns", "tencentcloud", "huaweicloud", "azuredns",
}

// zoneStore holds fabricated records, keyed by "provider|domain". Shared across
// every provider instance so an edit made through the UI is still there when
// the page is reloaded.
type zoneStore struct {
	mu      sync.Mutex
	records map[string][]dns.Record
	nextID  int
}

var zones = &zoneStore{records: map[string][]dns.Record{}, nextID: 1000}

// seedRecords is the starting record set for a zone. Deterministic per domain,
// so two instances in the same container agree and a restart does not reshuffle
// the table.
func seedRecords(domain string) []dns.Record {
	base := seed("dns", domain)
	ttl := func(n int) int { return []int{300, 600, 3600}[n%3] }

	specs := []struct {
		kind, name, content string
	}{
		{"A", "@", fmt.Sprintf("203.0.113.%d", rangeInt(base, 10, 60))},
		{"A", "www", fmt.Sprintf("203.0.113.%d", rangeInt(base>>4, 10, 60))},
		{"AAAA", "@", "2001:db8::1"},
		{"CNAME", "cdn", "d1a2b3c4.cloudfront.net"},
		{"CNAME", "docs", "hosting.example-pages.net"},
		{"MX", "@", "10 mail.example-mx.net"},
		{"TXT", "@", "v=spf1 include:_spf.example-mx.net ~all"},
		{"TXT", "_dmarc", "v=DMARC1; p=quarantine; rua=mailto:dmarc@" + domain},
		{"TXT", "_acme-challenge", "3Zx9Qk_placeholder_token_for_demo_only"},
		{"SRV", "_sip._tcp", "10 60 5060 sip.example-voice.net"},
		{"NS", "sub", "ns1.example-dns.net"},
		{"CAA", "@", "0 issue \"letsencrypt.org\""},
	}

	out := make([]dns.Record, 0, len(specs))
	for i, spec := range specs {
		out = append(out, dns.Record{
			ID:      fmt.Sprintf("demo-%s-%d", strings.ReplaceAll(domain, ".", "-"), i+1),
			Type:    spec.kind,
			Name:    spec.name,
			Content: spec.content,
			TTL:     ttl(i),
			Comment: "demo record",
		})
	}
	return out
}

func (z *zoneStore) key(provider, domain string) string {
	return provider + "|" + strings.ToLower(domain)
}

func (z *zoneStore) list(provider, domain string) []dns.Record {
	z.mu.Lock()
	defer z.mu.Unlock()

	k := z.key(provider, domain)
	if _, ok := z.records[k]; !ok {
		z.records[k] = seedRecords(domain)
	}
	return slices.Clone(z.records[k])
}

func (z *zoneStore) add(provider, domain string, record dns.Record) dns.Record {
	z.mu.Lock()
	defer z.mu.Unlock()

	k := z.key(provider, domain)
	if _, ok := z.records[k]; !ok {
		z.records[k] = seedRecords(domain)
	}
	z.nextID++
	record.ID = fmt.Sprintf("demo-new-%d", z.nextID)
	z.records[k] = append(z.records[k], record)
	return record
}

func (z *zoneStore) update(provider, domain, id string, record dns.Record) (dns.Record, bool) {
	z.mu.Lock()
	defer z.mu.Unlock()

	k := z.key(provider, domain)
	for i, existing := range z.records[k] {
		if existing.ID == id {
			record.ID = id
			z.records[k][i] = record
			return record, true
		}
	}
	return dns.Record{}, false
}

func (z *zoneStore) remove(provider, domain, id string) bool {
	z.mu.Lock()
	defer z.mu.Unlock()

	k := z.key(provider, domain)
	for i, existing := range z.records[k] {
		if existing.ID == id {
			z.records[k] = slices.Delete(z.records[k], i, i+1)
			return true
		}
	}
	return false
}

// dnsProvider serves one credential's zones.
type dnsProvider struct {
	code string
}

var (
	_ dns.Provider           = (*dnsProvider)(nil)
	_ dns.RecordLineProvider = (*dnsProvider)(nil)
)

func toRecord(input dns.RecordInput) dns.Record {
	record := dns.Record{
		Type:     input.Type,
		Name:     input.Name,
		Content:  input.Content,
		TTL:      input.TTL,
		Priority: input.Priority,
		Weight:   input.Weight,
		Proxied:  input.Proxied,
		Comment:  input.Comment,
	}
	if input.Line != nil {
		record.Line = *input.Line
	}
	return record
}

func (p *dnsProvider) ListRecords(_ context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	records := zones.list(p.code, domain)
	if filter.Type == "" && filter.Name == "" {
		return records, nil
	}

	out := records[:0:0]
	for _, r := range records {
		if filter.Type != "" && !strings.EqualFold(r.Type, filter.Type) {
			continue
		}
		if filter.Name != "" && !strings.EqualFold(r.Name, filter.Name) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (p *dnsProvider) CreateRecord(_ context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	return zones.add(p.code, domain, toRecord(input)), nil
}

func (p *dnsProvider) UpdateRecord(_ context.Context, domain, recordID string, input dns.RecordInput) (dns.Record, error) {
	record, ok := zones.update(p.code, domain, recordID, toRecord(input))
	if !ok {
		return dns.Record{}, fmt.Errorf("demo: record %s not found in %s", recordID, domain)
	}
	return record, nil
}

func (p *dnsProvider) DeleteRecord(_ context.Context, domain, recordID string) error {
	if !zones.remove(p.code, domain, recordID) {
		return fmt.Errorf("demo: record %s not found in %s", recordID, domain)
	}
	return nil
}

// recordLines mirror the shape real Chinese providers return; Cloudflare and
// Azure have no notion of resolution lines, so they get none.
func (p *dnsProvider) ListRecordLines(_ context.Context, _ string) ([]dns.RecordLine, error) {
	switch p.code {
	case "alidns", "tencentcloud", "huaweicloud":
		return []dns.RecordLine{
			{Code: "default", Name: "默认", DisplayName: "默认"},
			{Code: "telecom", Name: "电信", DisplayName: "电信", FatherCode: "default"},
			{Code: "unicom", Name: "联通", DisplayName: "联通", FatherCode: "default"},
			{Code: "mobile", Name: "移动", DisplayName: "移动", FatherCode: "default"},
			{Code: "oversea", Name: "境外", DisplayName: "境外", FatherCode: "default"},
		}, nil
	default:
		return nil, nil
	}
}

// installDNSProviders replaces the real vendor factories with in-memory ones.
func installDNSProviders() {
	for _, code := range demoProviderCodes {
		dns.RegisterProvider(code, func(cred *dns.Credential) (dns.Provider, error) {
			provider := cred.Code
			if provider == "" {
				provider = cred.Provider
			}
			return &dnsProvider{code: strings.ToLower(provider)}, nil
		})
	}
}
