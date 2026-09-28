package access_list

import (
	"strings"
)

// statement is one directive of an Nginx configuration together with the byte
// offsets it occupies in the source text. Offsets let the text editing helpers
// insert and remove single directives while leaving every other byte of the
// file untouched, which is what advanced mode sites require.
type statement struct {
	Name string
	Args []string
	// Start is the offset of the directive name, End the offset right after
	// the terminating ';' or '}'.
	Start int
	End   int
	// BodyStart is the offset right after '{' and BodyEnd the offset of the
	// matching '}'. Both are -1 for simple directives.
	BodyStart int
	BodyEnd   int
	// Comments are the comment lines directly above the directive.
	Comments []comment
	Children []*statement
}

type comment struct {
	Text  string
	Start int
	End   int
}

func (s *statement) hasBody() bool {
	return s.BodyStart >= 0
}

func (s *statement) arg(i int) string {
	if i < len(s.Args) {
		return s.Args[i]
	}
	return ""
}

type scanner struct {
	src string
	pos int
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokWord
	tokSemicolon
	tokOpen
	tokClose
	tokComment
)

type token struct {
	kind  tokenKind
	text  string
	start int
	end   int
}

func (s *scanner) next() (token, error) {
	for s.pos < len(s.src) && isSpace(s.src[s.pos]) {
		s.pos++
	}
	if s.pos >= len(s.src) {
		return token{kind: tokEOF, start: s.pos, end: s.pos}, nil
	}

	start := s.pos
	switch c := s.src[s.pos]; c {
	case ';':
		s.pos++
		return token{kind: tokSemicolon, text: ";", start: start, end: s.pos}, nil
	case '{':
		s.pos++
		return token{kind: tokOpen, text: "{", start: start, end: s.pos}, nil
	case '}':
		s.pos++
		return token{kind: tokClose, text: "}", start: start, end: s.pos}, nil
	case '#':
		for s.pos < len(s.src) && s.src[s.pos] != '\n' {
			s.pos++
		}
		return token{kind: tokComment, text: s.src[start:s.pos], start: start, end: s.pos}, nil
	case '"', '\'':
		text, err := s.quoted(c)
		if err != nil {
			return token{}, err
		}
		return token{kind: tokWord, text: text, start: start, end: s.pos}, nil
	}

	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if isSpace(c) || c == ';' || c == '{' || c == '}' {
			break
		}
		// ${name} is a variable, not a block.
		if c == '$' && s.pos+1 < len(s.src) && s.src[s.pos+1] == '{' {
			closing := strings.IndexByte(s.src[s.pos:], '}')
			if closing < 0 {
				return token{}, ErrConfigSyntax
			}
			b.WriteString(s.src[s.pos : s.pos+closing+1])
			s.pos += closing + 1
			continue
		}
		if c == '\\' && s.pos+1 < len(s.src) {
			b.WriteByte(c)
			b.WriteByte(s.src[s.pos+1])
			s.pos += 2
			continue
		}
		b.WriteByte(c)
		s.pos++
	}
	return token{kind: tokWord, text: b.String(), start: start, end: s.pos}, nil
}

func (s *scanner) quoted(quote byte) (string, error) {
	s.pos++
	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == '\\' && s.pos+1 < len(s.src) {
			b.WriteByte(s.src[s.pos+1])
			s.pos += 2
			continue
		}
		if c == quote {
			s.pos++
			return b.String(), nil
		}
		b.WriteByte(c)
		s.pos++
	}
	return "", ErrConfigSyntax
}

