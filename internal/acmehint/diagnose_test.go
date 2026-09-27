package acmehint

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	mu      sync.Mutex
	records map[string][]string
	errs    map[string]error
	queried []string
}

func (f *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	f.mu.Lock()
	f.queried = append(f.queried, host)
	f.mu.Unlock()
	if err := f.errs[host]; err != nil {
		return nil, err
	}
	var out []net.IPAddr
	for _, s := range f.records[host] {
		out = append(out, net.IPAddr{IP: net.ParseIP(s)})
	}
	return out, nil
}

type fakeCAA map[string][]CAARecord

func (f fakeCAA) LookupCAA(_ context.Context, name string) ([]CAARecord, error) {
	return f[name], nil
}

func localAddrs(addrs ...string) func() ([]net.IP, error) {
	return func() ([]net.IP, error) {
		ips := make([]net.IP, 0, len(addrs))
		for _, a := range addrs {
			ips = append(ips, net.ParseIP(a))
		}
		return ips, nil
	}
}

func codes(diags []Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func TestDiagnose(t *testing.T) {
	notFound := &net.DNSError{Err: "no such host", Name: "missing.example.com", IsNotFound: true}
	resolver := &fakeResolver{
		records: map[string][]string{
			"ok.example.com":        {"192.0.2.10"},
			"elsewhere.example.com": {"198.51.100.5"},
			"v6.example.com":        {"192.0.2.10", "2001:db8::1"},
			"loopback.example.com":  {"127.0.0.1"},
		},
		errs: map[string]error{
			"missing.example.com": notFound,
			"broken.example.com":  errors.New("i/o timeout"),
		},
	}

	tests := []struct {
		name      string
		domain    string
		ipv6      bool
		wantCodes []string
		wantLevel []string
		check     func(t *testing.T, diags []Diagnostic)
	}{
		{
			name:      "resolves locally",
			domain:    "ok.example.com",
			wantCodes: []string{CodeDNSOK},
			wantLevel: []string{LevelInfo},
		},
		{
			name:      "points elsewhere",
			domain:    "elsewhere.example.com",
			wantCodes: []string{CodeDNSPointsElsewhere},
			wantLevel: []string{LevelWarning},
			check: func(t *testing.T, diags []Diagnostic) {
				assert.Contains(t, diags[0].Message, "NAT")
				assert.Contains(t, diags[0].Message, "CDN")
				assert.Equal(t, "198.51.100.5", diags[0].Params["elsewhere"])
				assert.Equal(t, "192.0.2.10", diags[0].Params["local"])
			},
		},
		{
			name:      "loopback is never this server for a CA",
			domain:    "loopback.example.com",
			wantCodes: []string{CodeDNSPointsElsewhere},
			wantLevel: []string{LevelWarning},
		},
		{
			name:      "no records",
			domain:    "missing.example.com",
			wantCodes: []string{CodeDNSNoRecords},
			wantLevel: []string{LevelWarning},
		},
		{
			name:      "lookup failure",
			domain:    "broken.example.com",
			wantCodes: []string{CodeDNSLookupFailed},
			wantLevel: []string{LevelWarning},
		},
		{
			name:      "AAAA without IPv6 listen",
			domain:    "v6.example.com",
			wantCodes: []string{CodeDNSPointsElsewhere, CodeAAAAWithoutIPv6Listen},
			wantLevel: []string{LevelWarning, LevelWarning},
			check: func(t *testing.T, diags []Diagnostic) {
				assert.Equal(t, "2001:db8::1", diags[1].Params["ip"])
			},
		},
		{
			name:      "AAAA with IPv6 listen",
			domain:    "v6.example.com",
			ipv6:      true,
			wantCodes: []string{CodeDNSPointsElsewhere},
			wantLevel: []string{LevelWarning},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := Diagnose(context.Background(), []string{tt.domain}, Options{
				Resolver:      resolver,
				LocalAddrs:    localAddrs("127.0.0.1", "192.0.2.10"),
				HasIPv6Listen: tt.ipv6,
			})
			require.Equal(t, tt.wantCodes, codes(diags))
			for i, d := range diags {
				assert.Equal(t, tt.wantLevel[i], d.Level)
				assert.NotEmpty(t, d.Message)
				assert.Equal(t, tt.domain, d.Params["domain"])
			}
			if tt.check != nil {
				tt.check(t, diags)
			}
		})
	}
}

func TestDiagnoseSkipsIPsAndWildcardAddresses(t *testing.T) {
	resolver := &fakeResolver{records: map[string][]string{"example.com": {"192.0.2.10"}}}
	diags := Diagnose(context.Background(), []string{"192.0.2.10", "*.example.com", "Example.COM.", "example.com"}, Options{
		Resolver:   resolver,
		LocalAddrs: localAddrs("192.0.2.10"),
	})
	assert.Equal(t, []string{CodeDNSOK}, codes(diags))
	assert.Equal(t, []string{"example.com"}, resolver.queried)
}

