package azuredns

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestRecordIDRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		relative   string
		recordType armdns.RecordType
		want       string
	}{
		{name: "apex", relative: "@", recordType: armdns.RecordTypeA, want: "@|A"},
		{name: "subdomain", relative: "www", recordType: armdns.RecordTypeA, want: "www|A"},
		{name: "underscore label", relative: "_acme-challenge", recordType: armdns.RecordTypeTXT, want: "_acme-challenge|TXT"},
		{name: "wildcard", relative: "*.dev", recordType: armdns.RecordTypeCNAME, want: "*.dev|CNAME"},
		{name: "service label", relative: "_sip._tcp", recordType: armdns.RecordTypeSRV, want: "_sip._tcp|SRV"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := newRecordID(tt.relative, tt.recordType)
			require.Equal(t, tt.want, id)

			name, recordType, err := parseRecordID(id)
			require.NoError(t, err)
			require.Equal(t, tt.relative, name)
			require.Equal(t, tt.recordType, recordType)
		})
	}
}

func TestParseRecordIDNormalizesInput(t *testing.T) {
	name, recordType, err := parseRecordID(" WWW|a ")
	require.NoError(t, err)
	require.Equal(t, "www", name)
	require.Equal(t, armdns.RecordTypeA, recordType)
}

func TestParseRecordIDRejectsInvalidInput(t *testing.T) {
	invalid := []string{
		"",
		"www",
		"|A",
		" |A",
		"www|",
		"www|FOO",
		"a|b|A",
		"a/b|A",
		"a%b|A",
		"a b|A",
		"a#b|A",
		"a?b|A",
	}

	for _, id := range invalid {
		t.Run(id, func(t *testing.T) {
			_, _, err := parseRecordID(id)
			require.Error(t, err)
		})
	}
}

func TestRecordIDIsURLPathSafe(t *testing.T) {
	safe := regexp.MustCompile(`^[a-zA-Z0-9@*._\-|]+$`)

	names := []string{"@", "www", "_acme-challenge", "*.dev", "_sip._tcp", "a.b.c"}
	for _, name := range names {
		for _, recordType := range armdns.PossibleRecordTypeValues() {
			id := newRecordID(name, recordType)
			require.True(t, safe.MatchString(id), "unsafe record id %q", id)
			require.NotContains(t, id, "/")
		}
	}
}