// skipRawBlock consumes the body of a *_by_lua_block style directive, whose
// content is not Nginx syntax. It balances braces outside of quotes.
func (s *scanner) skipRawBlock() (int, error) {
	depth := 1
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch c {
		case '"', '\'':
			if _, err := s.quoted(c); err != nil {
				return 0, err
			}
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end := s.pos
				s.pos++
				return end, nil
			}
		}
		s.pos++
	}
	return 0, ErrConfigSyntax
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// parseStatements parses src into a statement tree. Offsets are relative to
// src.
func parseStatements(src string) ([]*statement, error) {
	s := &scanner{src: src}
	children, closed, err := s.parseBlock()
	if err != nil {
		return nil, err
	}
	if closed >= 0 {
		return nil, ErrConfigSyntax
	}
	return children, nil
}

// parseBlock parses statements until EOF or a closing brace. It reports the
// offset of the closing brace, or -1 at EOF.
func (s *scanner) parseBlock() ([]*statement, int, error) {
	var (
		children []*statement
		pending  []comment
		current  *statement
	)
	lastEnd := 0

	for {
		tok, err := s.next()
		if err != nil {
			return nil, 0, err
		}
		switch tok.kind {
		case tokEOF:
			if current != nil {
				return nil, 0, ErrConfigSyntax
			}
			return children, -1, nil
		case tokComment:
			if current != nil {
				// Inline comments inside a directive are not attached to anything.
				continue
			}
			// A blank line or another directive on the same line separates a
			// comment from the directive below it.
			if len(pending) > 0 && strings.Count(s.src[pending[len(pending)-1].End:tok.start], "\n") > 1 {
				pending = nil
			}
			if len(pending) == 0 && lastEnd > 0 && !strings.Contains(s.src[lastEnd:tok.start], "\n") {
				continue
			}
			pending = append(pending, comment{Text: tok.text, Start: tok.start, End: tok.end})
		case tokWord:
			if current == nil {
				if len(pending) > 0 && strings.Count(s.src[pending[len(pending)-1].End:tok.start], "\n") > 1 {
					pending = nil
				}
				current = &statement{Name: tok.text, Start: tok.start, BodyStart: -1, BodyEnd: -1, Comments: pending}
				pending = nil
				continue
			}
			current.Args = append(current.Args, tok.text)
		case tokSemicolon:
			if current == nil {
				return nil, 0, ErrConfigSyntax
			}
			current.End = tok.end
			children = append(children, current)
			lastEnd = current.End
			current = nil
		case tokOpen:
			if current == nil {
				return nil, 0, ErrConfigSyntax
			}
			current.BodyStart = tok.end
			if strings.HasSuffix(current.Name, "_by_lua_block") {
				end, err := s.skipRawBlock()
				if err != nil {
					return nil, 0, err
				}
				current.BodyEnd = end
			} else {
				body, end, err := s.parseBlock()
				if err != nil {
					return nil, 0, err
				}
				if end < 0 {
					return nil, 0, ErrConfigSyntax
				}
				current.Children = body
				current.BodyEnd = end
			}
			current.End = current.BodyEnd + 1
			children = append(children, current)
			lastEnd = current.End
			current = nil
		case tokClose:
			if current != nil {
				return nil, 0, ErrConfigSyntax
			}
			return children, tok.start, nil
		}
	}
}

// serverBlocks returns the server blocks of a site or stream file. Server
// blocks usually sit at the top level, but a file may also wrap them in a
// single http or stream block.
func serverBlocks(roots []*statement) []*statement {
	var servers []*statement
	for _, st := range roots {
		switch st.Name {
		case "server":
			if st.hasBody() {
				servers = append(servers, st)
			}
		case "http", "stream":
			servers = append(servers, serverBlocks(st.Children)...)
		}
	}
	return servers
}

// locationBlocks returns the location blocks directly inside a server.
func locationBlocks(server *statement) []*statement {
	var locations []*statement
	for _, st := range server.Children {
		if st.Name == "location" && st.hasBody() {
			locations = append(locations, st)
		}
	}
	return locations
}

// locationPath joins the modifier and URI of a location the way the site
// editor displays them.
func locationPath(location *statement) string {
	return strings.Join(location.Args, " ")
}
