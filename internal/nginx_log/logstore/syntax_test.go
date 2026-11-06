package logstore

import (
	"context"
	"math"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearchText(t *testing.T) {
	cases := []struct {
		in       string
		want     []Clause
		warnings []string
	}{
		{"status:404", []Clause{{Kind: ClauseRange, Column: ColStatus, Lo: 404, Hi: 404}}, nil},
		{"status:5xx", []Clause{{Kind: ClauseRange, Column: ColStatus, Lo: 500, Hi: 599}}, nil},
		{"status:400-499", []Clause{{Kind: ClauseRange, Column: ColStatus, Lo: 400, Hi: 499}}, nil},
		{"status:>=500", []Clause{{Kind: ClauseRange, Column: ColStatus, Lo: 500, Hi: math.MaxUint64}}, nil},
		{"-status:404", []Clause{{Negate: true, Kind: ClauseRange, Column: ColStatus, Lo: 404, Hi: 404}}, nil},
		{"rt:0.1..0.5", []Clause{{Kind: ClauseRange, Column: ColRequestTime, Lo: 100, Hi: 500}}, nil},
		{"bytes:>1000", []Clause{{Kind: ClauseRange, Column: ColBytesSent, Lo: 1001, Hi: math.MaxUint64}}, nil},
		{"rt:<0.5", []Clause{{Kind: ClauseRange, Column: ColRequestTime, Lo: 0, Hi: 499}}, nil},
		{"ip:10.0.0.0/8", []Clause{{Kind: ClauseIP, Column: ColIP, Prefix: netip.MustParsePrefix("10.0.0.0/8")}}, nil},
		{"ip:2001:db8::/32", []Clause{{Kind: ClauseIP, Column: ColIP, Prefix: netip.MustParsePrefix("2001:db8::/32")}}, nil},
		{"method:post", []Clause{{Kind: ClauseEquals, Column: ColMethod, Text: "post"}}, nil},
		{`path:"/a b"`, []Clause{{Kind: ClauseContains, Column: ColPath, Text: "/a b"}}, nil},
		{`UA:Curl`, []Clause{{Kind: ClauseContains, Column: ColUserAgent, Text: "curl"}}, nil},
		{`"GET /x" -bot`, []Clause{{Kind: ClauseText, Text: "GET /x"}, {Negate: true, Kind: ClauseText, Text: "bot"}}, nil},
		{`-"slow request"`, []Clause{{Negate: true, Kind: ClauseText, Text: "slow request"}}, nil},
		{"https://example.com 12:30:00", []Clause{{Kind: ClauseText, Text: "https://example.com"}, {Kind: ClauseText, Text: "12:30:00"}}, nil},
		{"colour:red", []Clause{{Kind: ClauseText, Text: "colour:red"}}, []string{"unknown_field"}},
		{"status:abc", []Clause{{Kind: ClauseText, Text: "status:abc"}}, []string{"invalid_value"}},
		{`"open`, []Clause{{Kind: ClauseText, Text: "open"}}, []string{"unterminated_quote"}},
		{" - ", nil, nil},
	}
	for _, c := range cases {
		got, warnings := ParseSearchText(c.in)
		assert.Equal(t, c.want, got, c.in)
		var reasons []string
		for _, w := range warnings {
			reasons = append(reasons, w.Reason)
		}
		assert.Equal(t, c.warnings, reasons, c.in)
	}
}

// Every filter of the syntax selects exactly what a brute-force evaluation
// over the rows selects.
func TestSearchTextFiltersMatchReference(t *testing.T) {
	rows := genRows(6000, 21)
	s := openTestStore(t, t.TempDir(), 700)
	ingest(t, s, rows, 0)

	inPrefix := func(ip, prefix string) bool {
		a, err := netip.ParseAddr(ip)
		return err == nil && netip.MustParsePrefix(prefix).Contains(a)
	}
	cases := map[string]func(r *testRow) bool{
		"status:5xx":         func(r *testRow) bool { return r.Status >= 500 && r.Status <= 599 },
		"-status:200":        func(r *testRow) bool { return r.Status != 200 },
		"status:300-499":     func(r *testRow) bool { return r.Status >= 300 && r.Status <= 499 },
		"method:get":         func(r *testRow) bool { return r.Method == "GET" },
		"ip:10.0.2.0/24":     func(r *testRow) bool { return inPrefix(r.IP, "10.0.2.0/24") },
		"-ip:10.0.0.0/23":    func(r *testRow) bool { return !inPrefix(r.IP, "10.0.0.0/23") },
		"path:/API/v1":       func(r *testRow) bool { return strings.Contains(strings.ToLower(r.Path), "/api/v1") },
		"ua:iphone -bot":     func(r *testRow) bool { return strings.Contains(strings.ToLower(r.UserAgent), "iphone") },
		"browser:chrome":     func(r *testRow) bool { return strings.Contains(strings.ToLower(r.Browser), "chrome") },
		"country:cn":         func(r *testRow) bool { return r.RegionCode == "CN" },
		"bytes:>50000":       func(r *testRow) bool { return r.BytesSent > 50000 },
		"rt:0.5..1.5":        func(r *testRow) bool { ms := secondsToMs(r.RequestTime); return ms >= 500 && ms <= 1500 },
		"get -google":        func(r *testRow) bool { return lc(r.Raw, "get") && !lc(r.Raw, "google") },
		`"HTTP/1.1" 404`:     func(r *testRow) bool { return lc(r.Raw, "http/1.1") && lc(r.Raw, "404") },
		"status:404 path:/s": func(r *testRow) bool { return r.Status == 404 && lc(r.Path, "/s") },
		"referer:example":    func(r *testRow) bool { return lc(r.Referer, "example") },
		"-device:bot rt:<1":  func(r *testRow) bool { return r.DeviceType != "bot" && secondsToMs(r.RequestTime) < 1000 },
	}
	for text, keep := range cases {
		clauses, warnings := ParseSearchText(text)
		require.Empty(t, warnings, text)
		want := 0
		for _, r := range rows {
			if keep(r) {
				want++
			}
		}
		n, err := s.Count(context.Background(), &Query{Clauses: clauses})
		require.NoError(t, err, text)
		assert.EqualValues(t, want, n, text)
	}
}

func lc(s, sub string) bool { return strings.Contains(strings.ToLower(s), strings.ToLower(sub)) }
