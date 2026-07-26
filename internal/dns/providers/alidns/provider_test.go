package alidns

import (
	"context"
	"testing"

	aliclient "github.com/alibabacloud-go/alidns-20150109/v5/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestListRecordsMapsResolutionLine(t *testing.T) {
	client := &fakeAliDNSClient{
		describeRecordsResponse: &aliclient.DescribeDomainRecordsResponse{
			Body: &aliclient.DescribeDomainRecordsResponseBody{
				DomainRecords: &aliclient.DescribeDomainRecordsResponseBodyDomainRecords{
					Record: []*aliclient.DescribeDomainRecordsResponseBodyDomainRecordsRecord{
						{
							RecordId: dara.String("record-1"),
							Type:     dara.String("A"),
							RR:       dara.String("www"),
							Value:    dara.String("192.0.2.1"),
							TTL:      dara.Int64(600),
							Line:     dara.String("telecom"),
						},
					},
				},
			},
		},
	}

	records, err := (&provider{client: client}).ListRecords(context.Background(), "example.com", dns.RecordFilter{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "telecom", records[0].Line)
}

func TestListRecordLinesMapsSupportedLines(t *testing.T) {
	client := &fakeAliDNSClient{
		describeSupportLinesResponse: &aliclient.DescribeSupportLinesResponse{
			Body: &aliclient.DescribeSupportLinesResponseBody{
				RecordLines: &aliclient.DescribeSupportLinesResponseBodyRecordLines{
					RecordLine: []*aliclient.DescribeSupportLinesResponseBodyRecordLinesRecordLine{
						{
							LineCode:        dara.String("default"),
							LineName:        dara.String("Default"),
							LineDisplayName: dara.String("Default line"),
						},
						{
							FatherCode:      dara.String("telecom"),
							LineCode:        dara.String("cn_telecom_zhejiang"),
							LineName:        dara.String("Zhejiang Telecom"),
							LineDisplayName: dara.String("China Telecom / Zhejiang"),
						},
						{LineName: dara.String("missing code")},
					},
				},
			},
		},
	}

	lines, err := (&provider{client: client}).ListRecordLines(context.Background(), "example.com")
	require.NoError(t, err)
	require.Equal(t, "example.com", dara.StringValue(client.describeSupportLinesRequest.DomainName))
	require.Equal(t, []dns.RecordLine{
		{Code: "default", Name: "Default", DisplayName: "Default line"},
		{
			Code:        "cn_telecom_zhejiang",
			Name:        "Zhejiang Telecom",
			DisplayName: "China Telecom / Zhejiang",
			FatherCode:  "telecom",
		},
	}, lines)
}

func TestCreateRecordUsesRequestedOrDefaultLine(t *testing.T) {
	for _, tc := range []struct {
		name     string
		line     *string
		expected string
	}{
		{name: "requested line", line: dara.String("telecom"), expected: "telecom"},
		{name: "default line", expected: defaultLineName},
		{name: "blank line", line: dara.String("  "), expected: defaultLineName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newRecordMutationClient(tc.expected)
			_, err := (&provider{client: client}).CreateRecord(context.Background(), "example.com", dns.RecordInput{
				Type:    "A",
				Name:    "www",
				Content: "192.0.2.1",
				TTL:     600,
				Line:    tc.line,
			})
			require.NoError(t, err)
			require.Equal(t, tc.expected, dara.StringValue(client.addRecordRequest.Line))
		})
	}
}

func TestUpdateRecordPreservesLineWhenOmitted(t *testing.T) {
	client := newRecordMutationClient("telecom")
	_, err := (&provider{client: client}).UpdateRecord(context.Background(), "example.com", "record-1", dns.RecordInput{
		Type:    "A",
		Name:    "www",
		Content: "192.0.2.2",
		TTL:     600,
	})
	require.NoError(t, err)
	require.Equal(t, "telecom", dara.StringValue(client.updateRecordRequest.Line))
	require.Equal(t, 2, client.describeRecordInfoCalls)
}

