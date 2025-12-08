package tencentcloud

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const (
	defaultRecordLine = "默认"
	tencentEndpoint   = "dnspod.tencentcloudapi.com"
)

type provider struct {
	client *dnspod.Client
}

func init() {
	dns.RegisterProvider("tencentcloud", newProvider)
}

func newProvider(cred *dns.Credential) (dns.Provider, error) {
	secretID := strings.TrimSpace(firstNonEmpty(
		cred.Values["TENCENTCLOUD_SECRET_ID"],
		cred.Values["QCLOUD_SECRET_ID"],
	))
	secretKey := strings.TrimSpace(firstNonEmpty(
		cred.Values["TENCENTCLOUD_SECRET_KEY"],
		cred.Values["QCLOUD_SECRET_KEY"],
	))

	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("tencentcloud: missing secret id or secret key")
	}

	var credential common.CredentialIface
	if token := strings.TrimSpace(firstNonEmpty(
		cred.Values["TENCENTCLOUD_SESSION_TOKEN"],
	)); token != "" {
		credential = common.NewTokenCredential(secretID, secretKey, token)
	} else {
		credential = common.NewCredential(secretID, secretKey)
	}

	cp := profile.NewClientProfile()
	cp.HttpProfile = &profile.HttpProfile{
		Endpoint:   tencentEndpoint,
		ReqTimeout: int(defaultTimeout().Seconds()),
	}

	client, err := dnspod.NewClient(credential, "", cp)
	if err != nil {
		return nil, fmt.Errorf("tencentcloud: new client: %w", err)
	}

	return &provider{client: client}, nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domain)
	req.Offset = common.Uint64Ptr(0)
	req.Limit = common.Uint64Ptr(3000)

	if filter.Name != "" {
		req.Subdomain = common.StringPtr(filter.Name)
	}
	if filter.Type != "" {
		req.RecordType = common.StringPtr(strings.ToUpper(filter.Type))
	}

	resp, err := p.client.DescribeRecordListWithContext(ctx, req)
	if err != nil {
		if isTencentNoDataError(err) {
			return []dns.Record{}, nil
		}
		return nil, fmt.Errorf("tencentcloud: list records: %w", err)
	}

	if resp.Response == nil || len(resp.Response.RecordList) == 0 {
		return []dns.Record{}, nil
	}

	result := make([]dns.Record, 0, len(resp.Response.RecordList))
	for _, item := range resp.Response.RecordList {
		if item == nil {
			continue
		}
		record := dns.Record{
			ID:       uint64ToString(item.RecordId),
			Type:     stringValue(item.Type),
			Name:     normalizeName(stringValue(item.Name)),
			Content:  stringValue(item.Value),
			TTL:      int(uint64Value(item.TTL)),
			Weight:   uint64Pointer(item.Weight),
			Priority: uint64Pointer(item.MX),
		}
		result = append(result, record)
	}

	return result, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	req := dnspod.NewCreateRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.SubDomain = common.StringPtr(normalizeSubDomain(input.Name))
	req.RecordType = common.StringPtr(strings.ToUpper(strings.TrimSpace(input.Type)))
	req.RecordLine = common.StringPtr(defaultRecordLine)
	req.Value = common.StringPtr(strings.TrimSpace(input.Content))
	req.TTL = common.Uint64Ptr(uint64(input.TTL))

	if input.Priority != nil {
		mx := uint64(max(*input.Priority, 0))
		req.MX = &mx
	}

	if input.Weight != nil {
		weight := uint64(max(*input.Weight, 0))
		req.Weight = &weight
	}

	resp, err := p.client.CreateRecordWithContext(ctx, req)
	if err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: create record: %w", err)
	}

	if resp.Response == nil || resp.Response.RecordId == nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: create record: empty response")
	}

	return p.describeRecord(ctx, domain, strconv.FormatUint(*resp.Response.RecordId, 10))
}

func (p *provider) UpdateRecord(ctx context.Context, domain string, recordID string, input dns.RecordInput) (dns.Record, error) {
	id, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: invalid record id: %w", err)
	}

	req := dnspod.NewModifyRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordId = common.Uint64Ptr(id)
	req.SubDomain = common.StringPtr(normalizeSubDomain(input.Name))
	req.RecordType = common.StringPtr(strings.ToUpper(strings.TrimSpace(input.Type)))
	req.RecordLine = common.StringPtr(defaultRecordLine)
	req.Value = common.StringPtr(strings.TrimSpace(input.Content))
	req.TTL = common.Uint64Ptr(uint64(input.TTL))

	if input.Priority != nil {
		mx := uint64(max(*input.Priority, 0))
		req.MX = &mx
	}
	if input.Weight != nil {
		weight := uint64(max(*input.Weight, 0))
		req.Weight = &weight
	}

	if _, err := p.client.ModifyRecordWithContext(ctx, req); err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: update record: %w", err)
	}

	return p.describeRecord(ctx, domain, recordID)
}

func (p *provider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	id, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return fmt.Errorf("tencentcloud: invalid record id: %w", err)
	}

	req := dnspod.NewDeleteRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordId = common.Uint64Ptr(id)

	if _, err := p.client.DeleteRecordWithContext(ctx, req); err != nil {
		return fmt.Errorf("tencentcloud: delete record: %w", err)
	}

	return nil
}

func (p *provider) describeRecord(ctx context.Context, domain string, recordID string) (dns.Record, error) {
	id, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: invalid record id: %w", err)
	}

	req := dnspod.NewDescribeRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordId = common.Uint64Ptr(id)

	resp, err := p.client.DescribeRecordWithContext(ctx, req)
	if err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: describe record: %w", err)
	}

	if resp.Response == nil || resp.Response.RecordInfo == nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: describe record: empty response")
	}

	info := resp.Response.RecordInfo
	record := dns.Record{
		ID:       recordID,
		Type:     stringValue(info.RecordType),
		Name:     normalizeName(stringValue(info.SubDomain)),
		Content:  stringValue(info.Value),
		TTL:      int(uint64Value(info.TTL)),
		Weight:   uint64Pointer(info.Weight),
		Priority: uint64Pointer(info.MX),
	}

	return record, nil
}

func normalizeSubDomain(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "@" {
		return "@"
	}
	return name
}

func normalizeName(name string) string {
	if name == "" {
		return "@"
	}
	return name
}

func uint64ToString(value *uint64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatUint(*value, 10)
}

func uint64Value(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

func uint64Pointer(value *uint64) *int {
	if value == nil || *value == 0 {
		return nil
	}
	v := int(*value)
	return &v
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func defaultTimeout() time.Duration {
	return 10 * time.Second
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func isTencentNoDataError(err error) bool {
	var sdkErr *tcerrors.TencentCloudSDKError
	if errors.As(err, &sdkErr) {
		return sdkErr.Code == "ResourceNotFound.NoDataOfRecord"
	}
	return false
}
