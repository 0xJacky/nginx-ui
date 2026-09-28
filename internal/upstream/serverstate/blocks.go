// Package serverstate lists the servers of every upstream block in the nginx
// configuration and switches a single server on or off through the `down`
// parameter, the same flag the site editor toggles.
//
// Files are edited in place: only the `down` token of the targeted server line
// is added or removed, so comments, formatting and every other directive of a
// hand-written site file stay byte for byte as they were.
package serverstate

import (
	"sort"
	"strconv"
	"strings"
)

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenSemicolon
	tokenOpenBrace
	tokenCloseBrace
	tokenComment
)

// token is one lexical element of an nginx configuration file together with
// its byte offsets in the source.
type token struct {
	kind  tokenKind
	value string // unquoted value of a word
	start int
	end   int
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// tokenize splits content the way nginx reads a configuration file: words are
// separated by whitespace, `;`, `{` and `}` stand on their own, `#` starts a
// comment at the beginning of a word, quotes group a word and a backslash
// escapes the next character. `${var}` stays inside its word.
func tokenize(content string) []token {
	var tokens []token
	n := len(content)
	i := 0
	for i < n {
		c := content[i]
		switch {
		case isSpace(c):
			i++
		case c == '#':
			start := i
			for i < n && content[i] != '\n' {
				i++
			}
			end := i
			for end > start && content[end-1] == '\r' {
				end--
			}
			tokens = append(tokens, token{kind: tokenComment, value: content[start:end], start: start, end: end})
		case c == ';':
			tokens = append(tokens, token{kind: tokenSemicolon, start: i, end: i + 1})
			i++
		case c == '{':
			tokens = append(tokens, token{kind: tokenOpenBrace, start: i, end: i + 1})
			i++
		case c == '}':
			tokens = append(tokens, token{kind: tokenCloseBrace, start: i, end: i + 1})
			i++
		case c == '"' || c == '\'':
			start := i
			quote := c
			i++
			var b strings.Builder
			for i < n && content[i] != quote {
				if content[i] == '\\' && i+1 < n {
					i++
				}
				b.WriteByte(content[i])
				i++
			}
			if i < n {
				i++ // closing quote
			}
			tokens = append(tokens, token{kind: tokenWord, value: b.String(), start: start, end: i})
		default:
			start := i
			var b strings.Builder
			for i < n {
				ch := content[i]
				if isSpace(ch) || ch == ';' || ch == '}' {
					break
				}
				if ch == '{' {
					// `${name}` is a variable, any other brace opens a block.
					if i == start || content[i-1] != '$' {
						break
					}
					for i < n && content[i] != '}' {
						b.WriteByte(content[i])
						i++
					}
					if i < n {
						b.WriteByte('}')
						i++
					}
					continue
				}
				if ch == '\\' && i+1 < n {
					i++
				}
				b.WriteByte(content[i])
				i++
			}
			tokens = append(tokens, token{kind: tokenWord, value: b.String(), start: start, end: i})
		}
	}
	return tokens
}

// ServerLine is one `server` directive inside an upstream block.
type ServerLine struct {
	Address string
	// Params holds every parameter after the address, as written.
	Params []string
	// args are the word tokens after the directive name, address first.
	args []token
}

// Down reports whether the server carries the `down` parameter.
func (s ServerLine) Down() bool {
	for _, param := range s.Params {
		if param == "down" {
			return true
		}
	}
	return false
}

// Backup reports whether the server is a backup server.
func (s ServerLine) Backup() bool {
	for _, param := range s.Params {
		if param == "backup" {
			return true
		}
	}
	return false
}

// Weight returns the configured weight, or 0 when the default applies.
func (s ServerLine) Weight() int {
	for _, param := range s.Params {
		if value, ok := strings.CutPrefix(param, "weight="); ok {
			if n, err := strconv.Atoi(value); err == nil {
				return n
			}
		}
	}
	return 0
}

// OtherParams returns the parameters besides down, backup and weight.
func (s ServerLine) OtherParams() string {
	other := make([]string, 0, len(s.Params))
	for _, param := range s.Params {
		if param == "down" || param == "backup" || strings.HasPrefix(param, "weight=") {
			continue
		}
		other = append(other, param)
	}
	return strings.Join(other, " ")
}

// Directive is one simple directive (ending with `;`) directly inside an
// upstream block.
type Directive struct {
	Name string
	// Args are the unquoted parameter values.
	Args []string
	// RawArgs are the parameters exactly as written, quotes and escapes
	// included, so a directive can be copied without changing its meaning.
	RawArgs []string
}

// Block is one upstream block of a configuration file.
type Block struct {
	Name    string
	Servers []ServerLine
	// Directives lists every simple directive of the block in file order,
	// the server lines included.
	Directives []Directive
	// Comments holds the comments inside the block, `#` included, in file
	// order.
	Comments []string
	// NestedBlocks names the blocks opened inside the upstream block, which
	// the stock upstream module does not have.
	NestedBlocks []string
	// Parent is the directive of the block enclosing the upstream block, for
	// example "http" or "stream"; empty at the top level of the file.
	Parent string
	// Start and End are the byte offsets of the whole block in the source,
	// from the `upstream` keyword to the closing brace. End is 0 when the
	// block is not closed.
	Start int
	End   int
}

// ParseBlocks returns the upstream blocks of content in file order. Blocks in
// comments are ignored because comments never form directives.
func ParseBlocks(content string) []Block {
	tokens := tokenize(content)

	type frame struct {
		name     string
		upstream int // index into blocks, -1 for any other block
	}
	var (
		blocks    []Block
		stack     []frame
		directive []token
	)
	current := func() int {
		if len(stack) == 0 {
			return -1
		}
		return stack[len(stack)-1].upstream
	}
	// enclosing returns the innermost upstream block the parser is in, even
	// inside a block nested in it.
	enclosing := func() int {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].upstream >= 0 {
				return stack[i].upstream
			}
		}
		return -1
	}

	for _, tok := range tokens {
		switch tok.kind {
		case tokenComment:
			if idx := enclosing(); idx >= 0 {
				blocks[idx].Comments = append(blocks[idx].Comments, tok.value)
			}
		case tokenWord:
			directive = append(directive, tok)
		case tokenSemicolon:
			if idx := current(); idx >= 0 && len(directive) >= 1 {
				d := Directive{
					Name:    directive[0].value,
					Args:    make([]string, 0, len(directive)-1),
					RawArgs: make([]string, 0, len(directive)-1),
				}
				for _, arg := range directive[1:] {
					d.Args = append(d.Args, arg.value)
					d.RawArgs = append(d.RawArgs, content[arg.start:arg.end])
				}
				blocks[idx].Directives = append(blocks[idx].Directives, d)
			}
			if idx := current(); idx >= 0 && len(directive) >= 2 && directive[0].value == "server" {
				args := append([]token(nil), directive[1:]...)
				params := make([]string, 0, len(args)-1)
				for _, arg := range args[1:] {
					params = append(params, arg.value)
				}
				blocks[idx].Servers = append(blocks[idx].Servers, ServerLine{
					Address: args[0].value,
					Params:  params,
					args:    args,
				})
			}
			directive = directive[:0]
		case tokenOpenBrace:
			name := ""
			if len(directive) > 0 {
				name = directive[0].value
			}
			f := frame{name: name, upstream: -1}
			if idx := enclosing(); idx >= 0 {
				blocks[idx].NestedBlocks = append(blocks[idx].NestedBlocks, name)
			} else if len(directive) == 2 && directive[0].value == "upstream" {
				parent := ""
				if len(stack) > 0 {
					parent = stack[len(stack)-1].name
				}
				blocks = append(blocks, Block{
					Name:       directive[1].value,
					Servers:    []ServerLine{},
					Directives: []Directive{},
					Parent:     parent,
					Start:      directive[0].start,
				})
				f.upstream = len(blocks) - 1
			}
			stack = append(stack, f)
			directive = directive[:0]
		case tokenCloseBrace:
			if len(stack) > 0 {
				if idx := stack[len(stack)-1].upstream; idx >= 0 {
					blocks[idx].End = tok.end
				}
				stack = stack[:len(stack)-1]
			}
			directive = directive[:0]
		}
	}
	return blocks
}

