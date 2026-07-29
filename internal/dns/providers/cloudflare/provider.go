package cloudflare

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	cfdns "github.com/cloudflare/cloudflare-go/v7/dns"
	cfopt "github.com/cloudflare/cloudflare-go/v7/option"
	cfzones "github.com/cloudflare/cloudflare-go/v7/zones"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const defaultTimeout = 10 * time.Second

type provider struct {
	records   *cfdns.RecordService
	zones     *cfzones.ZoneService
	zoneCache sync.Map
}

func init() {
	dns.RegisterProvider("cloudflare", newProvider)
}

func newProvider(cred *dns.Credential) (dns.Provider, error) {
	httpClient := &http.Client{Timeout: defaultTimeout}

	opts := []cfopt.RequestOption{
		cfopt.WithHTTPClient(httpClient),
	}

	if baseURL := firstNonEmpty(
		cred.Additional["CLOUDFLARE_BASE_URL"],
		cred.Additional["CF_BASE_URL"],
	); baseURL != "" {
		opts = append(opts, cfopt.WithBaseURL(baseURL))
	}

	token := firstNonEmpty(
		cred.Values["CLOUDFLARE_DNS_API_TOKEN"],
		cred.Values["CF_DNS_API_TOKEN"],
		cred.Values["CLOUDFLARE_API_TOKEN"],
		cred.Values["CF_API_TOKEN"],
	)

	if token != "" {
		opts = append(opts, cfopt.WithAPIToken(token))
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
		opts = append(opts, cfopt.WithAPIKey(key), cfopt.WithAPIEmail(email))
	}

	return &provider{
		records: cfdns.NewRecordService(opts...),
		zones:   cfzones.NewZoneService(opts...),
	}, nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	zoneID, err := p.zoneID(ctx, domain)
	if err != nil {
		return nil, err
	}

	params := cfdns.RecordListParams{}
	setField(&params.ZoneID.Value, &params.ZoneID.Present, zoneID)

	recordType := strings.ToUpper(strings.TrimSpace(filter.Type))
	if recordType != "" {
		setField(&params.Type.Value, &params.Type.Present, cfdns.RecordListParamsType(recordType))
	}

	if name := strings.TrimSpace(filter.Name); name != "" {
		setField(&params.Name.Value.Exact.Value, &params.Name.Value.Exact.Present, buildFQDN(domain, name))
		params.Name.Present = true
	}

	pager := p.records.ListAutoPaging(ctx, params)

	result := make([]dns.Record, 0)
	for pager.Next() {
		record := pager.Current()
		proxied := record.Proxied
		result = append(result, dns.Record{
			ID:       record.ID,
			Type:     string(record.Type),
			Name:     toRelativeName(record.Name, domain),
			Content:  record.Content,
			TTL:      int(record.TTL),
			Priority: toOptionalPriority(record.Priority),
			Proxied:  &proxied,
			Comment:  record.Comment,
		})
	}

	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("cloudflare: list records: %w", err)
	}

	return result, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	zoneID, err := p.zoneID(ctx, domain)
	if err != nil {
		return dns.Record{}, err
	}

	body := cfdns.RecordNewParamsBody{}
	setField(&body.Type.Value, &body.Type.Present, cfdns.RecordNewParamsBodyType(strings.ToUpper(strings.TrimSpace(input.Type))))
	setField(&body.Name.Value, &body.Name.Present, buildFQDN(domain, input.Name))
	setField(&body.Content.Value, &body.Content.Present, strings.TrimSpace(input.Content))
	setField(&body.TTL.Value, &body.TTL.Present, cfdns.TTL(normalizeTTL(input.TTL)))

	if input.Proxied != nil {
		setField(&body.Proxied.Value, &body.Proxied.Present, *input.Proxied)
	}

	if input.Priority != nil {
		value := float64(max(*input.Priority, 0))
		setField(&body.Priority.Value, &body.Priority.Present, value)
	}

	if input.Comment != "" {
		setField(&body.Comment.Value, &body.Comment.Present, input.Comment)
	}

	params := cfdns.RecordNewParams{Body: body}
	setField(&params.ZoneID.Value, &params.ZoneID.Present, zoneID)
	record, err := p.records.New(ctx, params)
	if err != nil {
		return dns.Record{}, fmt.Errorf("cloudflare: create record: %w", err)
	}

	return dns.Record{
		ID:       record.ID,
		Type:     string(record.Type),
		Name:     toRelativeName(record.Name, domain),
		Content:  record.Content,
		TTL:      int(record.TTL),
		Priority: toOptionalPriority(record.Priority),
		Proxied:  boolPtr(record.Proxied),
		Comment:  record.Comment,
	}, nil
}

