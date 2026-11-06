package logstore

import (
	"math"
	"net/netip"
	"strconv"
	"strings"
)

// The search box syntax, after the log analytics plugin on Tantivy.
//
// Words and field:value filters are separated by spaces and combine with AND.
// A leading "-" excludes, double quotes keep a phrase together. Whatever does
// not parse stays ordinary text, so the parser never fails; tokens that looked
// like a filter but were not one are reported as warnings.
//
//	status:404  status:5xx  status:400-499  status:>=500
//	method:POST
//	ip:1.2.3.4  ip:10.0.0.0/8  ip:2001:db8::/32
//	path:/api/  ua:curl  referer:google        (case-insensitive substring)
//	browser:  os:  device:  region:  city:     (case-insensitive substring)
//	country:CN                                 (country code)
//	bytes:>1000  rt:>0.5  rt:0.1..0.5          (bytes sent, request time in s)
//	level:error                                (error logs)
//
// Free words match the raw line as case-insensitive substrings.

// ClauseKind is what a clause tests.
type ClauseKind uint8

const (
	ClauseText     ClauseKind = iota // substring of the raw line
	ClauseContains                   // substring of a dictionary column
	ClauseEquals                     // whole value of a dictionary column, case-insensitive
	ClauseRange                      // numeric column within [Lo, Hi]
	ClauseIP                         // ip within the prefix
)

// Clause is one condition of the search box. Clauses combine with AND.
type Clause struct {
	Negate bool
	Kind   ClauseKind
	Column Column
	Text   string       // ClauseText, ClauseContains, ClauseEquals
	Lo, Hi uint64       // ClauseRange, inclusive; request_time in ms
	Prefix netip.Prefix // ClauseIP
}

// SyntaxWarning reports a token that looked like a filter but was taken as
// text.
type SyntaxWarning struct {
	Token  string `json:"token"`
	Reason string `json:"reason"` // unknown_field, invalid_value, unterminated_quote
}

var syntaxFields = map[string]Column{
	"status": ColStatus, "method": ColMethod, "ip": ColIP,
	"path": ColPath, "ua": ColUserAgent, "user_agent": ColUserAgent,
	"referer": ColReferer, "referrer": ColReferer,
	"browser": ColBrowser, "os": ColOS, "device": ColDeviceType,
	"country": ColRegionCode, "region": ColProvince, "city": ColCity,
	"bytes": ColBytesSent, "rt": ColRequestTime, "request_time": ColRequestTime, "time": ColRequestTime,
}

// levelColumn is set by the error log support; zero means level: is unknown.
var levelColumn Column

// ParseSearchText turns the search box into clauses.
func ParseSearchText(text string) ([]Clause, []SyntaxWarning) {
	var clauses []Clause
	var warnings []SyntaxWarning
	for _, tok := range tokenize(text, &warnings) {
		if tok.quoted {
			clauses = append(clauses, Clause{Negate: tok.negate, Kind: ClauseText, Text: tok.text})
			continue
		}
		name, value, isField := strings.Cut(tok.text, ":")
		if !isField || !isFieldName(name) || strings.HasPrefix(value, "//") {
			// Not a filter: a URL, a time, an IPv6 address or a plain word.
			clauses = append(clauses, Clause{Negate: tok.negate, Kind: ClauseText, Text: tok.text})
			continue
		}
		clause, ok, reason := parseFilter(strings.ToLower(name), value)
		if !ok {
			warnings = append(warnings, SyntaxWarning{Token: tok.raw, Reason: reason})
			clauses = append(clauses, Clause{Negate: tok.negate, Kind: ClauseText, Text: tok.text})
			continue
		}
		clause.Negate = tok.negate
		clauses = append(clauses, clause)
	}
	return clauses, warnings
}

// isFieldName reports whether name can be a filter name: letters and
// underscores, starting with a letter. A URL scheme followed by // and a
// time such as 12:30:00 are then plain text.
func isFieldName(name string) bool {
	if name == "" || !isLetter(name[0]) {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isLetter(name[i]) && name[i] != '_' {
			return false
		}
	}
	return true
}