func TestNormalizeRelativeName(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "", want: apexName},
		{input: ".", want: apexName},
		{input: "@", want: apexName},
		{input: "  ", want: apexName},
		{input: "WWW", want: "www"},
		{input: "www.", want: "www"},
		{input: " www ", want: "www"},
		{input: "*.dev", want: "*.dev"},
		{input: "a/b", wantErr: true},
		{input: "a%b", wantErr: true},
		{input: "a#b", wantErr: true},
		{input: "a?b", wantErr: true},
		{input: "a|b", wantErr: true},
		{input: "a b", wantErr: true},
		{input: "a\tb", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeRelativeName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRelativeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		zone  string
		want  string
	}{
		{name: "fqdn", input: "www.example.com", zone: "example.com", want: "www"},
		{name: "apex fqdn", input: "example.com", zone: "example.com", want: apexName},
		{name: "already relative", input: "www", zone: "example.com", want: "www"},
		{name: "wildcard", input: "*.example.com", zone: "example.com", want: "*"},
		{name: "nested", input: "a.b.example.com", zone: "example.com", want: "a.b"},
		{name: "trailing dots", input: "www.example.com.", zone: "example.com.", want: "www"},
		{name: "empty", input: "", zone: "example.com", want: apexName},
		{name: "apex marker", input: "@", zone: "example.com", want: apexName},
		{name: "not a suffix", input: "notexample.com", zone: "example.com", want: "notexample.com"},
		{name: "uppercase", input: "WWW.EXAMPLE.COM", zone: "example.com", want: "www"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relativeName(tt.input, tt.zone)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestToRecordType(t *testing.T) {
	for _, recordType := range armdns.PossibleRecordTypeValues() {
		got, err := toRecordType(strings.ToLower(string(recordType)))
		require.NoError(t, err)
		require.Equal(t, recordType, got)
	}

	for _, invalid := range []string{"", "DS", "SVCB", "  "} {
		_, err := toRecordType(invalid)
		require.Error(t, err)
	}
}

func TestRecordTypeFromResourceType(t *testing.T) {
	tests := []struct {
		input string
		want  armdns.RecordType
		ok    bool
	}{
		{input: "Microsoft.Network/dnszones/A", want: armdns.RecordTypeA, ok: true},
		{input: "Microsoft.Network/dnsZones/TXT", want: armdns.RecordTypeTXT, ok: true},
		{input: "SRV", want: armdns.RecordTypeSRV, ok: true},
		{input: "Microsoft.Network/dnszones/DS", ok: false},
		{input: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := recordTypeFromResourceType(tt.input)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFlattenRecordSet(t *testing.T) {
	tests := []struct {
		name         string
		recordSet    *armdns.RecordSet
		wantOK       bool
		wantID       string
		wantName     string
		wantType     string
		wantContent  string
		wantTTL      int
		wantPriority *int
		wantWeight   *int
	}{
		{
			name: "single a record",
			recordSet: recordSet("www", "A", &armdns.RecordSetProperties{
				TTL:      ptr(int64(300)),
				ARecords: []*armdns.ARecord{{IPv4Address: ptr("1.2.3.4")}},
			}),
			wantOK:      true,
			wantID:      "www|A",
			wantName:    "www",
			wantType:    "A",
			wantContent: "1.2.3.4",
			wantTTL:     300,
		},
		{
			name: "multi value a record",
			recordSet: recordSet("www", "A", &armdns.RecordSetProperties{
				TTL: ptr(int64(60)),
				ARecords: []*armdns.ARecord{
					{IPv4Address: ptr("1.2.3.4")},
					{IPv4Address: ptr("5.6.7.8")},
				},
			}),
			wantOK:      true,
			wantID:      "www|A",
			wantName:    "www",
			wantType:    "A",
			wantContent: "1.2.3.4\n5.6.7.8",
			wantTTL:     60,
		},
		{
			name: "aaaa record",
			recordSet: recordSet("@", "AAAA", &armdns.RecordSetProperties{
				TTL:         ptr(int64(120)),
				AaaaRecords: []*armdns.AaaaRecord{{IPv6Address: ptr("2001:db8::1")}},
			}),
			wantOK:      true,
			wantID:      "@|AAAA",
			wantName:    "@",
			wantType:    "AAAA",
			wantContent: "2001:db8::1",
			wantTTL:     120,
		},
		{
			name: "cname record",
			recordSet: recordSet("blog", "CNAME", &armdns.RecordSetProperties{
				TTL:         ptr(int64(3600)),
				CnameRecord: &armdns.CnameRecord{Cname: ptr("target.example.net")},
			}),
			wantOK:      true,
			wantID:      "blog|CNAME",
			wantName:    "blog",
			wantType:    "CNAME",
			wantContent: "target.example.net",
			wantTTL:     3600,
		},
		{
			name: "chunked txt record",
			recordSet: recordSet("_acme-challenge", "TXT", &armdns.RecordSetProperties{
				TTL: ptr(int64(60)),
				TxtRecords: []*armdns.TxtRecord{
					{Value: []*string{ptr(strings.Repeat("a", 255)), ptr(strings.Repeat("b", 45))}},
				},
			}),
			wantOK:      true,
			wantID:      "_acme-challenge|TXT",
			wantName:    "_acme-challenge",
			wantType:    "TXT",
			wantContent: strings.Repeat("a", 255) + strings.Repeat("b", 45),
			wantTTL:     60,
		},
		{
			name: "multi value txt record",
			recordSet: recordSet("@", "TXT", &armdns.RecordSetProperties{
				TTL: ptr(int64(60)),
				TxtRecords: []*armdns.TxtRecord{
					{Value: []*string{ptr("first")}},
					{Value: []*string{ptr("second")}},
				},
			}),
			wantOK:      true,
			wantID:      "@|TXT",
			wantName:    "@",
			wantType:    "TXT",
			wantContent: "first\nsecond",
			wantTTL:     60,
		},
		{
			name: "mx record exposes priority",
			recordSet: recordSet("@", "MX", &armdns.RecordSetProperties{
				TTL: ptr(int64(3600)),
				MxRecords: []*armdns.MxRecord{
					{Preference: ptr(int32(10)), Exchange: ptr("mail1.example.com")},
					{Preference: ptr(int32(20)), Exchange: ptr("mail2.example.com")},
				},
			}),
			wantOK:       true,
			wantID:       "@|MX",
			wantName:     "@",
			wantType:     "MX",
			wantContent:  "10 mail1.example.com\n20 mail2.example.com",
			wantTTL:      3600,
			wantPriority: ptr(10),
		},
		{
			name: "srv record exposes priority and weight",
			recordSet: recordSet("_sip._tcp", "SRV", &armdns.RecordSetProperties{
				TTL: ptr(int64(300)),
				SrvRecords: []*armdns.SrvRecord{
					{Priority: ptr(int32(10)), Weight: ptr(int32(5)), Port: ptr(int32(5060)), Target: ptr("sip.example.com")},
				},
			}),
			wantOK:       true,
			wantID:       "_sip._tcp|SRV",
			wantName:     "_sip._tcp",
			wantType:     "SRV",
			wantContent:  "10 5 5060 sip.example.com",
			wantTTL:      300,
			wantPriority: ptr(10),
			wantWeight:   ptr(5),
		},
		{
			name: "caa record quotes value",
			recordSet: recordSet("@", "CAA", &armdns.RecordSetProperties{
				TTL:        ptr(int64(3600)),
				CaaRecords: []*armdns.CaaRecord{{Flags: ptr(int32(0)), Tag: ptr("issue"), Value: ptr("letsencrypt.org")}},
			}),
			wantOK:      true,
			wantID:      "@|CAA",
			wantName:    "@",
			wantType:    "CAA",
			wantContent: `0 issue "letsencrypt.org"`,
			wantTTL:     3600,
		},
		{
			name: "ns record",
			recordSet: recordSet("@", "NS", &armdns.RecordSetProperties{
				TTL: ptr(int64(172800)),
				NsRecords: []*armdns.NsRecord{
					{Nsdname: ptr("ns1-01.azure-dns.com.")},
					{Nsdname: ptr("ns2-01.azure-dns.net.")},
				},
			}),
			wantOK:      true,
			wantID:      "@|NS",
			wantName:    "@",
			wantType:    "NS",
			wantContent: "ns1-01.azure-dns.com.\nns2-01.azure-dns.net.",
			wantTTL:     172800,
		},
		{
			name: "ptr record",
			recordSet: recordSet("1", "PTR", &armdns.RecordSetProperties{
				TTL:        ptr(int64(3600)),
				PtrRecords: []*armdns.PtrRecord{{Ptrdname: ptr("host.example.com")}},
			}),
			wantOK:      true,
			wantID:      "1|PTR",
			wantName:    "1",
			wantType:    "PTR",
			wantContent: "host.example.com",
			wantTTL:     3600,
		},
		{
			name: "soa record",
			recordSet: recordSet("@", "SOA", &armdns.RecordSetProperties{
				TTL: ptr(int64(3600)),
				SoaRecord: &armdns.SoaRecord{
					Host:         ptr("ns1-01.azure-dns.com."),
					Email:        ptr("azuredns-hostmaster.microsoft.com"),
					SerialNumber: ptr(int64(1)),
					RefreshTime:  ptr(int64(3600)),
					RetryTime:    ptr(int64(300)),
					ExpireTime:   ptr(int64(2419200)),
					MinimumTTL:   ptr(int64(300)),
				},
			}),
			wantOK:      true,
			wantID:      "@|SOA",
			wantName:    "@",
			wantType:    "SOA",
			wantContent: "ns1-01.azure-dns.com. azuredns-hostmaster.microsoft.com 1 3600 300 2419200 300",
			wantTTL:     3600,
		},
		{
			name: "alias record set falls back to target resource",
			recordSet: recordSet("alias", "A", &armdns.RecordSetProperties{
				TTL:            ptr(int64(60)),
				TargetResource: &armdns.SubResource{ID: ptr("/subscriptions/s/resourceGroups/rg/providers/Microsoft.Network/publicIPAddresses/pip")},
			}),
			wantOK:      true,
			wantID:      "alias|A",
			wantName:    "alias",
			wantType:    "A",
			wantContent: "/subscriptions/s/resourceGroups/rg/providers/Microsoft.Network/publicIPAddresses/pip",
			wantTTL:     60,
		},
		{
			// Azure reports names relative to the zone already. Stripping the zone
			// suffix a second time would give this set the same ID as the real "www".
			name: "name that ends with the zone name is preserved",
			recordSet: recordSet("www.example.com", "A", &armdns.RecordSetProperties{
				TTL:      ptr(int64(60)),
				ARecords: []*armdns.ARecord{{IPv4Address: ptr("1.2.3.4")}},
			}),
			wantOK:      true,
			wantID:      "www.example.com|A",
			wantName:    "www.example.com",
			wantType:    "A",
			wantContent: "1.2.3.4",
			wantTTL:     60,
		},
		{
			name: "name equal to the zone name is preserved",
			recordSet: recordSet("example.com", "A", &armdns.RecordSetProperties{
				TTL:      ptr(int64(60)),
				ARecords: []*armdns.ARecord{{IPv4Address: ptr("1.2.3.4")}},
			}),
			wantOK:      true,
			wantID:      "example.com|A",
			wantName:    "example.com",
			wantType:    "A",
			wantContent: "1.2.3.4",
			wantTTL:     60,
		},
		{
			name:      "nil record set",
			recordSet: nil,
			wantOK:    false,
		},
		{
			name:      "nil properties",
			recordSet: recordSet("www", "A", nil),
			wantOK:    false,
		},
		{
			name:      "unknown record type",
			recordSet: recordSet("www", "DS", &armdns.RecordSetProperties{TTL: ptr(int64(60))}),
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, ok := flattenRecordSet(tt.recordSet)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}

			require.Equal(t, tt.wantID, record.ID)
			require.Equal(t, tt.wantName, record.Name)
			require.Equal(t, tt.wantType, record.Type)
			require.Equal(t, tt.wantContent, record.Content)
			require.Equal(t, tt.wantTTL, record.TTL)
			require.Equal(t, tt.wantPriority, record.Priority)
			require.Equal(t, tt.wantWeight, record.Weight)
		})
	}
}

func TestRecordSetPropertiesPerType(t *testing.T) {
	t.Run("a record", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "1.2.3.4\n5.6.7.8", TTL: 300}, armdns.RecordTypeA)
		require.NoError(t, err)
		require.Equal(t, int64(300), *props.TTL)
		require.Len(t, props.ARecords, 2)
		require.Equal(t, "1.2.3.4", *props.ARecords[0].IPv4Address)
		require.Equal(t, "5.6.7.8", *props.ARecords[1].IPv4Address)
	})

	t.Run("aaaa record", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "2001:db8::1", TTL: 60}, armdns.RecordTypeAAAA)
		require.NoError(t, err)
		require.Len(t, props.AaaaRecords, 1)
		require.Equal(t, "2001:db8::1", *props.AaaaRecords[0].IPv6Address)
	})

	t.Run("cname record preserves the value verbatim", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "target.example.net.", TTL: 60}, armdns.RecordTypeCNAME)
		require.NoError(t, err)
		require.Equal(t, "target.example.net.", *props.CnameRecord.Cname)
	})

	t.Run("cname record rejects multiple values", func(t *testing.T) {
		_, err := recordSetProperties(dns.RecordInput{Content: "a.example.net\nb.example.net", TTL: 60}, armdns.RecordTypeCNAME)
		require.Error(t, err)
	})

	t.Run("txt record chunks long values", func(t *testing.T) {
		value := strings.Repeat("x", 600)
		props, err := recordSetProperties(dns.RecordInput{Content: value, TTL: 60}, armdns.RecordTypeTXT)
		require.NoError(t, err)
		require.Len(t, props.TxtRecords, 1)
		require.Len(t, props.TxtRecords[0].Value, 3)
		require.Equal(t, value, joinTXT(props.TxtRecords[0].Value))
	})

	t.Run("txt record preserves the value verbatim", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: `"hello world"`, TTL: 60}, armdns.RecordTypeTXT)
		require.NoError(t, err)
		require.Equal(t, `"hello world"`, joinTXT(props.TxtRecords[0].Value))
	})

	t.Run("ns record preserves the value verbatim", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "ns1.example.net.\nns2.example.net.", TTL: 60}, armdns.RecordTypeNS)
		require.NoError(t, err)
		require.Len(t, props.NsRecords, 2)
		require.Equal(t, "ns1.example.net.", *props.NsRecords[0].Nsdname)
		require.Equal(t, "ns2.example.net.", *props.NsRecords[1].Nsdname)
	})

	t.Run("ptr record preserves the value verbatim", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "host.example.com.", TTL: 60}, armdns.RecordTypePTR)
		require.NoError(t, err)
		require.Equal(t, "host.example.com.", *props.PtrRecords[0].Ptrdname)
	})

	t.Run("mx record with inline preference", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "10 mail.example.com", TTL: 60}, armdns.RecordTypeMX)
		require.NoError(t, err)
		require.Equal(t, int32(10), *props.MxRecords[0].Preference)
		require.Equal(t, "mail.example.com", *props.MxRecords[0].Exchange)
	})

	t.Run("mx record falls back to priority field", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "mail.example.com", TTL: 60, Priority: ptr(20)}, armdns.RecordTypeMX)
		require.NoError(t, err)
		require.Equal(t, int32(20), *props.MxRecords[0].Preference)
	})

	t.Run("srv record with all fields", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: "10 5 5060 sip.example.com", TTL: 60}, armdns.RecordTypeSRV)
		require.NoError(t, err)
		require.Equal(t, int32(10), *props.SrvRecords[0].Priority)
		require.Equal(t, int32(5), *props.SrvRecords[0].Weight)
		require.Equal(t, int32(5060), *props.SrvRecords[0].Port)
		require.Equal(t, "sip.example.com", *props.SrvRecords[0].Target)
	})

	t.Run("srv record falls back to priority and weight fields", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{
			Content:  "5060 sip.example.com",
			TTL:      60,
			Priority: ptr(1),
			Weight:   ptr(2),
		}, armdns.RecordTypeSRV)
		require.NoError(t, err)
		require.Equal(t, int32(1), *props.SrvRecords[0].Priority)
		require.Equal(t, int32(2), *props.SrvRecords[0].Weight)
		require.Equal(t, int32(5060), *props.SrvRecords[0].Port)
	})

	t.Run("caa record with flags", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: `0 issue "letsencrypt.org"`, TTL: 60}, armdns.RecordTypeCAA)
		require.NoError(t, err)
		require.Equal(t, int32(0), *props.CaaRecords[0].Flags)
		require.Equal(t, "issue", *props.CaaRecords[0].Tag)
		require.Equal(t, "letsencrypt.org", *props.CaaRecords[0].Value)
	})

	t.Run("caa record without flags", func(t *testing.T) {
		props, err := recordSetProperties(dns.RecordInput{Content: `issue letsencrypt.org`, TTL: 60}, armdns.RecordTypeCAA)
		require.NoError(t, err)
		require.Equal(t, int32(0), *props.CaaRecords[0].Flags)
		require.Equal(t, "issue", *props.CaaRecords[0].Tag)
		require.Equal(t, "letsencrypt.org", *props.CaaRecords[0].Value)
	})

	t.Run("soa record is read only", func(t *testing.T) {
		_, err := recordSetProperties(dns.RecordInput{Content: "anything", TTL: 60}, armdns.RecordTypeSOA)
		require.Error(t, err)
	})

	t.Run("empty content is rejected", func(t *testing.T) {
		_, err := recordSetProperties(dns.RecordInput{Content: "  \n\n", TTL: 60}, armdns.RecordTypeA)
		require.Error(t, err)
	})
}

