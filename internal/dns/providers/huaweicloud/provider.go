package huaweicloud

import (
	"context"
	"fmt"
	"sort"
	"strings"

	hwauth "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	hwconfig "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	hwdns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	hwmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	hwregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

const (
	providerCode      = "huaweicloud"
	defaultRecordLine = "default_view"
	pageSize          = int32(500)
)

type provider struct {
	client huaweiDNSClient
}

type huaweiDNSClient interface {
	ListPublicZones(*hwmodel.ListPublicZonesRequest) (*hwmodel.ListPublicZonesResponse, error)
	ListRecordSetsWithLine(*hwmodel.ListRecordSetsWithLineRequest) (*hwmodel.ListRecordSetsWithLineResponse, error)
	CreateRecordSetWithLine(*hwmodel.CreateRecordSetWithLineRequest) (*hwmodel.CreateRecordSetWithLineResponse, error)
	UpdateRecordSets(*hwmodel.UpdateRecordSetsRequest) (*hwmodel.UpdateRecordSetsResponse, error)
	DeleteRecordSets(*hwmodel.DeleteRecordSetsRequest) (*hwmodel.DeleteRecordSetsResponse, error)
	ShowRecordSetWithLine(*hwmodel.ShowRecordSetWithLineRequest) (*hwmodel.ShowRecordSetWithLineResponse, error)
	ListPublicZoneLines(*hwmodel.ListPublicZoneLinesRequest) (*hwmodel.ListPublicZoneLinesResponse, error)
}

var (
	_ dns.Provider           = (*provider)(nil)
	_ dns.RecordLineProvider = (*provider)(nil)
)

func init() {
	dns.RegisterProvider(providerCode, newProvider)
}

func newProvider(cred *dns.Credential) (dns.Provider, error) {
	accessKeyID := lookupCredential(cred, "HUAWEICLOUD_ACCESS_KEY_ID")
	secretAccessKey := lookupCredential(cred, "HUAWEICLOUD_SECRET_ACCESS_KEY")
	regionID := lookupCredential(cred, "HUAWEICLOUD_REGION")

	if accessKeyID == "" || secretAccessKey == "" || regionID == "" {
		return nil, fmt.Errorf("huaweicloud: missing access key id, secret access key, or region")
	}

	credential, err := hwauth.NewCredentialsBuilder().
		WithAk(accessKeyID).
		WithSk(secretAccessKey).
		SafeBuild()
	if err != nil {
		return nil, fmt.Errorf("huaweicloud: build credential: %w", err)
	}

	region, err := hwregion.SafeValueOf(regionID)
	if err != nil {
		return nil, fmt.Errorf("huaweicloud: resolve region: %w", err)
	}

	httpClient, err := hwdns.DnsClientBuilder().
		WithHttpConfig(hwconfig.DefaultHttpConfig().WithTimeout(defaultTimeout())).
		WithCredential(credential).
		WithRegion(region).
		SafeBuild()
	if err != nil {
		return nil, fmt.Errorf("huaweicloud: build client: %w", err)
	}

	return &provider{client: hwdns.NewDnsClient(httpClient)}, nil
}

func (p *provider) ListRecords(ctx context.Context, domain string, filter dns.RecordFilter) ([]dns.Record, error) {
	zoneID, err := p.resolveZoneID(ctx, domain)
	if err != nil {
		return nil, err
	}

	zoneType := "public"
	request := &hwmodel.ListRecordSetsWithLineRequest{
		ZoneType: &zoneType,
		ZoneId:   &zoneID,
		Limit:    pointer(pageSize),
	}
	if recordType := strings.ToUpper(strings.TrimSpace(filter.Type)); recordType != "" {
		request.Type = &recordType
	}
	if name := strings.TrimSpace(filter.Name); name != "" {
		fqdn := recordFQDN(domain, name)
		request.Name = &fqdn
	}

	records := make([]dns.Record, 0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("huaweicloud: list records: %w", err)
		}

		response, err := p.client.ListRecordSetsWithLine(request)
		if err != nil {
			return nil, fmt.Errorf("huaweicloud: list records: %w", err)
		}
		if response == nil || response.Recordsets == nil || len(*response.Recordsets) == 0 {
			break
		}

		items := *response.Recordsets
		for i := range items {
			records = append(records, recordFromFields(recordFields{
				id:       stringValue(items[i].Id),
				name:     stringValue(items[i].Name),
				zoneName: stringValue(items[i].ZoneName),
				typeName: stringValue(items[i].Type),
				ttl:      int32Value(items[i].Ttl),
				values:   stringSliceValue(items[i].Records),
				line:     stringValue(items[i].Line),
				weight:   items[i].Weight,
			}))
		}

		if len(items) < int(pageSize) {
			break
		}

		nextMarker := stringValue(items[len(items)-1].Id)
		if nextMarker == "" || nextMarker == stringValue(request.Marker) {
			return nil, fmt.Errorf("huaweicloud: list records: pagination did not advance")
		}
		request.Marker = &nextMarker
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		if records[i].Type != records[j].Type {
			return records[i].Type < records[j].Type
		}
		if records[i].Line != records[j].Line {
			return records[i].Line < records[j].Line
		}
		return records[i].ID < records[j].ID
	})

	return records, nil
}