func TestUpdateRecordUsesRequestedLineWithoutPrefetch(t *testing.T) {
	client := newRecordMutationClient("mobile")
	_, err := (&provider{client: client}).UpdateRecord(context.Background(), "example.com", "record-1", dns.RecordInput{
		Type:    "A",
		Name:    "www",
		Content: "192.0.2.2",
		TTL:     600,
		Line:    dara.String("mobile"),
	})
	require.NoError(t, err)
	require.Equal(t, "mobile", dara.StringValue(client.updateRecordRequest.Line))
	require.Equal(t, 1, client.describeRecordInfoCalls)
}

func newRecordMutationClient(line string) *fakeAliDNSClient {
	return &fakeAliDNSClient{
		addRecordResponse: &aliclient.AddDomainRecordResponse{
			Body: &aliclient.AddDomainRecordResponseBody{RecordId: dara.String("record-1")},
		},
		updateRecordResponse: &aliclient.UpdateDomainRecordResponse{},
		describeRecordInfoResponse: &aliclient.DescribeDomainRecordInfoResponse{
			Body: &aliclient.DescribeDomainRecordInfoResponseBody{
				RecordId: dara.String("record-1"),
				Type:     dara.String("A"),
				RR:       dara.String("www"),
				Value:    dara.String("192.0.2.2"),
				TTL:      dara.Int64(600),
				Line:     dara.String(line),
			},
		},
	}
}

type fakeAliDNSClient struct {
	describeRecordsRequest       *aliclient.DescribeDomainRecordsRequest
	describeRecordsResponse      *aliclient.DescribeDomainRecordsResponse
	addRecordRequest             *aliclient.AddDomainRecordRequest
	addRecordResponse            *aliclient.AddDomainRecordResponse
	updateRecordRequest          *aliclient.UpdateDomainRecordRequest
	updateRecordResponse         *aliclient.UpdateDomainRecordResponse
	describeRecordInfoResponse   *aliclient.DescribeDomainRecordInfoResponse
	describeRecordInfoCalls      int
	describeSupportLinesRequest  *aliclient.DescribeSupportLinesRequest
	describeSupportLinesResponse *aliclient.DescribeSupportLinesResponse
}

func (c *fakeAliDNSClient) DescribeDomainRecordsWithOptions(request *aliclient.DescribeDomainRecordsRequest, _ *dara.RuntimeOptions) (*aliclient.DescribeDomainRecordsResponse, error) {
	c.describeRecordsRequest = request
	return c.describeRecordsResponse, nil
}

func (c *fakeAliDNSClient) AddDomainRecordWithOptions(request *aliclient.AddDomainRecordRequest, _ *dara.RuntimeOptions) (*aliclient.AddDomainRecordResponse, error) {
	c.addRecordRequest = request
	return c.addRecordResponse, nil
}

func (c *fakeAliDNSClient) UpdateDomainRecordWithOptions(request *aliclient.UpdateDomainRecordRequest, _ *dara.RuntimeOptions) (*aliclient.UpdateDomainRecordResponse, error) {
	c.updateRecordRequest = request
	return c.updateRecordResponse, nil
}

func (c *fakeAliDNSClient) DeleteDomainRecordWithOptions(_ *aliclient.DeleteDomainRecordRequest, _ *dara.RuntimeOptions) (*aliclient.DeleteDomainRecordResponse, error) {
	return &aliclient.DeleteDomainRecordResponse{}, nil
}

func (c *fakeAliDNSClient) DescribeDomainRecordInfoWithOptions(_ *aliclient.DescribeDomainRecordInfoRequest, _ *dara.RuntimeOptions) (*aliclient.DescribeDomainRecordInfoResponse, error) {
	c.describeRecordInfoCalls++
	return c.describeRecordInfoResponse, nil
}

func (c *fakeAliDNSClient) DescribeSupportLinesWithOptions(request *aliclient.DescribeSupportLinesRequest, _ *dara.RuntimeOptions) (*aliclient.DescribeSupportLinesResponse, error) {
	c.describeSupportLinesRequest = request
	return c.describeSupportLinesResponse, nil
}