func TestDiagnoseWithoutLocalAddresses(t *testing.T) {
	resolver := &fakeResolver{records: map[string][]string{"example.com": {"192.0.2.10"}}}
	diags := Diagnose(context.Background(), []string{"example.com"}, Options{
		Resolver:   resolver,
		LocalAddrs: func() ([]net.IP, error) { return nil, errors.New("no interfaces") },
	})
	assert.Equal(t, []string{CodeDNSResolved}, codes(diags))
	assert.Equal(t, LevelInfo, diags[0].Level)
}

func TestDiagnoseCAA(t *testing.T) {
	resolver := &fakeResolver{records: map[string][]string{
		"www.blocked.test": {"192.0.2.10"},
		"www.allowed.test": {"192.0.2.10"},
	}}
	caa := fakeCAA{
		"blocked.test":    {{Tag: "issue", Value: "pki.goog"}},
		"allowed.test":    {{Tag: "issue", Value: "letsencrypt.org; validationmethods=http-01"}, {Tag: "iodef", Value: "mailto:a@b"}},
		"wild.test":       {{Tag: "issue", Value: "letsencrypt.org"}, {Tag: "issuewild", Value: ";"}},
		"iodefonly.test":  {{Tag: "iodef", Value: "mailto:a@b"}},
		"sub.nested.test": nil,
		"nested.test":     {{Tag: "issue", Value: "sectigo.com"}},
	}

	opts := Options{
		Resolver:      resolver,
		LocalAddrs:    localAddrs("192.0.2.10"),
		CAAIdentities: CAAIdentitiesForDirectory("https://acme-v02.api.letsencrypt.org/directory"),
		CAALookuper:   caa,
	}

	diags := Diagnose(context.Background(), []string{"www.blocked.test"}, opts)
	require.Equal(t, []string{CodeDNSOK, CodeCAABlocksCA}, codes(diags))
	assert.Equal(t, "blocked.test", diags[1].Params["caa_name"])
	assert.Equal(t, "letsencrypt.org", diags[1].Params["ca"])

	diags = Diagnose(context.Background(), []string{"www.allowed.test"}, opts)
	assert.Equal(t, []string{CodeDNSOK}, codes(diags))

	diags = Diagnose(context.Background(), []string{"*.wild.test"}, opts)
	assert.Equal(t, []string{CodeCAABlocksCA}, codes(diags))

	diags = Diagnose(context.Background(), []string{"*.iodefonly.test"}, opts)
	assert.Empty(t, diags)

	diags = Diagnose(context.Background(), []string{"*.sub.nested.test"}, opts)
	require.Equal(t, []string{CodeCAABlocksCA}, codes(diags))
	assert.Equal(t, "nested.test", diags[0].Params["caa_name"])
}

func TestCAAIdentitiesForDirectory(t *testing.T) {
	assert.Equal(t, []string{"letsencrypt.org"}, CAAIdentitiesForDirectory(""))
	assert.Equal(t, []string{"letsencrypt.org"}, CAAIdentitiesForDirectory("https://acme-staging-v02.api.letsencrypt.org/directory"))
	assert.Equal(t, []string{"sectigo.com"}, CAAIdentitiesForDirectory("https://acme.zerossl.com/v2/DV90"))
	assert.Equal(t, []string{"pki.goog"}, CAAIdentitiesForDirectory("https://dv.acme-v02.api.pki.goog/directory"))
	assert.Nil(t, CAAIdentitiesForDirectory("https://pebble:14000/dir"))
}

func TestEvidenceFromDiagnosticsFeedsClassify(t *testing.T) {
	resolver := &fakeResolver{records: map[string][]string{"bad.e2e.test": {"10.30.0.20"}}}
	diags := Diagnose(context.Background(), []string{"bad.e2e.test"}, Options{
		Resolver:   resolver,
		LocalAddrs: localAddrs("10.30.0.10"),
	})
	ev := EvidenceFromDiagnostics(diags)
	assert.Equal(t, []string{"10.30.0.20"}, ev.Resolved["bad.e2e.test"])
	assert.Equal(t, []string{"10.30.0.10"}, ev.Local)

	hint := Classify(errors.New(pebbleWrongServer), ev)
	require.NotNil(t, hint)
	assert.Equal(t, CodeDNSPointsElsewhere, hint.Code)
}

func TestWithDNSPort(t *testing.T) {
	assert.Equal(t, []string{"1.1.1.1:53", "8.8.8.8:5353", "[2606:4700::1111]:53"},
		withDNSPort([]string{"1.1.1.1", " 8.8.8.8:5353 ", "", "2606:4700::1111"}))
}
