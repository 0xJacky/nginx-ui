// Package managed implements standalone upstream groups that Nginx UI owns.
//
// Every managed upstream lives in its own file, conf.d/upstream-<name>.conf,
// which the http context already includes (the self check verifies the
// `include conf.d/*.conf` line). Sites reference a group by name with
// `proxy_pass http://<name>;`, so changing the servers or the balancing method
// is done once, in that file, instead of in every site.
package managed

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy"
)

// Load balancing methods supported by the open source nginx upstream module.
const (
	MethodRoundRobin = ""
	MethodLeastConn  = "least_conn"
	MethodIPHash     = "ip_hash"
	MethodHash       = "hash"
	MethodRandom     = "random"
)

// Server is one `server` entry of an upstream group.
type Server struct {
	Address     string `json:"address"`
	Weight      *int   `json:"weight,omitempty"`
	MaxFails    *int   `json:"max_fails,omitempty"`
	FailTimeout string `json:"fail_timeout,omitempty"`
	Backup      bool   `json:"backup"`
	Down        bool   `json:"down"`
	// Params keeps server parameters this form does not model (max_conns,
	// slow_start, resolve, ...) so a round trip does not drop them.
	Params string `json:"params,omitempty"`
	// Socket is the key the availability checker reports this server under,
	// for example "web.internal:80" for "web.internal". It is derived from the
	// server line when a Detail is built, empty when the server is never
	// probed, and ignored on input.
	Socket string `json:"socket"`
}

// Upstream is the structured form of a managed upstream group.
type Upstream struct {
	Name       string `json:"name"`
	Method     string `json:"method"`
	HashKey    string `json:"hash_key,omitempty"`
	Consistent bool   `json:"consistent"`
	Keepalive  int    `json:"keepalive"`
	// Zone places the group in a shared memory zone named after the group
	// (`zone <name> <size>;`), so every worker process balances with the same
	// state and failure counters.
	Zone     bool     `json:"zone"`
	ZoneSize string   `json:"zone_size,omitempty"`
	Servers  []Server `json:"servers"`
	// ExtraDirectives holds any other upstream-level directives, one per line,
	// for example `zone backend 64k;` or `keepalive_timeout 60s;`.
	ExtraDirectives string `json:"extra_directives,omitempty"`
}

var (
	reName        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)
	reUnsafeToken = regexp.MustCompile(`[\s;{}#"'\\]`)
	reTime        = regexp.MustCompile(`^\d+(ms|s|m|h|d|w|M|y)?$`)
	reServerParam = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(=[^\s;{}#"'\\]+)?$`)
	reZoneSize    = regexp.MustCompile(`^(\d+)([kKmM]?)$`)
	reZoneLine    = regexp.MustCompile(`^zone\s+(\S+)\s+([^\s;]+)\s*;?$`)
	reZoneAny     = regexp.MustCompile(`(?m)^\s*zone(\s|;|$)`)
)

// DefaultZoneSize is the shared memory size given to new upstream groups. It
// holds the state of a few hundred servers.
const DefaultZoneSize = "64k"

// minZoneSize is the smallest zone nginx accepts on 4 KiB page systems
// (eight pages); smaller zones fail `nginx -t` with "zone is too small".
const minZoneSize = 32 * 1024

// zoneSizeBytes converts an nginx size such as 64k or 1m into bytes.
func zoneSizeBytes(size string) (int64, bool) {
	m := reZoneSize.FindStringSubmatch(size)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, false
	}
	switch strings.ToLower(m[2]) {
	case "k":
		n *= 1024
	case "m":
		n *= 1024 * 1024
	}
	return n, true
}

// ValidateName reports whether name can be used as a managed upstream name.
func ValidateName(name string) error {
	if !reName.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}

func isSafeToken(value string) bool {
	return value != "" && !reUnsafeToken.MatchString(value)
}

// Normalize trims the free-form fields so validation and rendering see the
// same values the user meant.
func (u *Upstream) Normalize() {
	u.Name = strings.TrimSpace(u.Name)
	u.Method = strings.TrimSpace(u.Method)
	if u.Method == "round_robin" {
		u.Method = MethodRoundRobin
	}
	u.HashKey = strings.TrimSpace(u.HashKey)
	if u.Method != MethodHash {
		u.HashKey = ""
		u.Consistent = false
	}
	u.ExtraDirectives = strings.TrimSpace(u.ExtraDirectives)
	u.ZoneSize = strings.TrimSpace(u.ZoneSize)
	u.liftZoneFromExtra()
	if u.Zone && u.ZoneSize == "" {
		u.ZoneSize = DefaultZoneSize
	}
	if !u.Zone {
		u.ZoneSize = ""
	}
	for i := range u.Servers {
		s := &u.Servers[i]
		s.Address = strings.TrimSpace(s.Address)
		s.FailTimeout = strings.TrimSpace(s.FailTimeout)
		s.Params = strings.Join(strings.Fields(s.Params), " ")
		// Derived on output only; a value echoed back by a client means nothing.
		s.Socket = ""
	}
}

