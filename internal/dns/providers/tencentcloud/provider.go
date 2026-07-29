package tencentcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const (
	defaultRecordLine      = "默认"
	defaultTencentEndpoint = "https://dnspod.tencentcloudapi.com"
	tencentService         = "dnspod"
	tencentAPIVersion      = "2021-03-23"
	maxRecordsPageSize     = 3000
)

type provider struct {
	client tencentDNSClient
}

type tencentDNSClient interface {
	Call(ctx context.Context, action string, request, response any) error
}

type commonDNSClient struct {
	client *common.Client
}

type recordListRequest struct {
	Domain     string `json:"Domain"`
	Subdomain  string `json:"Subdomain,omitempty"`
	RecordType string `json:"RecordType,omitempty"`
	Offset     uint64 `json:"Offset"`
	Limit      uint64 `json:"Limit"`
}

type recordMutationRequest struct {
	Domain     string  `json:"Domain"`
	RecordID   uint64  `json:"RecordId,omitempty"`
	Subdomain  string  `json:"SubDomain"`
	RecordType string  `json:"RecordType"`
	RecordLine string  `json:"RecordLine"`
	Value      string  `json:"Value"`
	TTL        uint64  `json:"TTL"`
	MX         *uint64 `json:"MX,omitempty"`
	Weight     *uint64 `json:"Weight,omitempty"`
}

type recordReferenceRequest struct {
	Domain   string `json:"Domain"`
	RecordID uint64 `json:"RecordId"`
}

type recordListItem struct {
	RecordID uint64  `json:"RecordId"`
	Type     string  `json:"Type"`
	Name     string  `json:"Name"`
	Value    string  `json:"Value"`
	TTL      uint64  `json:"TTL"`
	Line     string  `json:"Line"`
	Weight   *uint64 `json:"Weight"`
	MX       *uint64 `json:"MX"`
}

type recordInfo struct {
	ID         uint64  `json:"Id"`
	Subdomain  string  `json:"SubDomain"`
	RecordType string  `json:"RecordType"`
	RecordLine string  `json:"RecordLine"`
	Value      string  `json:"Value"`
	TTL        uint64  `json:"TTL"`
	Weight     *uint64 `json:"Weight"`
	MX         *uint64 `json:"MX"`
}

type listRecordsResponse struct {
	Response struct {
		RecordCountInfo struct {
			TotalCount uint64 `json:"TotalCount"`
		} `json:"RecordCountInfo"`
		RecordList []recordListItem `json:"RecordList"`
	} `json:"Response"`
}

type createRecordResponse struct {
	Response struct {
		RecordID uint64 `json:"RecordId"`
	} `json:"Response"`
}

type describeRecordResponse struct {
	Response struct {
		RecordInfo *recordInfo `json:"RecordInfo"`
	} `json:"Response"`
}

func init() {
	dns.RegisterProvider("tencentcloud", newProvider)
}

func newProvider(cred *dns.Credential) (dns.Provider, error) {
	secretID := firstNonEmpty(
		cred.Values["TENCENTCLOUD_SECRET_ID"],
		cred.Values["QCLOUD_SECRET_ID"],
	)
	secretKey := firstNonEmpty(
		cred.Values["TENCENTCLOUD_SECRET_KEY"],
		cred.Values["QCLOUD_SECRET_KEY"],
	)
	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("tencentcloud: missing secret id or secret key")
	}

	var credential common.CredentialIface
	if token := firstNonEmpty(cred.Values["TENCENTCLOUD_SESSION_TOKEN"]); token != "" {
		credential = common.NewTokenCredential(secretID, secretKey, token)
	} else {
		credential = common.NewCredential(secretID, secretKey)
	}

	endpoint := firstNonEmpty(cred.Additional["TENCENTCLOUD_ENDPOINT"], defaultTencentEndpoint)
	parsed, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("tencentcloud: invalid endpoint: %w", err)
	}

	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile = &profile.HttpProfile{
		Endpoint:   parsed.Host,
		Scheme:     strings.ToUpper(parsed.Scheme),
		ReqMethod:  "POST",
		ReqTimeout: int(defaultTimeout().Seconds()),
	}
	client := common.NewCommonClient(credential, "", clientProfile)
	return &provider{client: &commonDNSClient{client: client}}, nil
}

func parseEndpoint(endpoint string) (*url.URL, error) {
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, fmt.Errorf("endpoint path is not supported")
	}
	return parsed, nil
}