func (p *provider) ListRecordLines(ctx context.Context, domain string) ([]dns.RecordLine, error) {
	zoneID, err := p.resolveZoneID(ctx, domain)
	if err != nil {
		return nil, err
	}

	lines := make([]dns.RecordLine, 0)
	for offset := int32(0); ; offset += pageSize {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("huaweicloud: list record lines: %w", err)
		}

		response, err := p.client.ListPublicZoneLines(&hwmodel.ListPublicZoneLinesRequest{
			ZoneId: zoneID,
			Limit:  pointer(pageSize),
			Offset: pointer(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("huaweicloud: list record lines: %w", err)
		}
		if response == nil || response.Lines == nil || len(*response.Lines) == 0 {
			break
		}

		items := *response.Lines
		for i := range items {
			code := strings.TrimSpace(stringValue(items[i].Line))
			if code == "" {
				continue
			}

			name := strings.TrimSpace(stringValue(items[i].LineName))
			if name == "" && code == defaultRecordLine {
				name = "Default"
			}
			lines = append(lines, dns.RecordLine{Code: code, Name: name})
		}

		if len(items) < int(pageSize) {
			break
		}
	}

	return lines, nil
}

func (p *provider) CreateRecord(ctx context.Context, domain string, input dns.RecordInput) (dns.Record, error) {
	zoneID, err := p.resolveZoneID(ctx, domain)
	if err != nil {
		return dns.Record{}, err
	}

	recordType := strings.ToUpper(strings.TrimSpace(input.Type))
	values, err := recordValuesFromInput(recordType, input)
	if err != nil {
		return dns.Record{}, err
	}

	line := recordLine(input.Line, defaultRecordLine)
	body := &hwmodel.CreateRecordSetWithLineRequestBody{
		Name:    recordFQDN(domain, input.Name),
		Type:    recordType,
		Ttl:     int32Pointer(input.TTL),
		Records: &values,
		Line:    &line,
	}
	if input.Comment != "" {
		body.Description = pointer(input.Comment)
	}
	if recordType != "SRV" && input.Weight != nil {
		body.Weight = int32Pointer(*input.Weight)
	}

	if err := ctx.Err(); err != nil {
		return dns.Record{}, fmt.Errorf("huaweicloud: create record: %w", err)
	}
	response, err := p.client.CreateRecordSetWithLine(&hwmodel.CreateRecordSetWithLineRequest{
		ZoneId: zoneID,
		Body:   body,
	})
	if err != nil {
		return dns.Record{}, fmt.Errorf("huaweicloud: create record: %w", err)
	}
	if response == nil || strings.TrimSpace(stringValue(response.Id)) == "" {
		return dns.Record{}, fmt.Errorf("huaweicloud: create record: empty record id")
	}

	return recordFromFields(recordFields{
		id:       stringValue(response.Id),
		name:     stringValue(response.Name),
		zoneName: firstNonEmpty(stringValue(response.ZoneName), domain),
		typeName: stringValue(response.Type),
		ttl:      int32Value(response.Ttl),
		values:   stringSliceValue(response.Records),
		line:     stringValue(response.Line),
		weight:   response.Weight,
	}), nil
}

func (p *provider) UpdateRecord(ctx context.Context, domain string, recordID string, input dns.RecordInput) (dns.Record, error) {
	zoneID, err := p.resolveZoneID(ctx, domain)
	if err != nil {
		return dns.Record{}, err
	}

	existing, err := p.showRecord(ctx, zoneID, recordID)
	if err != nil {
		return dns.Record{}, err
	}

	existingLine := firstNonEmpty(stringValue(existing.Line), defaultRecordLine)
	if requestedLine := recordLine(input.Line, ""); requestedLine != "" && requestedLine != existingLine {
		return dns.Record{}, fmt.Errorf("huaweicloud: changing a record resolution line is not supported; create a new record on line %q", requestedLine)
	}

	recordType := strings.ToUpper(strings.TrimSpace(input.Type))
	values, err := recordValuesFromInput(recordType, input)
	if err != nil {
		return dns.Record{}, err
	}

	body := &hwmodel.UpdateRecordSetsReq{
		Name:    recordFQDN(domain, input.Name),
		Type:    recordType,
		Ttl:     int32Pointer(input.TTL),
		Records: &values,
	}
	if input.Comment != "" {
		body.Description = pointer(input.Comment)
	}
	if recordType != "SRV" && input.Weight != nil {
		body.Weight = int32Pointer(*input.Weight)
	}

	if err := ctx.Err(); err != nil {
		return dns.Record{}, fmt.Errorf("huaweicloud: update record: %w", err)
	}
	response, err := p.client.UpdateRecordSets(&hwmodel.UpdateRecordSetsRequest{
		ZoneId:      zoneID,
		RecordsetId: recordID,
		Body:        body,
	})
	if err != nil {
		return dns.Record{}, fmt.Errorf("huaweicloud: update record: %w", err)
	}
	if response == nil {
		return dns.Record{}, fmt.Errorf("huaweicloud: update record: empty response")
	}

	return recordFromFields(recordFields{
		id:       firstNonEmpty(stringValue(response.Id), recordID),
		name:     stringValue(response.Name),
		zoneName: firstNonEmpty(stringValue(response.ZoneName), domain),
		typeName: stringValue(response.Type),
		ttl:      int32Value(response.Ttl),
		values:   stringSliceValue(response.Records),
		line:     firstNonEmpty(stringValue(response.Line), existingLine),
		weight:   response.Weight,
	}), nil
}

func (p *provider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	zoneID, err := p.resolveZoneID(ctx, domain)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("huaweicloud: delete record: %w", err)
	}

	_, err = p.client.DeleteRecordSets(&hwmodel.DeleteRecordSetsRequest{
		ZoneId:      zoneID,
		RecordsetId: recordID,
	})
	if err != nil {
		return fmt.Errorf("huaweicloud: delete record: %w", err)
	}

	return nil
}

