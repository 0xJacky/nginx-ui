package alidns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

// Domains are persisted as punycode, but a database that predates that still holds
// the Unicode spelling, so both have to reach Aliyun as punycode.
func TestListRecordsSendsPunycodeDomainName(t *testing.T) {
	t.Parallel()

	for _, stored := range []string{"xn--fsq.example.com", "例.example.com"} {
		client := &fakeAliDNSClient{handler: func(_ string, _ map[string]any, result any) error {
			return copyJSON(result, describeDomainRecordsResponse{})
		}}

		_, err := (&provider{client: client}).ListRecords(t.Context(), stored, dns.RecordFilter{})
		require.NoError(t, err)
		require.Len(t, client.calls, 1)
		require.Equal(t, "xn--fsq.example.com", client.calls[0].query["DomainName"],
			"stored spelling %q must be sent as punycode", stored)
	}
}

func TestCreateRecordSendsPunycodeDomainAndLabel(t *testing.T) {
	t.Parallel()

	client := &fakeAliDNSClient{handler: func(action string, _ map[string]any, result any) error {
		if action == "AddDomainRecord" {
			return copyJSON(result, recordIDResponse{RecordID: "record-1"})
		}
		return copyJSON(result, domainRecord{RecordID: "record-1", Type: "A", RR: "xn--fsq", Value: "192.0.2.1"})
	}}

	_, err := (&provider{client: client}).CreateRecord(t.Context(), "例.example.com", dns.RecordInput{
		Type: "A", Name: "例", Content: "192.0.2.1", TTL: 600,
	})
	require.NoError(t, err)
	require.Equal(t, "xn--fsq.example.com", client.calls[0].query["DomainName"])
	require.Equal(t, "xn--fsq", client.calls[0].query["RR"])
}

func TestListRecordLinesSendsPunycodeDomainName(t *testing.T) {
	t.Parallel()

	client := &fakeAliDNSClient{handler: func(_ string, _ map[string]any, result any) error {
		return copyJSON(result, describeSupportLinesResponse{})
	}}

	_, err := (&provider{client: client}).ListRecordLines(t.Context(), "例.example.com")
	require.NoError(t, err)
	require.Equal(t, "xn--fsq.example.com", client.calls[0].query["DomainName"])
}

func TestRRFromNamePreservesDNSConstructs(t *testing.T) {
	t.Parallel()

	require.Equal(t, "xn--fsq", rrFromName("例"))
	require.Equal(t, "@", rrFromName("@"))
	require.Equal(t, "@", rrFromName(""))
	require.Equal(t, "*", rrFromName("*"))
	require.Equal(t, "_acme-challenge", rrFromName("_acme-challenge"))
	require.Equal(t, "www", rrFromName(" www "))
}