func isLetter(b byte) bool { return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' }

type token struct {
	raw    string // as typed, for warnings
	text   string // without the minus and the quotes
	negate bool
	quoted bool // a whole-token phrase
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

// tokenize splits on whitespace. A token that starts with a double quote is a
// phrase; a quote later in a token lets a filter value hold spaces, as in
// path:"/a b". A leading minus negates either.
func tokenize(s string, warnings *[]SyntaxWarning) []token {
	var out []token
	i := 0
	for {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			return out
		}
		start := i
		tok := token{}
		if s[i] == '-' && i+1 < len(s) && !isSpace(s[i+1]) {
			tok.negate = true
			i++
		}
		var b strings.Builder
		if s[i] == '"' {
			end := strings.IndexByte(s[i+1:], '"')
			if end < 0 {
				*warnings = append(*warnings, SyntaxWarning{Token: s[start:], Reason: "unterminated_quote"})
				b.WriteString(s[i+1:])
				i = len(s)
			} else {
				b.WriteString(s[i+1 : i+1+end])
				i += end + 2
				tok.quoted = true
			}
		}
		for !tok.quoted && i < len(s) && !isSpace(s[i]) {
			if s[i] == '"' {
				end := strings.IndexByte(s[i+1:], '"')
				if end < 0 {
					*warnings = append(*warnings, SyntaxWarning{Token: s[start:], Reason: "unterminated_quote"})
					b.WriteString(s[i+1:])
					i = len(s)
					break
				}
				b.WriteString(s[i+1 : i+1+end])
				i += end + 2
				continue
			}
			b.WriteByte(s[i])
			i++
		}
		tok.raw, tok.text = s[start:i], b.String()
		if tok.text != "" && (tok.quoted || tok.text != "-") {
			out = append(out, tok)
		}
	}
}

func parseFilter(name, value string) (Clause, bool, string) {
	if name == "level" {
		if levelColumn == 0 || value == "" {
			return Clause{}, false, "unknown_field"
		}
		return Clause{Kind: ClauseEquals, Column: levelColumn, Text: strings.ToLower(value)}, true, ""
	}
	col, known := syntaxFields[name]
	if !known {
		return Clause{}, false, "unknown_field"
	}
	if value == "" {
		return Clause{}, false, "invalid_value"
	}
	switch col {
	case ColStatus:
		lo, hi, ok := statusSpan(value)
		return Clause{Kind: ClauseRange, Column: col, Lo: lo, Hi: hi}, ok, "invalid_value"
	case ColBytesSent:
		lo, hi, ok := numberSpan(value, 1)
		return Clause{Kind: ClauseRange, Column: col, Lo: lo, Hi: hi}, ok, "invalid_value"
	case ColRequestTime:
		lo, hi, ok := numberSpan(value, 1000)
		return Clause{Kind: ClauseRange, Column: col, Lo: lo, Hi: hi}, ok, "invalid_value"
	case ColIP:
		prefix, ok := ipPrefix(value)
		return Clause{Kind: ClauseIP, Column: col, Prefix: prefix}, ok, "invalid_value"
	case ColMethod, ColRegionCode:
		return Clause{Kind: ClauseEquals, Column: col, Text: strings.ToLower(value)}, true, ""
	default:
		return Clause{Kind: ClauseContains, Column: col, Text: lowerASCII(value)}, true, ""
	}
}

// statusSpan reads 404, 5xx, 400-499 and the forms of numberSpan.
func statusSpan(v string) (uint64, uint64, bool) {
	lv := strings.ToLower(v)
	if len(lv) == 3 && lv[1:] == "xx" && lv[0] >= '1' && lv[0] <= '5' {
		base := uint64(lv[0]-'0') * 100
		return base, base + 99, true
	}
	if a, b, ok := strings.Cut(v, "-"); ok && a != "" && b != "" {
		lo, err1 := strconv.ParseUint(a, 10, 16)
		hi, err2 := strconv.ParseUint(b, 10, 16)
		return lo, hi, err1 == nil && err2 == nil && lo <= hi
	}
	return numberSpan(v, 1)
}

// numberSpan reads n, >n, >=n, <n, <=n and a..b, either side of a range open.
// scale converts the unit of the value into the stored one (s to ms).
func numberSpan(v string, scale float64) (uint64, uint64, bool) {
	parse := func(s string) (uint64, bool) {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return uint64(math.Round(f * scale)), true
	}
	if a, b, ok := strings.Cut(v, ".."); ok {
		lo, hi := uint64(0), uint64(math.MaxUint64)
		if a != "" {
			if lo, ok = parse(a); !ok {
				return 0, 0, false
			}
		}
		if b != "" {
			if hi, ok = parse(b); !ok {
				return 0, 0, false
			}
		}
		return lo, hi, (a != "" || b != "") && lo <= hi
	}
	for _, op := range []string{">=", "<=", ">", "<"} {
		if rest, ok := strings.CutPrefix(v, op); ok {
			n, ok := parse(rest)
			if !ok {
				return 0, 0, false
			}
			switch op {
			case ">=":
				return n, math.MaxUint64, true
			case "<=":
				return 0, n, true
			case ">":
				return n + 1, math.MaxUint64, true
			default:
				if n == 0 {
					return 0, 0, false
				}
				return 0, n - 1, true
			}
		}
	}
	n, ok := parse(v)
	return n, n, ok
}

// ipPrefix reads an address or a network. IPv4 addresses are compared as
// IPv4, so ip:10.0.0.0/8 matches 10.1.2.3 and not its IPv6-mapped form.
func ipPrefix(v string) (netip.Prefix, bool) {
	if p, err := netip.ParsePrefix(v); err == nil {
		return p.Masked(), true
	}
	if a, err := netip.ParseAddr(v); err == nil {
		return netip.PrefixFrom(a, a.BitLen()), true
	}
	return netip.Prefix{}, false
}
