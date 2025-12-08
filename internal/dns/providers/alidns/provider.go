package alidns

import (
	"context"
	"fmt"
	"strings"
	"time"

	aliclient "github.com/alibabacloud-go/alidns-20150109/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	utilruntime "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/dara"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const (
	defaultRegion   = "cn-hangzhou"
	defaultLineName = "default"
)

type provider struct {
	client *aliclient.Client
}

func init() {
	dns.RegisterProvider("alidns", newProvider)
}

func newProvider(cred *dns.Credential) (dns.Provider, error) {
	accessKey := firstNonEmpty(
		cred.Values["ALICLOUD_ACCESS_KEY"],
		cred.Values["ALICLOUD_ACCESS_KEY_ID"],
	)
	secretKey := firstNonEmpty(
		cred.Values["ALICLOUD_SECRET_KEY"],
		cred.Values["ALICLOUD_ACCESS_KEY_SECRET"],
	)

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("alidns: missing access key or secret")
	}

	region := firstNonEmpty(
		cred.Values["ALICLOUD_REGION_ID"],
		cred.Additional["ALICLOUD_REGION_ID"],
		defaultRegion,
	)

	timeout := int(defaultTimeout().Milliseconds())

	cfg := new(openapi.Config).
		SetAccessKeyId(accessKey).
		SetAccessKeySecret(secretKey).
		SetRegionId(region).
		SetReadTimeout(timeout).
		SetConnectTimeout(timeout)

	if token := firstNonEmpty(
		cred.Values["ALICLOUD_SECURITY_TOKEN"],
		cred.Additional["ALICLOUD_SECURITY_TOKEN"],
	); token != "" {
		cfg = cfg.SetSecurityToken(token)
	}

	client, err := aliclient.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("alidns: new client: %w", err)
	}

	return &provider{client: client}, nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	request := &aliclient.DescribeDomainRecordsRequest{
		DomainName: dara.String(domain),
		PageSize:   dara.Int64(500),
	}

	if filter.Name != "" {
		request.RRKeyWord = dara.String(filter.Name)
	}

	if filter.Type != "" {
		request.TypeKeyWord = dara.String(strings.ToUpper(filter.Type))
	}

	response, err := p.client.DescribeDomainRecordsWithOptions(request, runtimeOptions(ctx))
	if err != nil {
		return nil, fmt.Errorf("alidns: list records: %w", err)
	}

	if response.Body == nil || response.Body.DomainRecords == nil {
		return []dns.Record{}, nil
	}

	items := response.Body.DomainRecords.Record
	result := make([]dns.Record, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		record := dns.Record{
			ID:      stringValue(item.RecordId),
			Type:    stringValue(item.Type),
			Name:    rrToName(stringValue(item.RR)),
			Content: stringValue(item.Value),
			TTL:     int(int64Value(item.TTL)),
			Weight:  intPointerFrom32(item.Weight),
		}
		if item.Priority != nil {
			value := int(int64Value(item.Priority))
			record.Priority = &value
		}
		result = append(result, record)
	}

	return result, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	request := &aliclient.AddDomainRecordRequest{
		DomainName: dara.String(domain),
		Type:       dara.String(strings.ToUpper(strings.TrimSpace(input.Type))),
		RR:         dara.String(rrFromName(input.Name)),
		Value:      dara.String(strings.TrimSpace(input.Content)),
		TTL:        dara.Int64(int64(input.TTL)),
		Line:       dara.String(defaultLineName),
	}

	if input.Priority != nil {
		request.Priority = dara.Int64(int64(*input.Priority))
	}

	response, err := p.client.AddDomainRecordWithOptions(request, runtimeOptions(ctx))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "DomainRecordDuplicate") {
			return dns.Record{}, fmt.Errorf("alidns: a DNS record with the same name, type, and line already exists")
		}
		return dns.Record{}, fmt.Errorf("alidns: add record: %w", err)
	}

	recordID := ""
	if response.Body != nil && response.Body.RecordId != nil {
		recordID = *response.Body.RecordId
	}

	if recordID == "" {
		return dns.Record{}, fmt.Errorf("alidns: empty record id")
	}

	return p.describeRecord(ctx, recordID)
}

func (p *provider) UpdateRecord(ctx context.Context, _ string, recordID string, input dns.RecordInput) (dns.Record, error) {
	request := &aliclient.UpdateDomainRecordRequest{
		RecordId: dara.String(recordID),
		Type:     dara.String(strings.ToUpper(strings.TrimSpace(input.Type))),
		RR:       dara.String(rrFromName(input.Name)),
		Value:    dara.String(strings.TrimSpace(input.Content)),
		TTL:      dara.Int64(int64(input.TTL)),
		Line:     dara.String(defaultLineName),
	}

	if input.Priority != nil {
		request.Priority = dara.Int64(int64(*input.Priority))
	}

	if _, err := p.client.UpdateDomainRecordWithOptions(request, runtimeOptions(ctx)); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "DomainRecordDuplicate") {
			// If the record already exists with the same values during update,
			// treat it as success and return the current record
			return p.describeRecord(ctx, recordID)
		}
		return dns.Record{}, fmt.Errorf("alidns: update record: %w", err)
	}

	return p.describeRecord(ctx, recordID)
}

func (p *provider) DeleteRecord(ctx context.Context, _ string, recordID string) error {
	request := &aliclient.DeleteDomainRecordRequest{
		RecordId: dara.String(recordID),
	}

	if _, err := p.client.DeleteDomainRecordWithOptions(request, runtimeOptions(ctx)); err != nil {
		return fmt.Errorf("alidns: delete record: %w", err)
	}

	return nil
}

func (p *provider) describeRecord(ctx context.Context, recordID string) (dns.Record, error) {
	request := &aliclient.DescribeDomainRecordInfoRequest{
		RecordId: dara.String(recordID),
	}

	resp, err := p.client.DescribeDomainRecordInfoWithOptions(request, runtimeOptions(ctx))
	if err != nil {
		return dns.Record{}, fmt.Errorf("alidns: describe record: %w", err)
	}

	if resp.Body == nil {
		return dns.Record{}, fmt.Errorf("alidns: describe record: empty body")
	}

	record := dns.Record{
		ID:      stringValue(resp.Body.RecordId),
		Type:    stringValue(resp.Body.Type),
		Name:    rrToName(stringValue(resp.Body.RR)),
		Content: stringValue(resp.Body.Value),
		TTL:     int(int64Value(resp.Body.TTL)),
	}

	if resp.Body.Priority != nil {
		value := int(int64Value(resp.Body.Priority))
		record.Priority = &value
	}

	return record, nil
}

func runtimeOptions(ctx context.Context) *utilruntime.RuntimeOptions {
	timeout := defaultTimeout()
	opts := &utilruntime.RuntimeOptions{}
	opts.SetConnectTimeout(int(timeout.Milliseconds()))
	opts.SetReadTimeout(int(timeout.Milliseconds()))
	opts.SetAutoretry(true)
	opts.SetMaxAttempts(3)
	opts.SetBackoffPolicy("exponential")
	opts.SetBackoffPeriod(1)
	return opts
}

func defaultTimeout() time.Duration {
	return 10 * time.Second
}

func rrFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "@" {
		return "@"
	}
	return name
}

func rrToName(rr string) string {
	if rr == "" {
		return "@"
	}
	return rr
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func intPointerFrom32(value *int32) *int {
	if value == nil {
		return nil
	}
	v := int(*value)
	if v == 0 {
		return nil
	}
	return &v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
