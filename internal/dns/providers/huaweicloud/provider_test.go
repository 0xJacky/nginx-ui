package huaweicloud

import (
	"context"
	"testing"

	hwmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestListRecordsMapsHuaweiRecordSets(t *testing.T) {
	client := newFakeHuaweiDNSClient()
	client.recordSetsResponse = &hwmodel.ListRecordSetsWithLineResponse{
		Recordsets: pointer([]hwmodel.QueryRecordSetWithLineAndTagsResp{
			{
				Id:       pointer("record-a"),
				Name:     pointer("www.example.com."),
				ZoneName: pointer("example.com."),
				Type:     pointer("A"),
				Ttl:      pointer(int32(300)),
				Records:  pointer([]string{"192.0.2.1", "192.0.2.2"}),
				Line:     pointer("custom_line"),
				Weight:   pointer(int32(20)),
			},
			{
				Id:       pointer("record-mx"),
				Name:     pointer("example.com."),
				ZoneName: pointer("example.com."),
				Type:     pointer("MX"),
				Ttl:      pointer(int32(600)),
				Records:  pointer([]string{"10 mail1.example.com.", "10 mail2.example.com."}),
				Line:     pointer(defaultRecordLine),
			},
		}),
	}

	records, err := (&provider{client: client}).ListRecords(context.Background(), "example.com", dns.RecordFilter{
		Type: "a",
		Name: "www",
	})
	require.NoError(t, err)
	require.Len(t, records, 2)

	require.Equal(t, dns.Record{
		ID:       "record-mx",
		Type:     "MX",
		Name:     "@",
		Content:  "mail1.example.com.\nmail2.example.com.",
		TTL:      600,
		Line:     defaultRecordLine,
		Priority: pointer(10),
	}, records[0])
	require.Equal(t, "www", records[1].Name)
	require.Equal(t, "192.0.2.1\n192.0.2.2", records[1].Content)
	require.Equal(t, "custom_line", records[1].Line)
	require.Equal(t, 20, *records[1].Weight)

	require.NotNil(t, client.recordSetsRequest)
	require.Equal(t, "zone-1", stringValue(client.recordSetsRequest.ZoneId))
	require.Equal(t, "A", stringValue(client.recordSetsRequest.Type))
	require.Equal(t, "www.example.com.", stringValue(client.recordSetsRequest.Name))
}

func TestListRecordLinesMapsDefaultAndCustomLines(t *testing.T) {
	client := newFakeHuaweiDNSClient()
	client.linesResponse = &hwmodel.ListPublicZoneLinesResponse{
		Lines: pointer([]hwmodel.PublicZoneLines{
			{Line: pointer(defaultRecordLine)},
			{Line: pointer("custom_1"), LineName: pointer("Mainland")},
			{LineName: pointer("missing id")},
		}),
	}

	lines, err := (&provider{client: client}).ListRecordLines(context.Background(), "example.com")
	require.NoError(t, err)
	require.Equal(t, []dns.RecordLine{
		{Code: defaultRecordLine, Name: "Default"},
		{Code: "custom_1", Name: "Mainland"},
	}, lines)
	require.Equal(t, "zone-1", client.linesRequest.ZoneId)
}

func TestCreateRecordUsesLineAndHuaweiValueFormats(t *testing.T) {
	client := newFakeHuaweiDNSClient()
	client.createResponse = &hwmodel.CreateRecordSetWithLineResponse{
		Id:       pointer("record-1"),
		Name:     pointer("example.com."),
		ZoneName: pointer("example.com."),
		Type:     pointer("MX"),
		Ttl:      pointer(int32(600)),
		Records:  pointer([]string{"10 mail1.example.com.", "10 mail2.example.com."}),
		Line:     pointer("custom_1"),
	}

	priority := 10
	line := "custom_1"
	record, err := (&provider{client: client}).CreateRecord(context.Background(), "example.com", dns.RecordInput{
		Type:     "mx",
		Name:     "@",
		Content:  "mail1.example.com.\nmail2.example.com.",
		TTL:      600,
		Line:     &line,
		Priority: &priority,
	})
	require.NoError(t, err)
	require.Equal(t, "record-1", record.ID)
	require.Equal(t, "mail1.example.com.\nmail2.example.com.", record.Content)
	require.Equal(t, priority, *record.Priority)

	body := client.createRequest.Body
	require.Equal(t, "example.com.", body.Name)
	require.Equal(t, "MX", body.Type)
	require.Equal(t, "custom_1", stringValue(body.Line))
	require.Equal(t, []string{"10 mail1.example.com.", "10 mail2.example.com."}, stringSliceValue(body.Records))
}

func TestCreateRecordDefaultsLineAndQuotesTXT(t *testing.T) {
	client := newFakeHuaweiDNSClient()
	client.createResponse = &hwmodel.CreateRecordSetWithLineResponse{
		Id:       pointer("record-1"),
		Name:     pointer("_acme-challenge.example.com."),
		ZoneName: pointer("example.com."),
		Type:     pointer("TXT"),
		Ttl:      pointer(int32(300)),
		Records:  pointer([]string{"\"token\""}),
		Line:     pointer(defaultRecordLine),
	}

	_, err := (&provider{client: client}).CreateRecord(context.Background(), "example.com", dns.RecordInput{
		Type:    "TXT",
		Name:    "_acme-challenge",
		Content: "token",
		TTL:     300,
	})
	require.NoError(t, err)
	require.Equal(t, defaultRecordLine, stringValue(client.createRequest.Body.Line))
	require.Equal(t, []string{"\"token\""}, stringSliceValue(client.createRequest.Body.Records))
}

func TestUpdateRecordPreservesExistingLine(t *testing.T) {
	client := newFakeHuaweiDNSClient()
	client.showResponse = &hwmodel.ShowRecordSetWithLineResponse{
		Id:   pointer("record-1"),
		Line: pointer("custom_1"),
	}
	client.updateResponse = &hwmodel.UpdateRecordSetsResponse{
		Id:       pointer("record-1"),
		Name:     pointer("www.example.com."),
		ZoneName: pointer("example.com."),
		Type:     pointer("A"),
		Ttl:      pointer(int32(120)),
		Records:  pointer([]string{"192.0.2.10"}),
		Line:     pointer("custom_1"),
	}

	record, err := (&provider{client: client}).UpdateRecord(context.Background(), "example.com", "record-1", dns.RecordInput{
		Type:    "A",
		Name:    "www",
		Content: "192.0.2.10",
		TTL:     120,
	})
	require.NoError(t, err)
	require.Equal(t, "custom_1", record.Line)
	require.Equal(t, "www.example.com.", client.updateRequest.Body.Name)
}

func TestUpdateRecordRejectsResolutionLineChange(t *testing.T) {
	client := newFakeHuaweiDNSClient()
	client.showResponse = &hwmodel.ShowRecordSetWithLineResponse{
		Id:   pointer("record-1"),
		Line: pointer("custom_1"),
	}

	requestedLine := "custom_2"
	_, err := (&provider{client: client}).UpdateRecord(context.Background(), "example.com", "record-1", dns.RecordInput{
		Type:    "A",
		Name:    "www",
		Content: "192.0.2.10",
		TTL:     120,
		Line:    &requestedLine,
	})
	require.EqualError(t, err, "huaweicloud: changing a record resolution line is not supported; create a new record on line \"custom_2\"")
	require.Nil(t, client.updateRequest)
}

func TestSRVValuesRoundTripPriorityAndWeight(t *testing.T) {
	content, priority, weight := recordContent("SRV", []string{
		"10 20 443 service1.example.com.",
		"10 20 8443 service2.example.com.",
	})
	require.Equal(t, "443 service1.example.com.\n8443 service2.example.com.", content)
	require.Equal(t, 10, *priority)
	require.Equal(t, 20, *weight)

	values, err := recordValuesFromInput("SRV", dns.RecordInput{
		Content:  content,
		TTL:      300,
		Priority: priority,
		Weight:   weight,
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"10 20 443 service1.example.com.",
		"10 20 8443 service2.example.com.",
	}, values)
}

func TestDeleteRecordUsesResolvedZone(t *testing.T) {
	client := newFakeHuaweiDNSClient()
	err := (&provider{client: client}).DeleteRecord(context.Background(), "example.com", "record-1")
	require.NoError(t, err)
	require.Equal(t, "zone-1", client.deleteRequest.ZoneId)
	require.Equal(t, "record-1", client.deleteRequest.RecordsetId)
}

func TestNewProviderRequiresHuaweiCredentials(t *testing.T) {
	_, err := newProvider(&dns.Credential{Values: map[string]string{}})
	require.EqualError(t, err, "huaweicloud: missing access key id, secret access key, or region")
}

type fakeHuaweiDNSClient struct {
	zonesResponse      *hwmodel.ListPublicZonesResponse
	recordSetsRequest  *hwmodel.ListRecordSetsWithLineRequest
	recordSetsResponse *hwmodel.ListRecordSetsWithLineResponse
	createRequest      *hwmodel.CreateRecordSetWithLineRequest
	createResponse     *hwmodel.CreateRecordSetWithLineResponse
	updateRequest      *hwmodel.UpdateRecordSetsRequest
	updateResponse     *hwmodel.UpdateRecordSetsResponse
	deleteRequest      *hwmodel.DeleteRecordSetsRequest
	showRequest        *hwmodel.ShowRecordSetWithLineRequest
	showResponse       *hwmodel.ShowRecordSetWithLineResponse
	linesRequest       *hwmodel.ListPublicZoneLinesRequest
	linesResponse      *hwmodel.ListPublicZoneLinesResponse
}

func newFakeHuaweiDNSClient() *fakeHuaweiDNSClient {
	return &fakeHuaweiDNSClient{
		zonesResponse: &hwmodel.ListPublicZonesResponse{
			Zones: pointer([]hwmodel.PublicZoneResp{{
				Id:   pointer("zone-1"),
				Name: pointer("example.com."),
			}}),
		},
		deleteRequest: nil,
	}
}

func (c *fakeHuaweiDNSClient) ListPublicZones(*hwmodel.ListPublicZonesRequest) (*hwmodel.ListPublicZonesResponse, error) {
	return c.zonesResponse, nil
}

func (c *fakeHuaweiDNSClient) ListRecordSetsWithLine(request *hwmodel.ListRecordSetsWithLineRequest) (*hwmodel.ListRecordSetsWithLineResponse, error) {
	c.recordSetsRequest = request
	return c.recordSetsResponse, nil
}

func (c *fakeHuaweiDNSClient) CreateRecordSetWithLine(request *hwmodel.CreateRecordSetWithLineRequest) (*hwmodel.CreateRecordSetWithLineResponse, error) {
	c.createRequest = request
	return c.createResponse, nil
}

func (c *fakeHuaweiDNSClient) UpdateRecordSets(request *hwmodel.UpdateRecordSetsRequest) (*hwmodel.UpdateRecordSetsResponse, error) {
	c.updateRequest = request
	return c.updateResponse, nil
}

func (c *fakeHuaweiDNSClient) DeleteRecordSets(request *hwmodel.DeleteRecordSetsRequest) (*hwmodel.DeleteRecordSetsResponse, error) {
	c.deleteRequest = request
	return &hwmodel.DeleteRecordSetsResponse{}, nil
}

func (c *fakeHuaweiDNSClient) ShowRecordSetWithLine(request *hwmodel.ShowRecordSetWithLineRequest) (*hwmodel.ShowRecordSetWithLineResponse, error) {
	c.showRequest = request
	return c.showResponse, nil
}

func (c *fakeHuaweiDNSClient) ListPublicZoneLines(request *hwmodel.ListPublicZoneLinesRequest) (*hwmodel.ListPublicZoneLinesResponse, error) {
	c.linesRequest = request
	return c.linesResponse, nil
}
