package alidns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const (
	defaultRegion      = "cn-hangzhou"
	defaultLineName    = "default"
	publicEndpoint     = "https://alidns.aliyuncs.com"
	apiVersion         = "2015-01-09"
	maxRecordsPageSize = 500
)

type provider struct {
	client aliDNSClient
}

type aliDNSClient interface {
	Call(ctx context.Context, action string, query map[string]any, result any) error
}

type openAPIClient struct {
	client   *openapi.Client
	protocol string
}

type domainRecord struct {
	RecordID string `json:"RecordId"`
	Type     string `json:"Type"`
	RR       string `json:"RR"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Line     string `json:"Line"`
	Priority *int   `json:"Priority"`
	Weight   *int   `json:"Weight"`
}

type describeDomainRecordsResponse struct {
	TotalCount    int `json:"TotalCount"`
	PageNumber    int `json:"PageNumber"`
	PageSize      int `json:"PageSize"`
	DomainRecords struct {
		Records []domainRecord `json:"Record"`
	} `json:"DomainRecords"`
}

type describeSupportLinesResponse struct {
	RecordLines struct {
		Lines []supportLine `json:"RecordLine"`
	} `json:"RecordLines"`
}

type supportLine struct {
	Code        string `json:"LineCode"`
	Name        string `json:"LineName"`
	DisplayName string `json:"LineDisplayName"`
	FatherCode  string `json:"FatherCode"`
}

type recordIDResponse struct {
	RecordID string `json:"RecordId"`
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
	securityToken := firstNonEmpty(
		cred.Values["ALICLOUD_SECURITY_TOKEN"],
		cred.Additional["ALICLOUD_SECURITY_TOKEN"],
	)
	endpoint := aliDNSEndpoint(region, cred.Additional["ALICLOUD_ENDPOINT"])

	client, err := newOpenAPIClient(accessKey, secretKey, securityToken, region, endpoint)
	if err != nil {
		return nil, err
	}

	return &provider{client: client}, nil
}

func newOpenAPIClient(accessKey, secretKey, securityToken, region, endpoint string) (*openAPIClient, error) {
	parsed, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("alidns: invalid endpoint: %w", err)
	}

	timeout := int(defaultTimeout().Milliseconds())
	config := new(openapi.Config).
		SetAccessKeyId(accessKey).
		SetAccessKeySecret(secretKey).
		SetSecurityToken(securityToken).
		SetRegionId(region).
		SetEndpoint(parsed.Host).
		SetProtocol(strings.ToUpper(parsed.Scheme)).
		SetSignatureAlgorithm("ACS3-HMAC-SHA256").
		SetReadTimeout(timeout).
		SetConnectTimeout(timeout)

	client, err := openapi.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("alidns: new client: %w", err)
	}

	return &openAPIClient{client: client, protocol: strings.ToUpper(parsed.Scheme)}, nil
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

func aliDNSEndpoint(region, override string) string {
	if endpoint := strings.TrimSpace(override); endpoint != "" {
		return endpoint
	}
	if strings.EqualFold(strings.TrimSpace(region), "public") {
		return publicEndpoint
	}
	return fmt.Sprintf("https://alidns.%s.aliyuncs.com", strings.TrimSpace(region))
}

func (c *openAPIClient) Call(ctx context.Context, action string, query map[string]any, result any) error {
	request := &openapi.OpenApiRequest{Query: openapiutil.Query(query)}
	params := new(openapi.Params).
		SetAction(action).
		SetVersion(apiVersion).
		SetProtocol(c.protocol).
		SetPathname("/").
		SetMethod("POST").
		SetAuthType("AK").
		SetStyle("RPC").
		SetReqBodyType("formData").
		SetBodyType("json")

	response, err := c.client.CallApiWithCtx(ctx, params, request, runtimeOptions())
	if err != nil {
		return err
	}
	body, ok := response["body"]
	if !ok {
		return fmt.Errorf("missing response body")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal response body: %w", err)
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	result := make([]dns.Record, 0)
	for pageNumber := 1; ; pageNumber++ {
		query := map[string]any{
			"DomainName": domain,
			"PageNumber": pageNumber,
			"PageSize":   maxRecordsPageSize,
		}
		if name := strings.TrimSpace(filter.Name); name != "" {
			query["RRKeyWord"] = name
		}
		if recordType := strings.TrimSpace(filter.Type); recordType != "" {
			query["TypeKeyWord"] = strings.ToUpper(recordType)
		}

		var response describeDomainRecordsResponse
		if err := p.client.Call(ctx, "DescribeDomainRecords", query, &response); err != nil {
			return nil, fmt.Errorf("alidns: list records: %w", err)
		}

		for _, record := range response.DomainRecords.Records {
			result = append(result, record.toDNSRecord())
		}

		if len(response.DomainRecords.Records) < maxRecordsPageSize ||
			(response.TotalCount > 0 && len(result) >= response.TotalCount) {
			break
		}
	}
	return result, nil
}

func (p *provider) ListRecordLines(ctx context.Context, domain string) ([]dns.RecordLine, error) {
	var response describeSupportLinesResponse
	if err := p.client.Call(ctx, "DescribeSupportLines", map[string]any{"DomainName": domain}, &response); err != nil {
		return nil, fmt.Errorf("alidns: list record lines: %w", err)
	}

	result := make([]dns.RecordLine, 0, len(response.RecordLines.Lines))
	for _, line := range response.RecordLines.Lines {
		code := strings.TrimSpace(line.Code)
		if code == "" {
			continue
		}
		result = append(result, dns.RecordLine{
			Code:        code,
			Name:        line.Name,
			DisplayName: line.DisplayName,
			FatherCode:  line.FatherCode,
		})
	}
	return result, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	query := recordMutationQuery(input)
	query["DomainName"] = domain
	query["Line"] = recordLine(input.Line, defaultLineName)

	var response recordIDResponse
	if err := p.client.Call(ctx, "AddDomainRecord", query, &response); err != nil {
		if isAPIErrorCode(err, "DomainRecordDuplicate") {
			return dns.Record{}, fmt.Errorf("alidns: a DNS record with the same name, type, and line already exists")
		}
		return dns.Record{}, fmt.Errorf("alidns: add record: %w", err)
	}
	if response.RecordID == "" {
		return dns.Record{}, fmt.Errorf("alidns: empty record id")
	}
	return p.describeRecord(ctx, response.RecordID)
}

func (p *provider) UpdateRecord(ctx context.Context, _ string, recordID string, input dns.RecordInput) (dns.Record, error) {
	line, err := p.updateRecordLine(ctx, recordID, input.Line)
	if err != nil {
		return dns.Record{}, err
	}

	query := recordMutationQuery(input)
	query["RecordId"] = recordID
	query["Line"] = line
	if err := p.client.Call(ctx, "UpdateDomainRecord", query, &struct{}{}); err != nil {
		if isAPIErrorCode(err, "DomainRecordDuplicate") {
			return p.describeRecord(ctx, recordID)
		}
		return dns.Record{}, fmt.Errorf("alidns: update record: %w", err)
	}
	return p.describeRecord(ctx, recordID)
}

func (p *provider) DeleteRecord(ctx context.Context, _ string, recordID string) error {
	if err := p.client.Call(ctx, "DeleteDomainRecord", map[string]any{"RecordId": recordID}, &struct{}{}); err != nil {
		return fmt.Errorf("alidns: delete record: %w", err)
	}
	return nil
}

func (p *provider) describeRecord(ctx context.Context, recordID string) (dns.Record, error) {
	var response domainRecord
	if err := p.client.Call(ctx, "DescribeDomainRecordInfo", map[string]any{"RecordId": recordID}, &response); err != nil {
		return dns.Record{}, fmt.Errorf("alidns: describe record: %w", err)
	}
	if response.RecordID == "" {
		return dns.Record{}, fmt.Errorf("alidns: describe record: empty body")
	}
	return response.toDNSRecord(), nil
}

func (p *provider) updateRecordLine(ctx context.Context, recordID string, requested *string) (string, error) {
	if line := recordLine(requested, ""); line != "" {
		return line, nil
	}
	record, err := p.describeRecord(ctx, recordID)
	if err != nil {
		return "", fmt.Errorf("alidns: preserve record line: %w", err)
	}
	return firstNonEmpty(strings.TrimSpace(record.Line), defaultLineName), nil
}

func (r domainRecord) toDNSRecord() dns.Record {
	return dns.Record{
		ID:       r.RecordID,
		Type:     r.Type,
		Name:     rrToName(r.RR),
		Content:  r.Value,
		TTL:      r.TTL,
		Line:     r.Line,
		Priority: r.Priority,
		Weight:   nonZeroIntPointer(r.Weight),
	}
}

func recordMutationQuery(input dns.RecordInput) map[string]any {
	query := map[string]any{
		"Type":  strings.ToUpper(strings.TrimSpace(input.Type)),
		"RR":    rrFromName(input.Name),
		"Value": strings.TrimSpace(input.Content),
		"TTL":   input.TTL,
	}
	if input.Priority != nil {
		query["Priority"] = *input.Priority
	}
	return query
}

func isAPIErrorCode(err error, code string) bool {
	var coded interface{ GetCode() *string }
	if errors.As(err, &coded) && coded.GetCode() != nil && *coded.GetCode() == code {
		return true
	}
	return strings.Contains(err.Error(), code)
}

func runtimeOptions() *dara.RuntimeOptions {
	timeout := defaultTimeout()
	return new(dara.RuntimeOptions).
		SetConnectTimeout(int(timeout.Milliseconds())).
		SetReadTimeout(int(timeout.Milliseconds())).
		SetAutoretry(true).
		SetMaxAttempts(3).
		SetBackoffPolicy("exponential").
		SetBackoffPeriod(1)
}

func defaultTimeout() time.Duration {
	return 10 * time.Second
}

func recordLine(line *string, fallback string) string {
	if line == nil {
		return fallback
	}
	return firstNonEmpty(strings.TrimSpace(*line), fallback)
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

func nonZeroIntPointer(value *int) *int {
	if value == nil || *value == 0 {
		return nil
	}
	copy := *value
	return &copy
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