func TestRecordSetPropertiesRoundTripsThroughFlatten(t *testing.T) {
	// A record listed from Azure must produce content that rebuilds the same payload,
	// otherwise editing a record in the UI would silently mangle it.
	tests := []struct {
		name       string
		recordType armdns.RecordType
		recordSet  *armdns.RecordSet
	}{
		{
			name:       "mx",
			recordType: armdns.RecordTypeMX,
			recordSet: recordSet("@", "MX", &armdns.RecordSetProperties{
				TTL: ptr(int64(3600)),
				MxRecords: []*armdns.MxRecord{
					{Preference: ptr(int32(10)), Exchange: ptr("mail1.example.com")},
					{Preference: ptr(int32(20)), Exchange: ptr("mail2.example.com")},
				},
			}),
		},
		{
			name:       "srv",
			recordType: armdns.RecordTypeSRV,
			recordSet: recordSet("_sip._tcp", "SRV", &armdns.RecordSetProperties{
				TTL: ptr(int64(300)),
				SrvRecords: []*armdns.SrvRecord{
					{Priority: ptr(int32(10)), Weight: ptr(int32(5)), Port: ptr(int32(5060)), Target: ptr("sip.example.com")},
				},
			}),
		},
		{
			name:       "caa",
			recordType: armdns.RecordTypeCAA,
			recordSet: recordSet("@", "CAA", &armdns.RecordSetProperties{
				TTL:        ptr(int64(3600)),
				CaaRecords: []*armdns.CaaRecord{{Flags: ptr(int32(128)), Tag: ptr("issuewild"), Value: ptr("letsencrypt.org")}},
			}),
		},
		{
			name:       "multi value a",
			recordType: armdns.RecordTypeA,
			recordSet: recordSet("www", "A", &armdns.RecordSetProperties{
				TTL: ptr(int64(60)),
				ARecords: []*armdns.ARecord{
					{IPv4Address: ptr("1.2.3.4")},
					{IPv4Address: ptr("5.6.7.8")},
				},
			}),
		},
		{
			name:       "aaaa",
			recordType: armdns.RecordTypeAAAA,
			recordSet: recordSet("www", "AAAA", &armdns.RecordSetProperties{
				TTL:         ptr(int64(60)),
				AaaaRecords: []*armdns.AaaaRecord{{IPv6Address: ptr("2001:db8::1")}},
			}),
		},
		{
			name:       "cname keeps its trailing dot",
			recordType: armdns.RecordTypeCNAME,
			recordSet: recordSet("blog", "CNAME", &armdns.RecordSetProperties{
				TTL:         ptr(int64(3600)),
				CnameRecord: &armdns.CnameRecord{Cname: ptr("target.example.net.")},
			}),
		},
		{
			name:       "ns keeps its trailing dots",
			recordType: armdns.RecordTypeNS,
			recordSet: recordSet("@", "NS", &armdns.RecordSetProperties{
				TTL: ptr(int64(172800)),
				NsRecords: []*armdns.NsRecord{
					{Nsdname: ptr("ns1-01.azure-dns.com.")},
					{Nsdname: ptr("ns2-01.azure-dns.net.")},
				},
			}),
		},
		{
			name:       "ptr keeps its trailing dot",
			recordType: armdns.RecordTypePTR,
			recordSet: recordSet("1", "PTR", &armdns.RecordSetProperties{
				TTL:        ptr(int64(3600)),
				PtrRecords: []*armdns.PtrRecord{{Ptrdname: ptr("host.example.com.")}},
			}),
		},
		{
			name:       "chunked txt reassembles and re-splits",
			recordType: armdns.RecordTypeTXT,
			recordSet: recordSet("_acme-challenge", "TXT", &armdns.RecordSetProperties{
				TTL: ptr(int64(60)),
				TxtRecords: []*armdns.TxtRecord{
					{Value: []*string{ptr(strings.Repeat("a", 255)), ptr(strings.Repeat("b", 45))}},
				},
			}),
		},
		{
			name:       "quoted txt value survives untouched",
			recordType: armdns.RecordTypeTXT,
			recordSet: recordSet("@", "TXT", &armdns.RecordSetProperties{
				TTL:        ptr(int64(60)),
				TxtRecords: []*armdns.TxtRecord{{Value: []*string{ptr(`"v=spf1 -all"`)}}},
			}),
		},
		{
			name:       "mx without an explicit preference",
			recordType: armdns.RecordTypeMX,
			recordSet: recordSet("@", "MX", &armdns.RecordSetProperties{
				TTL:       ptr(int64(3600)),
				MxRecords: []*armdns.MxRecord{{Preference: ptr(int32(0)), Exchange: ptr("mail.example.com.")}},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, ok := flattenRecordSet(tt.recordSet)
			require.True(t, ok)

			props, err := recordSetProperties(dns.RecordInput{
				Type:     record.Type,
				Name:     record.Name,
				Content:  record.Content,
				TTL:      record.TTL,
				Priority: record.Priority,
				Weight:   record.Weight,
			}, tt.recordType)
			require.NoError(t, err)

			rebuilt := recordFrom(record.Name, tt.recordType, props)
			require.Equal(t, record, rebuilt)
		})
	}
}

func TestParseMXValue(t *testing.T) {
	_, err := parseMXValue("abc mail.example.com", nil)
	require.Error(t, err)

	_, err = parseMXValue("10 20 mail.example.com", nil)
	require.Error(t, err)

	record, err := parseMXValue("mail.example.com.", nil)
	require.NoError(t, err)
	require.Equal(t, int32(0), *record.Preference)
	require.Equal(t, "mail.example.com.", *record.Exchange)
}

func TestParseSRVValue(t *testing.T) {
	record, err := parseSRVValue("5 5060 sip.example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, int32(0), *record.Priority)
	require.Equal(t, int32(5), *record.Weight)
	require.Equal(t, int32(5060), *record.Port)

	_, err = parseSRVValue("sip.example.com", nil, nil)
	require.Error(t, err)

	_, err = parseSRVValue("10 5 abc sip.example.com", nil, nil)
	require.Error(t, err)

	_, err = parseSRVValue("10 5 70000 sip.example.com", nil, nil)
	require.Error(t, err)
}

func TestParseCAAValue(t *testing.T) {
	record, err := parseCAAValue(`0 issue "ca.example.com; account=1"`)
	require.NoError(t, err)
	require.Equal(t, int32(0), *record.Flags)
	require.Equal(t, "issue", *record.Tag)
	require.Equal(t, "ca.example.com; account=1", *record.Value)

	_, err = parseCAAValue("issue")
	require.Error(t, err)

	_, err = parseCAAValue("0 issue")
	require.Error(t, err)
}

func TestChunkAndJoinTXT(t *testing.T) {
	values := []string{
		"",
		"a",
		strings.Repeat("x", 254),
		strings.Repeat("x", 255),
		strings.Repeat("x", 256),
		strings.Repeat("x", 600),
		strings.Repeat("x", 1024),
	}

	for _, value := range values {
		chunks := chunkTXT(value)
		require.NotEmpty(t, chunks)
		for _, chunk := range chunks {
			require.LessOrEqual(t, len(*chunk), txtChunkSize)
		}
		require.Equal(t, value, joinTXT(chunks))
	}
}

func TestChunkTXTKeepsRunesIntact(t *testing.T) {
	// A chunk cut mid rune is invalid UTF-8, and the JSON encoder used for the
	// request body would rewrite those bytes to U+FFFD before Azure ever sees them.
	values := []string{
		strings.Repeat("a", 254) + "中" + strings.Repeat("b", 100),
		strings.Repeat("a", 253) + "中" + strings.Repeat("b", 100),
		strings.Repeat("中", 200),
		strings.Repeat("a", 254) + "🎉" + strings.Repeat("b", 100),
		"中文测试",
	}

	for _, value := range values {
		chunks := chunkTXT(value)

		for i, chunk := range chunks {
			require.LessOrEqual(t, len(*chunk), txtChunkSize)
			require.True(t, utf8.ValidString(*chunk), "chunk %d of %q is not valid UTF-8", i, value)
		}

		require.Equal(t, value, joinTXT(chunks))

		// The wire format must survive a JSON round trip unchanged.
		encoded, err := json.Marshal(chunks)
		require.NoError(t, err)
		var decoded []*string
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		require.Equal(t, value, joinTXT(decoded))
	}
}

func TestFallbackPriorityAndWeightAreRangeChecked(t *testing.T) {
	// api/dns/dto.go does not range check these form fields, so an out of range
	// value must be rejected rather than silently truncated by an int32 cast.
	for _, out := range []int{-1, 65536, 4294967296} {
		_, err := parseMXValue("mail.example.com", ptr(out))
		require.Error(t, err, "MX preference %d should be rejected", out)

		_, err = parseSRVValue("5060 sip.example.com", ptr(out), ptr(0))
		require.Error(t, err, "SRV priority %d should be rejected", out)

		_, err = parseSRVValue("5060 sip.example.com", ptr(0), ptr(out))
		require.Error(t, err, "SRV weight %d should be rejected", out)
	}

	record, err := parseMXValue("mail.example.com", ptr(65535))
	require.NoError(t, err)
	require.Equal(t, int32(65535), *record.Preference)
}

func TestSplitValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "single", input: "1.2.3.4", want: []string{"1.2.3.4"}},
		{name: "unix newlines", input: "1.2.3.4\n5.6.7.8", want: []string{"1.2.3.4", "5.6.7.8"}},
		{name: "windows newlines", input: "1.2.3.4\r\n5.6.7.8", want: []string{"1.2.3.4", "5.6.7.8"}},
		{name: "blank lines dropped", input: "\n1.2.3.4\n\n\n5.6.7.8\n", want: []string{"1.2.3.4", "5.6.7.8"}},
		{name: "whitespace trimmed", input: "  1.2.3.4  ", want: []string{"1.2.3.4"}},
		{name: "empty", input: "", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, splitValues(tt.input))
		})
	}
}

