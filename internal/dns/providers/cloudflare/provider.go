package cloudflare

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	cf "github.com/cloudflare/cloudflare-go"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const defaultTimeout = 10 * time.Second

type provider struct {
	client    *cf.API
	zoneCache sync.Map
}

func init() {
	dns.RegisterProvider("cloudflare", newProvider)
}

func newProvider(cred *dns.Credential) (dns.Provider, error) {
	httpClient := &http.Client{Timeout: defaultTimeout}

	opts := []cf.Option{
		cf.HTTPClient(httpClient),
	}

	if baseURL := firstNonEmpty(
		cred.Additional["CLOUDFLARE_BASE_URL"],
		cred.Additional["CF_BASE_URL"],
	); baseURL != "" {
		opts = append(opts, cf.BaseURL(baseURL))
	}

	token := firstNonEmpty(
		cred.Values["CLOUDFLARE_DNS_API_TOKEN"],
		cred.Values["CF_DNS_API_TOKEN"],
		cred.Values["CLOUDFLARE_API_TOKEN"],
		cred.Values["CF_API_TOKEN"],
	)

	var (
		api *cf.API
		err error
	)

	if token != "" {
		api, err = cf.NewWithAPIToken(token, opts...)
	} else {
		email := firstNonEmpty(
			cred.Values["CLOUDFLARE_EMAIL"],
			cred.Values["CF_API_EMAIL"],
		)
		key := firstNonEmpty(
			cred.Values["CLOUDFLARE_API_KEY"],
			cred.Values["CF_API_KEY"],
		)
		if email == "" || key == "" {
			return nil, fmt.Errorf("cloudflare: missing API credentials")
		}
		api, err = cf.New(key, email, opts...)
	}

	if err != nil {
		return nil, fmt.Errorf("cloudflare: %w", err)
	}

	return &provider{
		client: api,
	}, nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	zoneID, err := p.zoneID(domain)
	if err != nil {
		return nil, err
	}

	params := cf.ListDNSRecordsParams{
		Type: strings.ToUpper(strings.TrimSpace(filter.Type)),
	}

	if params.Type == "" {
		params.Type = ""
	}

	if filter.Name != "" {
		params.Name = buildFQDN(domain, filter.Name)
	}

	records, _, err := p.client.ListDNSRecords(ctx, cf.ZoneIdentifier(zoneID), params)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: list records: %w", err)
	}

	result := make([]dns.Record, 0, len(records))
	for _, record := range records {
		result = append(result, dns.Record{
			ID:       record.ID,
			Type:     record.Type,
			Name:     toRelativeName(record.Name, domain),
			Content:  record.Content,
			TTL:      record.TTL,
			Priority: toOptionalInt(record.Priority),
			Proxied:  record.Proxied,
		})
	}

	return result, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	zoneID, err := p.zoneID(domain)
	if err != nil {
		return dns.Record{}, err
	}

	params := cf.CreateDNSRecordParams{
		Type:    strings.ToUpper(strings.TrimSpace(input.Type)),
		Name:    buildFQDN(domain, input.Name),
		Content: strings.TrimSpace(input.Content),
		TTL:     input.TTL,
		Proxied: input.Proxied,
	}

	if input.Priority != nil {
		value := uint16(max(*input.Priority, 0))
		params.Priority = &value
	}

	record, err := p.client.CreateDNSRecord(ctx, cf.ZoneIdentifier(zoneID), params)
	if err != nil {
		return dns.Record{}, fmt.Errorf("cloudflare: create record: %w", err)
	}

	return dns.Record{
		ID:       record.ID,
		Type:     record.Type,
		Name:     toRelativeName(record.Name, domain),
		Content:  record.Content,
		TTL:      record.TTL,
		Priority: toOptionalInt(record.Priority),
		Proxied:  record.Proxied,
	}, nil
}

func (p *provider) UpdateRecord(ctx context.Context, domain string, recordID string, input dns.RecordInput) (dns.Record, error) {
	zoneID, err := p.zoneID(domain)
	if err != nil {
		return dns.Record{}, err
	}

	params := cf.UpdateDNSRecordParams{
		ID:      recordID,
		Type:    strings.ToUpper(strings.TrimSpace(input.Type)),
		Name:    buildFQDN(domain, input.Name),
		Content: strings.TrimSpace(input.Content),
		TTL:     input.TTL,
		Proxied: input.Proxied,
	}

	if input.Priority != nil {
		value := uint16(max(*input.Priority, 0))
		params.Priority = &value
	}

	record, err := p.client.UpdateDNSRecord(ctx, cf.ZoneIdentifier(zoneID), params)
	if err != nil {
		return dns.Record{}, fmt.Errorf("cloudflare: update record: %w", err)
	}

	return dns.Record{
		ID:       record.ID,
		Type:     record.Type,
		Name:     toRelativeName(record.Name, domain),
		Content:  record.Content,
		TTL:      record.TTL,
		Priority: toOptionalInt(record.Priority),
		Proxied:  record.Proxied,
	}, nil
}

func (p *provider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	zoneID, err := p.zoneID(domain)
	if err != nil {
		return err
	}

	if err := p.client.DeleteDNSRecord(ctx, cf.ZoneIdentifier(zoneID), recordID); err != nil {
		return fmt.Errorf("cloudflare: delete record: %w", err)
	}

	return nil
}

func (p *provider) zoneID(domain string) (string, error) {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if zoneID, ok := p.zoneCache.Load(normalized); ok {
		return zoneID.(string), nil
	}

	id, err := p.client.ZoneIDByName(normalized)
	if err != nil {
		return "", fmt.Errorf("cloudflare: resolve zone id: %w", err)
	}

	p.zoneCache.Store(normalized, id)
	return id, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func buildFQDN(domain, name string) string {
	name = strings.TrimSpace(name)
	domain = strings.TrimSuffix(strings.TrimSpace(domain), ".")
	if name == "" || name == "@" {
		return domain
	}
	if strings.HasSuffix(name, "."+domain) {
		return name
	}
	if name == domain {
		return domain
	}
	return name + "." + domain
}

func toRelativeName(fqdn, domain string) string {
	fqdn = strings.TrimSuffix(strings.TrimSpace(fqdn), ".")
	domain = strings.TrimSuffix(strings.TrimSpace(domain), ".")
	if fqdn == domain {
		return "@"
	}
	if strings.HasSuffix(fqdn, "."+domain) {
		return strings.TrimSuffix(fqdn, "."+domain)
	}
	return fqdn
}

func toOptionalInt(value *uint16) *int {
	if value == nil {
		return nil
	}
	v := int(*value)
	return &v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