// edit replaces content[start:end] with text.
type edit struct {
	start, end int
	text       string
}

// SetDown adds or removes the `down` parameter on every `server` line whose
// address is address inside the upstream blocks named name. It returns the new
// content, how many server lines matched and whether anything changed. Only the
// `down` token and the whitespace in front of it are touched.
func SetDown(content, name, address string, down bool) (string, int, bool) {
	var edits []edit
	matched := 0
	for _, block := range ParseBlocks(content) {
		if block.Name != name {
			continue
		}
		for _, server := range block.Servers {
			if server.Address != address {
				continue
			}
			matched++
			edits = append(edits, serverEdits(content, server, down)...)
		}
	}
	if len(edits) == 0 {
		return content, matched, false
	}

	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		content = content[:e.start] + e.text + content[e.end:]
	}
	return content, matched, true
}

// serverEdits returns the edits that bring one server line to the wanted state.
func serverEdits(content string, server ServerLine, down bool) []edit {
	if down {
		if server.Down() {
			return nil
		}
		last := server.args[len(server.args)-1]
		return []edit{{start: last.end, end: last.end, text: " down"}}
	}

	var edits []edit
	for i := 1; i < len(server.args); i++ {
		arg := server.args[i]
		if arg.value != "down" {
			continue
		}
		// Drop the whitespace between the previous parameter and `down`. When a
		// comment sits in between, keep the line break that ends it, otherwise
		// the terminating `;` would end up inside the comment.
		prevEnd := server.args[i-1].end
		hasComment := strings.Contains(content[prevEnd:arg.start], "#")
		start := arg.start
		for start > prevEnd && isSpace(content[start-1]) {
			if hasComment && (content[start-1] == '\n' || content[start-1] == '\r') {
				break
			}
			start--
		}
		edits = append(edits, edit{start: start, end: arg.end})
	}
	return edits
}
