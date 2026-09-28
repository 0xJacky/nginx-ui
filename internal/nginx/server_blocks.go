package nginx

import (
	"context"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// ServerListen is one listen directive of an http server block.
type ServerListen struct {
	Addr          string // "" when the listen binds every address ("80", "*:80", "[::]:80" -> Addr "" with IPv6 true)
	Port          string // "80"; "80" when only an address is given
	IPv6          bool   // listen [..]:port
	SSL           bool
	DefaultServer bool // default_server or legacy "default"
}

// ServerLocation is a top-level location of an http server block.
type ServerLocation struct {
	Modifier  string // "", "=", "~", "~*", "^~"
	Path      string
	ProxyPass string // first proxy_pass param inside the location, "" if none
	Return    string // first return params inside the location, "" if none
}

// ServerBlock is an http server block read from the effective configuration.
type ServerBlock struct {
	File        string           // file the block was read from (from the nginx -T "# configuration file" header)
	ServerNames []string         // raw server_name params ("" / "_" kept as written)
	Listens     []ServerListen   // a block without listen gets {Addr:"", Port:"80"} like nginx
	Return      string           // server-level return params, "" if none
	Locations   []ServerLocation // top-level locations only
}

// ServerSocket is a listening socket and the block Nginx picks on it for a host.
type ServerSocket struct {
	Listen ServerListen
	Block  ServerBlock
	// ByName is false when the block was chosen as the socket's default server
	// (no server_name matched the host).
	ByName bool
}

// serverBlocksNginxTTimeout bounds the nginx -T run used by GetServerBlocks.
const serverBlocksNginxTTimeout = 30 * time.Second

// GetServerBlocks parses the http server blocks from `nginx -T`.
//
// It runs nginx -T on the current control target (local, SSH or container,
// through the same runner and binary resolution as getNginxT) on every call
// instead of reading the memoized getNginxT output: that cache is only reset
// when the control target changes, so it would describe the configuration as
// it was at startup rather than the one Nginx currently serves.
//
// Returns ErrNginxTOutputEmpty when nginx -T is unavailable, fails, or prints
// no configuration.
func GetServerBlocks() ([]ServerBlock, error) {
	exePath := getNginxSbinPath()
	if exePath == "" {
		return nil, ErrNginxTOutputEmpty
	}

	ctx, cancel := context.WithTimeout(context.Background(), serverBlocksNginxTTimeout)
	defer cancel()

	out, err := execCommandContext(ctx, exePath, "-T")
	if err != nil {
		logger.Warn("nginx.GetServerBlocks: nginx -T failed", "error", err)
		return nil, ErrNginxTOutputEmpty
	}

	sections := splitNginxTSections(out)
	if len(sections) == 0 {
		return nil, ErrNginxTOutputEmpty
	}

	return parseServerBlocksFromSections(sections), nil
}

// ParseServerBlocks parses nginx -T output.
//
// The output is split into the "# configuration file <path>:" sections nginx
// prints for every file it read. Because included files are printed as
// separate sections, the include tree is rebuilt from the first (main)
// section: include directives are expanded in place (relative patterns are
// resolved against the main file's directory, globs are matched against the
// section paths). Only server blocks that end up directly inside http { } are
// returned; servers under stream { } or mail { } are dropped. Sections that
// are never reached from the main file contribute their top-level server
// blocks only when those contain a server_name or a location, which stream
// servers never have.
func ParseServerBlocks(nginxT string) []ServerBlock {
	return parseServerBlocksFromSections(splitNginxTSections(nginxT))
}

// ResolveServerSockets returns, for every listening socket on `port`, the
// block Nginx would select for `host` following Nginx's rules: exact name,
// longest leading wildcard, longest trailing wildcard, first matching regex
// (in config order), else the socket's default_server, else the first block
// on that socket. Sockets: blocks sharing (Addr, Port, IPv6) form one socket;
// a specific-address socket is separate from the wildcard one, exactly like
// Nginx, which only considers the servers listening on the address a
// connection arrived on.
func ResolveServerSockets(blocks []ServerBlock, host, port string) []ServerSocket {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")

	type socketKey struct {
		addr string
		port string
		ipv6 bool
	}
	type socketGroup struct {
		listen   ServerListen
		blocks   []int
		fallback int // index into blocks of the default server, -1 if none
	}

	var order []socketKey
	groups := map[socketKey]*socketGroup{}

	for bi, block := range blocks {
		for _, l := range block.Listens {
			if l.Port != port {
				continue
			}
			key := socketKey{addr: strings.ToLower(l.Addr), port: l.Port, ipv6: l.IPv6}
			g, ok := groups[key]
			if !ok {
				g = &socketGroup{listen: ServerListen{Addr: l.Addr, Port: l.Port, IPv6: l.IPv6}, fallback: -1}
				groups[key] = g
				order = append(order, key)
			}
			pos := -1
			for i, existing := range g.blocks {
				if existing == bi {
					pos = i
					break
				}
			}
			if pos < 0 {
				g.blocks = append(g.blocks, bi)
				pos = len(g.blocks) - 1
			}
			if l.SSL {
				g.listen.SSL = true
			}
			if l.DefaultServer && g.fallback < 0 {
				g.fallback = pos
				g.listen.DefaultServer = true
			}
		}
	}

	result := make([]ServerSocket, 0, len(order))
	for _, key := range order {
		g := groups[key]
		candidates := make([]ServerBlock, len(g.blocks))
		for i, bi := range g.blocks {
			candidates[i] = blocks[bi]
		}

		idx := matchServerByName(candidates, host)
		byName := idx >= 0
		if !byName {
			idx = g.fallback
			if idx < 0 {
				idx = 0
			}
		}

		result = append(result, ServerSocket{
			Listen: g.listen,
			Block:  candidates[idx],
			ByName: byName,
		})
	}

	return result
}

// matchServerByName returns the index of the block whose server_name wins for
// host, or -1 when no name matches. host must already be lower case.
func matchServerByName(blocks []ServerBlock, host string) int {
	// 1. Exact names. ".example.com" also registers "example.com" as an exact
	// name in Nginx.
	for i, block := range blocks {
		for _, raw := range block.ServerNames {
			name := strings.ToLower(raw)
			if strings.HasPrefix(name, "~") {
				continue
			}
			if name == host || (strings.HasPrefix(name, ".") && name[1:] == host) {
				return i
			}
		}
	}

	// 2. Longest leading wildcard: "*.example.com" or ".example.com".
	best, bestLen := -1, -1
	for i, block := range blocks {
		for _, raw := range block.ServerNames {
			name := strings.ToLower(raw)
			var suffix string
			switch {
			case strings.HasPrefix(name, "*."):
				suffix = name[1:]
			case strings.HasPrefix(name, ".") && len(name) > 1:
				suffix = name
			default:
				continue
			}
			if strings.Contains(suffix, "*") {
				continue
			}
			if len(host) > len(suffix) && strings.HasSuffix(host, suffix) && len(suffix) > bestLen {
				best, bestLen = i, len(suffix)
			}
		}
	}
	if best >= 0 {
		return best
	}

	// 3. Longest trailing wildcard: "www.example.*".
	for i, block := range blocks {
		for _, raw := range block.ServerNames {
			name := strings.ToLower(raw)
			if !strings.HasSuffix(name, ".*") || strings.HasPrefix(name, "~") {
				continue
			}
			prefix := name[:len(name)-1]
			if strings.Contains(prefix, "*") {
				continue
			}
			if len(host) > len(prefix) && strings.HasPrefix(host, prefix) && len(prefix) > bestLen {
				best, bestLen = i, len(prefix)
			}
		}
	}
	if best >= 0 {
		return best
	}

	// 4. First matching regular expression in configuration order. Nginx
	// lower-cases the Host header and compiles caseless whenever the pattern
	// has upper-case letters, so a caseless match is equivalent.
	for i, block := range blocks {
		for _, raw := range block.ServerNames {
			if !strings.HasPrefix(raw, "~") {
				continue
			}
			re, err := regexp.Compile("(?i)" + raw[1:])
			if err != nil {
				continue
			}
			if re.MatchString(host) {
				return i
			}
		}
	}

	return -1
}

// nginxTSection is one "# configuration file <path>:" section of nginx -T.
type nginxTSection struct {
	path    string
	content string
}

var nginxTSectionHeader = regexp.MustCompile(`^# configuration file (.+):$`)

// splitNginxTSections splits nginx -T output into its per-file sections.
// Anything before the first header (the "syntax is ok" preamble or warnings)
// is ignored.
func splitNginxTSections(output string) []nginxTSection {
	var sections []nginxTSection
	var current *nginxTSection
	var body strings.Builder

	flush := func() {
		if current != nil {
			current.content = body.String()
			sections = append(sections, *current)
		}
		body.Reset()
	}

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if m := nginxTSectionHeader.FindStringSubmatch(trimmed); m != nil {
			flush()
			current = &nginxTSection{path: m[1]}
			continue
		}
		if current != nil {
			body.WriteString(trimmed)
			body.WriteByte('\n')
		}
	}
	flush()

	return sections
}