func (p *provider) UpdateRecord(ctx context.Context, domain string, recordID string, input dns.RecordInput) (dns.Record, error) {
	zoneID, err := p.zoneID(ctx, domain)
	if err != nil {
		return dns.Record{}, err
	}

	body := cfdns.RecordUpdateParamsBody{}
	setField(&body.Type.Value, &body.Type.Present, cfdns.RecordUpdateParamsBodyType(strings.ToUpper(strings.TrimSpace(input.Type))))
	setField(&body.Name.Value, &body.Name.Present, buildFQDN(domain, input.Name))
	setField(&body.Content.Value, &body.Content.Present, strings.TrimSpace(input.Content))
	setField(&body.TTL.Value, &body.TTL.Present, cfdns.TTL(normalizeTTL(input.TTL)))

	if input.Proxied != nil {
		setField(&body.Proxied.Value, &body.Proxied.Present, *input.Proxied)
	}

	if input.Priority != nil {
		value := float64(max(*input.Priority, 0))
		setField(&body.Priority.Value, &body.Priority.Present, value)
	}

	// Always set comment, including empty string to allow clearing
	setField(&body.Comment.Value, &body.Comment.Present, input.Comment)

	params := cfdns.RecordUpdateParams{Body: body}
	setField(&params.ZoneID.Value, &params.ZoneID.Present, zoneID)
	record, err := p.records.Update(ctx, recordID, params)
	if err != nil {
		return dns.Record{}, fmt.Errorf("cloudflare: update record: %w", err)
	}

	return dns.Record{
		ID:       record.ID,
		Type:     string(record.Type),
		Name:     toRelativeName(record.Name, domain),
		Content:  record.Content,
		TTL:      int(record.TTL),
		Priority: toOptionalPriority(record.Priority),
		Proxied:  boolPtr(record.Proxied),
		Comment:  record.Comment,
	}, nil
}

func (p *provider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	zoneID, err := p.zoneID(ctx, domain)
	if err != nil {
		return err
	}

	params := cfdns.RecordDeleteParams{}
	setField(&params.ZoneID.Value, &params.ZoneID.Present, zoneID)
	if _, err := p.records.Delete(ctx, recordID, params); err != nil {
		return fmt.Errorf("cloudflare: delete record: %w", err)
	}

	return nil
}

func (p *provider) zoneID(ctx context.Context, domain string) (string, error) {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if zoneID, ok := p.zoneCache.Load(normalized); ok {
		return zoneID.(string), nil
	}

	params := cfzones.ZoneListParams{}
	setField(&params.Name.Value, &params.Name.Present, normalized)
	pager := p.zones.ListAutoPaging(ctx, params)
	for pager.Next() {
		zone := pager.Current()
		if strings.EqualFold(strings.TrimSuffix(zone.Name, "."), normalized) {
			p.zoneCache.Store(normalized, zone.ID)
			return zone.ID, nil
		}
	}

	if err := pager.Err(); err != nil {
		return "", fmt.Errorf("cloudflare: resolve zone id: %w", err)
	}

	return "", fmt.Errorf("cloudflare: resolve zone id: not found")
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

func toOptionalPriority(value float64) *int {
	if value == 0 {
		return nil
	}
	v := int(value)
	return &v
}

func normalizeTTL(ttl int) int {
	if ttl <= 0 {
		return 1
	}
	return ttl
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func boolPtr(v bool) *bool {
	return &v
}

func setField[T any](target *T, present *bool, value T) {
	*target = value
	*present = true
}