func TestMatchesFilter(t *testing.T) {
	record := dns.Record{Name: "www", Type: "A"}

	require.True(t, matchesFilter(record, dns.RecordFilter{}))
	require.True(t, matchesFilter(record, dns.RecordFilter{Type: "a"}))
	require.True(t, matchesFilter(record, dns.RecordFilter{Name: "WW"}))
	require.True(t, matchesFilter(record, dns.RecordFilter{Type: "A", Name: "www"}))
	require.False(t, matchesFilter(record, dns.RecordFilter{Type: "TXT"}))
	require.False(t, matchesFilter(record, dns.RecordFilter{Name: "api"}))
}

func TestNormalizeTTL(t *testing.T) {
	require.Equal(t, int64(1), normalizeTTL(0))
	require.Equal(t, int64(1), normalizeTTL(-5))
	require.Equal(t, int64(1), normalizeTTL(1))
	require.Equal(t, int64(600), normalizeTTL(600))
	require.Equal(t, int64(2147483647), normalizeTTL(2147483648))
}

func recordSet(name, recordType string, props *armdns.RecordSetProperties) *armdns.RecordSet {
	return &armdns.RecordSet{
		Name:       ptr(name),
		Type:       ptr("Microsoft.Network/dnszones/" + recordType),
		Properties: props,
	}
}