// sbDirective is a parsed configuration directive.
type sbDirective struct {
	name     string
	args     []string
	hasBlock bool
	block    []sbDirective
	file     string
}

// maxIncludeDepth bounds include expansion so a cyclic include cannot loop.
const maxIncludeDepth = 32

// sbIncludeResolver expands include directives against the nginx -T sections.
type sbIncludeResolver struct {
	sections   []nginxTSection
	normalized []string
	parsed     [][]sbDirective
	reached    []bool
	confPrefix string
}

func newSbIncludeResolver(sections []nginxTSection) *sbIncludeResolver {
	r := &sbIncludeResolver{
		sections:   sections,
		normalized: make([]string, len(sections)),
		parsed:     make([][]sbDirective, len(sections)),
		reached:    make([]bool, len(sections)),
	}
	for i, s := range sections {
		r.normalized[i] = normalizeConfPath(s.path)
		r.parsed[i] = parseSbDirectives(s.content, s.path)
	}
	if len(sections) > 0 {
		r.confPrefix = path.Dir(r.normalized[0])
	}
	return r
}

// match returns the indexes of the sections an include pattern refers to, in
// the order nginx printed them (which is the order it read them).
func (r *sbIncludeResolver) match(pattern string) []int {
	p := strings.ReplaceAll(pattern, "\\", "/")
	if !isAbsConfPath(p) {
		p = r.confPrefix + "/" + p
	}
	p = normalizeConfPath(p)

	var matches []int
	isGlob := strings.ContainsAny(p, "*?[")
	for i, candidate := range r.normalized {
		if isGlob {
			if ok, err := path.Match(p, candidate); err == nil && ok {
				matches = append(matches, i)
			}
		} else if candidate == p {
			matches = append(matches, i)
		}
	}
	return matches
}

