package tencentcloud

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

// Domains are persisted as punycode, but a database that predates that still holds
// the Unicode spelling, so both have to reach DNSPod as punycode.
func TestListRecordsSendsPunycodeDomain(t *testing.T) {
	t.Parallel()

	for _, stored := range []string{"xn--fsq.example.com", "例.example.com"} {
		client := &fakeTencentDNSClient{handler: func(_ string, _, response any) error {
			return copyJSON(response, listRecordsResponse{})
		}}

		_, err := (&provider{client: client}).ListRecords(t.Context(), stored, dns.RecordFilter{Name: "例"})
		require.NoError(t, err)
		require.Len(t, client.calls, 1)

		request := client.calls[0].request.(recordListRequest)
		require.Equal(t, "xn--fsq.example.com", request.Domain,
			"stored spelling %q must be sent as punycode", stored)
		require.Equal(t, "xn--fsq", request.Subdomain)
	}
}

func TestRecordMutationSendsPunycodeDomainAndLabel(t *testing.T) {
	t.Parallel()

	request := newRecordMutationRequest("例.example.com", 0, dns.RecordInput{
		Type: "A", Name: "例", Content: "192.0.2.1",
	})
	require.Equal(t, "xn--fsq.example.com", request.Domain)
	require.Equal(t, "xn--fsq", request.Subdomain)
}

func TestDeleteRecordSendsPunycodeDomain(t *testing.T) {
	t.Parallel()

	client := &fakeTencentDNSClient{handler: func(_ string, _, _ any) error { return nil }}
	require.NoError(t, (&provider{client: client}).DeleteRecord(t.Context(), "例.example.com", "42"))

	request := client.calls[0].request.(recordReferenceRequest)
	require.Equal(t, "xn--fsq.example.com", request.Domain)
}

func TestNormalizeSubDomainPreservesDNSConstructs(t *testing.T) {
	t.Parallel()

	require.Equal(t, "xn--fsq", normalizeSubDomain("例"))
	require.Equal(t, "@", normalizeSubDomain("@"))
	require.Equal(t, "@", normalizeSubDomain(""))
	require.Equal(t, "*", normalizeSubDomain("*"))
	require.Equal(t, "_acme-challenge", normalizeSubDomain("_acme-challenge"))
}