// liftZoneFromExtra moves a `zone <name> <size>;` line for this group out of
// the additional directives into the zone fields, so it is neither emitted
// twice nor hidden from the form. When the zone switch is already on, the
// form value wins and the line is dropped.
func (u *Upstream) liftZoneFromExtra() {
	if u.ExtraDirectives == "" {
		return
	}
	lines := strings.Split(u.ExtraDirectives, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		m := reZoneLine.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil && m[1] == u.Name {
			if !u.Zone {
				u.Zone = true
				u.ZoneSize = m[2]
			}
			continue
		}
		kept = append(kept, line)
	}
	u.ExtraDirectives = strings.TrimSpace(strings.Join(kept, "\n"))
}

// Validate checks every field that ends up in the generated configuration.
// Values are rejected rather than escaped: anything that could break out of a
// directive (whitespace, `;`, braces, quotes, comments) is refused.
func (u *Upstream) Validate() error {
	if err := ValidateName(u.Name); err != nil {
		return err
	}

	switch u.Method {
	case MethodRoundRobin, MethodLeastConn, MethodIPHash, MethodRandom:
	case MethodHash:
		if u.HashKey == "" {
			return ErrHashKeyRequired
		}
		if !isSafeToken(u.HashKey) {
			return cosy.WrapErrorWithParams(ErrInvalidHashKey, u.HashKey)
		}
	default:
		return cosy.WrapErrorWithParams(ErrInvalidMethod, u.Method)
	}

	if len(u.Servers) == 0 {
		return ErrNoServers
	}

	for _, s := range u.Servers {
		if !isSafeToken(s.Address) || strings.Contains(s.Address, "://") {
			return cosy.WrapErrorWithParams(ErrInvalidServerAddress, s.Address)
		}
		if s.Weight != nil && *s.Weight < 1 {
			return cosy.WrapErrorWithParams(ErrInvalidWeight, s.Address)
		}
		if s.MaxFails != nil && *s.MaxFails < 0 {
			return cosy.WrapErrorWithParams(ErrInvalidMaxFails, s.Address)
		}
		if s.FailTimeout != "" && !reTime.MatchString(s.FailTimeout) {
			return cosy.WrapErrorWithParams(ErrInvalidFailTimeout, s.FailTimeout)
		}
		if s.Backup {
			switch u.Method {
			case MethodHash, MethodIPHash, MethodRandom:
				return cosy.WrapErrorWithParams(ErrBackupNotSupported, u.Method)
			}
		}
		for _, param := range strings.Fields(s.Params) {
			if !reServerParam.MatchString(param) {
				return cosy.WrapErrorWithParams(ErrInvalidServerParams, s.Params)
			}
		}
	}

	if u.Keepalive < 0 {
		return ErrInvalidKeepalive
	}

	if strings.ContainsAny(u.ExtraDirectives, "{}") {
		return ErrInvalidExtraDirectives
	}

	if u.Zone {
		bytes, ok := zoneSizeBytes(u.ZoneSize)
		if !ok || bytes < minZoneSize {
			return cosy.WrapErrorWithParams(ErrInvalidZoneSize, u.ZoneSize)
		}
		// nginx rejects a second zone directive in the same upstream block.
		if reZoneAny.MatchString(u.ExtraDirectives) {
			return ErrZoneInExtraDirectives
		}
	}

	return nil
}

// fileHeader marks a file as generated so an operator who opens it in the
// config editor knows where it comes from.
const fileHeader = "# Managed by Nginx UI: edit this upstream from the Upstream page.\n" +
	"# Sites reference it with `proxy_pass http://%s;`.\n"

// renderServer renders the parameters of a single server entry.
func renderServer(s Server) string {
	parts := []string{s.Address}
	if s.Weight != nil && *s.Weight != 1 {
		parts = append(parts, "weight="+strconv.Itoa(*s.Weight))
	}
	if s.MaxFails != nil {
		parts = append(parts, "max_fails="+strconv.Itoa(*s.MaxFails))
	}
	if s.FailTimeout != "" {
		parts = append(parts, "fail_timeout="+s.FailTimeout)
	}
	if s.Params != "" {
		parts = append(parts, s.Params)
	}
	if s.Backup {
		parts = append(parts, "backup")
	}
	if s.Down {
		parts = append(parts, "down")
	}
	return strings.Join(parts, " ")
}