// expand returns directives with every include replaced by the directives of
// the files it refers to, recursively.
func (r *sbIncludeResolver) expand(directives []sbDirective, stack []int) []sbDirective {
	out := make([]sbDirective, 0, len(directives))
	for _, d := range directives {
		if d.name == "include" && !d.hasBlock && len(d.args) == 1 {
			if len(stack) >= maxIncludeDepth {
				continue
			}
			for _, idx := range r.match(d.args[0]) {
				if containsInt(stack, idx) {
					continue
				}
				r.reached[idx] = true
				out = append(out, r.expand(r.parsed[idx], append(stack, idx))...)
			}
			continue
		}
		if d.hasBlock {
			d.block = r.expand(d.block, stack)
		}
		out = append(out, d)
	}
	return out
}

func containsInt(values []int, v int) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

func parseServerBlocksFromSections(sections []nginxTSection) []ServerBlock {
	if len(sections) == 0 {
		return nil
	}

	r := newSbIncludeResolver(sections)
	r.reached[0] = true
	root := r.expand(r.parsed[0], []int{0})

	var blocks []ServerBlock
	for _, d := range root {
		if d.name == "http" && d.hasBlock {
			blocks = appendHTTPServers(blocks, d.block)
		}
	}

	// Sections never reached from the main file have an unknown context.
	for i := range sections {
		if r.reached[i] {
			continue
		}
		for _, d := range r.expand(r.parsed[i], []int{i}) {
			switch {
			case d.name == "http" && d.hasBlock:
				blocks = appendHTTPServers(blocks, d.block)
			case d.name == "server" && d.hasBlock:
				block := buildServerBlock(d)
				if len(block.ServerNames) > 0 || len(block.Locations) > 0 {
					blocks = append(blocks, block)
				}
			}
		}
	}

	return blocks
}