func (c *commonDNSClient) Call(ctx context.Context, action string, requestData, result any) error {
	request := tchttp.NewCommonRequest(tencentService, tencentAPIVersion, action)
	request.SetContext(ctx)
	payload, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	if err := request.SetActionParameters(payload); err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	response := tchttp.NewCommonResponse()
	if err := c.client.Send(request, response); err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(response.GetBody(), result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	result := make([]dns.Record, 0)
	for offset := uint64(0); ; offset += maxRecordsPageSize {
		request := recordListRequest{
			Domain:     domain,
			Subdomain:  strings.TrimSpace(filter.Name),
			RecordType: strings.ToUpper(strings.TrimSpace(filter.Type)),
			Offset:     offset,
			Limit:      maxRecordsPageSize,
		}
		var response listRecordsResponse
		if err := p.client.Call(ctx, "DescribeRecordList", request, &response); err != nil {
			if isTencentNoDataError(err) {
				return []dns.Record{}, nil
			}
			return nil, fmt.Errorf("tencentcloud: list records: %w", err)
		}

		for _, record := range response.Response.RecordList {
			result = append(result, record.toDNSRecord())
		}
		if len(response.Response.RecordList) < maxRecordsPageSize ||
			(response.Response.RecordCountInfo.TotalCount > 0 && uint64(len(result)) >= response.Response.RecordCountInfo.TotalCount) {
			break
		}
	}
	return result, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	request := newRecordMutationRequest(domain, 0, input)
	var response createRecordResponse
	if err := p.client.Call(ctx, "CreateRecord", request, &response); err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: create record: %w", err)
	}
	if response.Response.RecordID == 0 {
		return dns.Record{}, fmt.Errorf("tencentcloud: create record: empty response")
	}
	return p.describeRecord(ctx, domain, strconv.FormatUint(response.Response.RecordID, 10))
}

func (p *provider) UpdateRecord(ctx context.Context, domain string, recordID string, input dns.RecordInput) (dns.Record, error) {
	id, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: invalid record id: %w", err)
	}
	request := newRecordMutationRequest(domain, id, input)
	if err := p.client.Call(ctx, "ModifyRecord", request, nil); err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: update record: %w", err)
	}
	return p.describeRecord(ctx, domain, recordID)
}

func (p *provider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	id, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return fmt.Errorf("tencentcloud: invalid record id: %w", err)
	}
	request := recordReferenceRequest{Domain: domain, RecordID: id}
	if err := p.client.Call(ctx, "DeleteRecord", request, nil); err != nil {
		return fmt.Errorf("tencentcloud: delete record: %w", err)
	}
	return nil
}

func (p *provider) describeRecord(ctx context.Context, domain string, recordID string) (dns.Record, error) {
	id, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: invalid record id: %w", err)
	}
	request := recordReferenceRequest{Domain: domain, RecordID: id}
	var response describeRecordResponse
	if err := p.client.Call(ctx, "DescribeRecord", request, &response); err != nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: describe record: %w", err)
	}
	if response.Response.RecordInfo == nil {
		return dns.Record{}, fmt.Errorf("tencentcloud: describe record: empty response")
	}
	return response.Response.RecordInfo.toDNSRecord(recordID), nil
}

func newRecordMutationRequest(domain string, recordID uint64, input dns.RecordInput) recordMutationRequest {
	request := recordMutationRequest{
		Domain:     domain,
		RecordID:   recordID,
		Subdomain:  normalizeSubDomain(input.Name),
		RecordType: strings.ToUpper(strings.TrimSpace(input.Type)),
		RecordLine: recordLine(input.Line, defaultRecordLine),
		Value:      strings.TrimSpace(input.Content),
		TTL:        uint64(max(input.TTL, 0)),
	}
	if input.Priority != nil {
		value := uint64(max(*input.Priority, 0))
		request.MX = &value
	}
	if input.Weight != nil {
		value := uint64(max(*input.Weight, 0))
		request.Weight = &value
	}
	return request
}

func (r recordListItem) toDNSRecord() dns.Record {
	return dns.Record{
		ID:       strconv.FormatUint(r.RecordID, 10),
		Type:     r.Type,
		Name:     normalizeName(r.Name),
		Content:  r.Value,
		TTL:      int(r.TTL),
		Line:     r.Line,
		Weight:   uint64Pointer(r.Weight),
		Priority: uint64Pointer(r.MX),
	}
}

func (r recordInfo) toDNSRecord(recordID string) dns.Record {
	return dns.Record{
		ID:       recordID,
		Type:     r.RecordType,
		Name:     normalizeName(r.Subdomain),
		Content:  r.Value,
		TTL:      int(r.TTL),
		Line:     r.RecordLine,
		Weight:   uint64Pointer(r.Weight),
		Priority: uint64Pointer(r.MX),
	}
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

func recordLine(line *string, fallback string) string {
	if line == nil {
		return fallback
	}
	return firstNonEmpty(*line, fallback)
}

func uint64Pointer(value *uint64) *int {
	if value == nil || *value == 0 {
		return nil
	}
	result := int(*value)
	return &result
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isTencentNoDataError(err error) bool {
	var sdkErr *tcerrors.TencentCloudSDKError
	return errors.As(err, &sdkErr) && sdkErr.Code == "ResourceNotFound.NoDataOfRecord"
}