// Render builds the configuration file content for the upstream. Callers are
// expected to Normalize and Validate first.
func (u *Upstream) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, fileHeader, u.Name)
	fmt.Fprintf(&b, "upstream %s {\n", u.Name)

	switch u.Method {
	case MethodLeastConn, MethodIPHash, MethodRandom:
		fmt.Fprintf(&b, "    %s;\n", u.Method)
	case MethodHash:
		if u.Consistent {
			fmt.Fprintf(&b, "    hash %s consistent;\n", u.HashKey)
		} else {
			fmt.Fprintf(&b, "    hash %s;\n", u.HashKey)
		}
	}

	if u.Zone {
		fmt.Fprintf(&b, "    zone %s %s;\n", u.Name, u.ZoneSize)
	}

	for _, s := range u.Servers {
		fmt.Fprintf(&b, "    server %s;\n", renderServer(s))
	}

	if u.Keepalive > 0 {
		fmt.Fprintf(&b, "    keepalive %d;\n", u.Keepalive)
	}

	for _, line := range strings.Split(u.ExtraDirectives, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasSuffix(line, ";") && !strings.HasPrefix(line, "#") {
			line += ";"
		}
		fmt.Fprintf(&b, "    %s\n", line)
	}

	b.WriteString("}\n")
	return b.String()
}

// parseServer maps the parameters of a `server` directive onto Server.
func parseServer(params string) Server {
	fields := strings.Fields(params)
	s := Server{}
	if len(fields) == 0 {
		return s
	}
	s.Address = fields[0]

	var extra []string
	for _, field := range fields[1:] {
		key, value, hasValue := strings.Cut(field, "=")
		switch {
		case key == "weight" && hasValue:
			if n, err := strconv.Atoi(value); err == nil {
				s.Weight = &n
				continue
			}
		case key == "max_fails" && hasValue:
			if n, err := strconv.Atoi(value); err == nil {
				s.MaxFails = &n
				continue
			}
		case key == "fail_timeout" && hasValue:
			s.FailTimeout = value
			continue
		case field == "backup":
			s.Backup = true
			continue
		case field == "down":
			s.Down = true
			continue
		}
		extra = append(extra, field)
	}
	s.Params = strings.Join(extra, " ")
	return s
}

// Parse reads a managed upstream file back into its structured form. It only
// accepts a file holding exactly one upstream block named name and nothing
// else, which is what Render produces; anything else is reported as not
// managed so it is never silently rewritten.
func Parse(name, content string) (*Upstream, error) {
	cfg, err := nginx.ParseNgxConfigByContent(content)
	if err != nil {
		return nil, err
	}
	if len(cfg.Upstreams) != 1 || len(cfg.Servers) != 0 ||
		strings.TrimSpace(cfg.Custom) != "" || cfg.Upstreams[0].Name != name {
		return nil, cosy.WrapErrorWithParams(ErrUpstreamFileNotManaged, FileName(name))
	}

	u := &Upstream{Name: name, Servers: make([]Server, 0)}
	var extra []string
	for _, d := range cfg.Upstreams[0].Directives {
		params := strings.TrimSpace(d.Params)
		switch d.Directive {
		case MethodLeastConn, MethodIPHash:
			if params == "" && u.Method == MethodRoundRobin {
				u.Method = d.Directive
				continue
			}
		case MethodRandom:
			// `random two least_conn;` has no form field; keep it verbatim.
			if params == "" && u.Method == MethodRoundRobin {
				u.Method = MethodRandom
				continue
			}
		case MethodHash:
			fields := strings.Fields(params)
			if u.Method == MethodRoundRobin && (len(fields) == 1 ||
				(len(fields) == 2 && fields[1] == "consistent")) {
				u.Method = MethodHash
				u.HashKey = fields[0]
				u.Consistent = len(fields) == 2
				continue
			}
		case "keepalive":
			if n, err := strconv.Atoi(params); err == nil && u.Keepalive == 0 {
				u.Keepalive = n
				continue
			}
		case "server":
			u.Servers = append(u.Servers, parseServer(params))
			continue
		case "zone":
			// Only a zone named after the group with its own size maps onto
			// the form; `zone other;` joins a zone declared elsewhere and is
			// kept verbatim.
			fields := strings.Fields(params)
			if len(fields) == 2 && fields[0] == name && !u.Zone {
				u.Zone = true
				u.ZoneSize = fields[1]
				continue
			}
		}
		line := d.Directive
		if params != "" {
			line += " " + params
		}
		extra = append(extra, line+";")
	}
	u.ExtraDirectives = strings.Join(extra, "\n")
	return u, nil
}