// appendHTTPServers appends the server blocks directly inside an http block.
func appendHTTPServers(blocks []ServerBlock, httpBody []sbDirective) []ServerBlock {
	for _, d := range httpBody {
		if d.name == "server" && d.hasBlock {
			blocks = append(blocks, buildServerBlock(d))
		}
	}
	return blocks
}

func buildServerBlock(d sbDirective) ServerBlock {
	block := ServerBlock{File: d.file}
	returnSet := false

	for _, child := range d.block {
		switch child.name {
		case "server_name":
			if !child.hasBlock {
				block.ServerNames = append(block.ServerNames, child.args...)
			}
		case "listen":
			if child.hasBlock || len(child.args) == 0 {
				continue
			}
			if l, ok := parseServerListen(child.args); ok && !containsListen(block.Listens, l) {
				block.Listens = append(block.Listens, l)
			}
		case "return":
			if !child.hasBlock && !returnSet {
				block.Return = strings.Join(child.args, " ")
				returnSet = true
			}
		case "location":
			if child.hasBlock {
				block.Locations = append(block.Locations, buildServerLocation(child))
			}
		}
	}

	if len(block.Listens) == 0 && !hasListenDirective(d.block) {
		// Nginx defaults to "listen *:80" (or *:8000 when not running as
		// root); nginx-ui manages a root-run Nginx, so assume port 80.
		block.Listens = []ServerListen{{Addr: "", Port: "80"}}
	}

	return block
}

// hasListenDirective reports whether a server has listen directives at all;
// a server whose only listens are skipped (unix sockets, quic) must not be
// given the implicit port 80 listen.
func hasListenDirective(directives []sbDirective) bool {
	for _, d := range directives {
		if d.name == "listen" && !d.hasBlock {
			return true
		}
	}
	return false
}

func containsListen(listens []ServerListen, l ServerListen) bool {
	for _, existing := range listens {
		if strings.EqualFold(existing.Addr, l.Addr) && existing.Port == l.Port && existing.IPv6 == l.IPv6 {
			return true
		}
	}
	return false
}

func buildServerLocation(d sbDirective) ServerLocation {
	var loc ServerLocation
	switch len(d.args) {
	case 0:
	case 1:
		name := d.args[0]
		switch {
		case strings.HasPrefix(name, "="):
			loc.Modifier, loc.Path = "=", name[1:]
		case strings.HasPrefix(name, "^~"):
			loc.Modifier, loc.Path = "^~", name[2:]
		case strings.HasPrefix(name, "~*"):
			loc.Modifier, loc.Path = "~*", name[2:]
		case strings.HasPrefix(name, "~"):
			loc.Modifier, loc.Path = "~", name[1:]
		default:
			loc.Path = name
		}
	default:
		loc.Modifier, loc.Path = d.args[0], d.args[1]
	}

	proxySet, returnSet := false, false
	for _, child := range d.block {
		if child.hasBlock {
			continue
		}
		switch child.name {
		case "proxy_pass":
			if !proxySet && len(child.args) > 0 {
				loc.ProxyPass = child.args[0]
				proxySet = true
			}
		case "return":
			if !returnSet {
				loc.Return = strings.Join(child.args, " ")
				returnSet = true
			}
		}
	}
	return loc
}