func (p *provider) showRecord(ctx context.Context, zoneID, recordID string) (*hwmodel.ShowRecordSetWithLineResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("huaweicloud: show record: %w", err)
	}

	response, err := p.client.ShowRecordSetWithLine(&hwmodel.ShowRecordSetWithLineRequest{
		ZoneId:      zoneID,
		RecordsetId: recordID,
	})
	if err != nil {
		return nil, fmt.Errorf("huaweicloud: show record: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("huaweicloud: show record: empty response")
	}

	return response, nil
}

func (p *provider) resolveZoneID(ctx context.Context, domain string) (string, error) {
	zoneName := normalizeZoneName(domain)
	if zoneName == "" {
		return "", fmt.Errorf("huaweicloud: resolve zone: empty domain")
	}

	requestName := zoneName + "."
	searchMode := "equal"
	zoneType := "public"
	request := &hwmodel.ListPublicZonesRequest{
		Type:       &zoneType,
		Limit:      pointer(pageSize),
		Name:       &requestName,
		SearchMode: &searchMode,
	}

	for {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("huaweicloud: resolve zone: %w", err)
		}

		response, err := p.client.ListPublicZones(request)
		if err != nil {
			return "", fmt.Errorf("huaweicloud: resolve zone: %w", err)
		}
		if response == nil || response.Zones == nil || len(*response.Zones) == 0 {
			break
		}

		items := *response.Zones
		for i := range items {
			if normalizeZoneName(stringValue(items[i].Name)) == zoneName {
				if id := strings.TrimSpace(stringValue(items[i].Id)); id != "" {
					return id, nil
				}
			}
		}

		if len(items) < int(pageSize) {
			break
		}

		nextMarker := stringValue(items[len(items)-1].Id)
		if nextMarker == "" || nextMarker == stringValue(request.Marker) {
			return "", fmt.Errorf("huaweicloud: resolve zone: pagination did not advance")
		}
		request.Marker = &nextMarker
	}

	return "", fmt.Errorf("huaweicloud: resolve zone: public zone %q not found", zoneName)
}