// parseServerListen parses the params of an http listen directive. ok is
// false for listens that are not HTTP/1 TCP sockets (unix sockets, quic).
func parseServerListen(args []string) (ServerListen, bool) {
	var l ServerListen
	spec := args[0]

	if strings.HasPrefix(strings.ToLower(spec), "unix:") {
		return l, false
	}

	for _, param := range args[1:] {
		switch strings.ToLower(param) {
		case "ssl":
			l.SSL = true
		case "default_server", "default":
			l.DefaultServer = true
		case "quic":
			return l, false
		}
	}

	switch {
	case strings.HasPrefix(spec, "["):
		end := strings.Index(spec, "]")
		if end < 0 {
			return l, false
		}
		l.IPv6 = true
		l.Addr = strings.ToLower(spec[1:end])
		rest := spec[end+1:]
		if strings.HasPrefix(rest, ":") && len(rest) > 1 {
			l.Port = rest[1:]
		} else {
			l.Port = "80"
		}
		if l.Addr == "::" || l.Addr == "::0" || l.Addr == "0:0:0:0:0:0:0:0" {
			l.Addr = ""
		}
	case isAllDigits(spec):
		l.Port = spec
	default:
		if i := strings.LastIndex(spec, ":"); i >= 0 {
			l.Addr, l.Port = spec[:i], spec[i+1:]
		} else {
			l.Addr, l.Port = spec, "80"
		}
		if l.Port == "" {
			l.Port = "80"
		}
		l.Addr = strings.ToLower(l.Addr)
		if l.Addr == "*" || l.Addr == "0.0.0.0" {
			l.Addr = ""
		}
	}

	return l, true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// normalizeConfPath turns a path into a forward-slash, cleaned form so that
// Windows paths such as `C:\nginx/conf/nginx.conf` compare with patterns
// written either way. Windows paths are lower-cased because their file
// systems are case-insensitive.
func normalizeConfPath(p string) string {
	p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
	if isWindowsDrivePath(p) {
		p = strings.ToLower(p)
	}
	return p
}

func isWindowsDrivePath(p string) bool {
	return len(p) >= 2 && p[1] == ':' &&
		((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z'))
}

func isAbsConfPath(p string) bool {
	return strings.HasPrefix(p, "/") || isWindowsDrivePath(p)
}

// sbTokenKind is the kind of a configuration token.
type sbTokenKind int

const (
	sbTokenEOF sbTokenKind = iota
	sbTokenWord
	sbTokenSemicolon
	sbTokenOpen
	sbTokenClose
)

type sbToken struct {
	kind sbTokenKind
	text string
}

// sbTokenizer reads tokens following ngx_conf_read_token: "#" starts a
// comment only at the start of a token, quotes only open at the start of a
// token, "}" only ends a block at the start of a token, and "${" does not
// open a block.
type sbTokenizer struct {
	src string
	pos int
}

func (t *sbTokenizer) next() sbToken {
	for t.pos < len(t.src) {
		c := t.src[t.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			t.pos++
		case c == '#':
			for t.pos < len(t.src) && t.src[t.pos] != '\n' {
				t.pos++
			}
		case c == ';':
			t.pos++
			return sbToken{kind: sbTokenSemicolon}
		case c == '{':
			t.pos++
			return sbToken{kind: sbTokenOpen}
		case c == '}':
			t.pos++
			return sbToken{kind: sbTokenClose}
		case c == '"' || c == '\'':
			return sbToken{kind: sbTokenWord, text: t.readQuoted(c)}
		default:
			return sbToken{kind: sbTokenWord, text: t.readWord()}
		}
	}
	return sbToken{kind: sbTokenEOF}
}

func (t *sbTokenizer) readQuoted(quote byte) string {
	t.pos++ // opening quote
	start := t.pos
	for t.pos < len(t.src) {
		c := t.src[t.pos]
		if c == '\\' && t.pos+1 < len(t.src) {
			t.pos += 2
			continue
		}
		if c == quote {
			text := t.src[start:t.pos]
			t.pos++
			return unescapeNginxWord(text)
		}
		t.pos++
	}
	return unescapeNginxWord(t.src[start:])
}

func (t *sbTokenizer) readWord() string {
	start := t.pos
	variable := false
	for t.pos < len(t.src) {
		c := t.src[t.pos]
		if c == '{' && variable {
			variable = false
			t.pos++
			continue
		}
		variable = false
		switch c {
		case '\\':
			t.pos += 2
			if t.pos > len(t.src) {
				t.pos = len(t.src)
			}
			continue
		case '$':
			variable = true
		case ' ', '\t', '\r', '\n', ';', '{':
			return unescapeNginxWord(t.src[start:t.pos])
		}
		t.pos++
	}
	return unescapeNginxWord(t.src[start:])
}

// skipLuaBlock skips the body of a *_by_lua_block after its opening brace.
// The body is Lua code, so braces inside Lua strings and comments must not
// count.
func (t *sbTokenizer) skipLuaBlock() {
	depth := 1
	for t.pos < len(t.src) {
		c := t.src[t.pos]
		switch {
		case c == '-' && strings.HasPrefix(t.src[t.pos:], "--"):
			t.pos += 2
			if level, ok := t.longBracketLevel(); ok {
				t.skipLongBracket(level)
				continue
			}
			for t.pos < len(t.src) && t.src[t.pos] != '\n' {
				t.pos++
			}
		case c == '"' || c == '\'':
			t.pos++
			for t.pos < len(t.src) {
				ch := t.src[t.pos]
				if ch == '\\' {
					t.pos += 2
					continue
				}
				t.pos++
				if ch == c || ch == '\n' {
					break
				}
			}
		case c == '[':
			if level, ok := t.longBracketLevel(); ok {
				t.skipLongBracket(level)
				continue
			}
			t.pos++
		case c == '{':
			depth++
			t.pos++
		case c == '}':
			depth--
			t.pos++
			if depth == 0 {
				return
			}
		default:
			t.pos++
		}
	}
}

// longBracketLevel reports whether a Lua long bracket ("[[" or "[==[")
// starts at the current position, and its level.
func (t *sbTokenizer) longBracketLevel() (int, bool) {
	if t.pos >= len(t.src) || t.src[t.pos] != '[' {
		return 0, false
	}
	i := t.pos + 1
	level := 0
	for i < len(t.src) && t.src[i] == '=' {
		level++
		i++
	}
	if i < len(t.src) && t.src[i] == '[' {
		return level, true
	}
	return 0, false
}

func (t *sbTokenizer) skipLongBracket(level int) {
	closing := "]" + strings.Repeat("=", level) + "]"
	t.pos += level + 2
	if end := strings.Index(t.src[t.pos:], closing); end >= 0 {
		t.pos += end + len(closing)
		return
	}
	t.pos = len(t.src)
}

// unescapeNginxWord applies the escapes ngx_conf_read_token resolves.
func unescapeNginxWord(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"', '\'', '\\':
				b.WriteByte(s[i+1])
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseSbDirectives parses configuration text into a directive tree. It is
// lenient: an unmatched "}" at the top level is ignored and an unterminated
// block is closed at the end of the input.
func parseSbDirectives(content, file string) []sbDirective {
	t := &sbTokenizer{src: content}
	directives, _ := parseSbBlock(t, file, 0)
	return directives
}

func parseSbBlock(t *sbTokenizer, file string, depth int) ([]sbDirective, bool) {
	var directives []sbDirective
	var args []string

	for {
		tok := t.next()
		switch tok.kind {
		case sbTokenEOF:
			return directives, false
		case sbTokenWord:
			args = append(args, tok.text)
		case sbTokenSemicolon:
			if len(args) > 0 {
				directives = append(directives, sbDirective{name: args[0], args: args[1:], file: file})
			}
			args = nil
		case sbTokenOpen:
			d := sbDirective{hasBlock: true, file: file}
			if len(args) > 0 {
				d.name, d.args = args[0], args[1:]
			}
			args = nil
			if strings.HasSuffix(d.name, "_by_lua_block") {
				t.skipLuaBlock()
			} else {
				d.block, _ = parseSbBlock(t, file, depth+1)
			}
			directives = append(directives, d)
		case sbTokenClose:
			if depth > 0 {
				return directives, true
			}
			args = nil
		}
	}
}
