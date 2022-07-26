package nginx

import (
	"bufio"
	"regexp"
	"strings"
	"unicode"
)

func (c *NgxConfig) parseServer(scanner *bufio.Scanner) {
	server := NewNgxServer()
	server.Directives = make(NgxDirectives)
	for scanner.Scan() {
		d := parseDirective(scanner)
		switch d.Directive {
		case Location:
			server.parseLocation(d.Params)
		case CommentStart:
			server.commentQueue.Enqueue(d.Params)
		default:
			server.parseDirective(d)
		}
	}

	// attach comments which are over the current server
	server.Comments = c.commentQueue.DequeueAllComments()

	c.Servers = append(c.Servers, *server)
}

func (c *NgxConfig) parseUpstream(scanner *bufio.Scanner) {
	upstream := NgxUpstream{}
	upstream.Directives = make(NgxDirectives)
	d := NgxDirective{}
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		// escape empty line or comment line
		if len(text) < 1 || text[0] == '#' {
			return
		}

		sep := len(text) - 1
		for k, v := range text {
			if unicode.IsSpace(v) {
				sep = k
				break
			}
		}

		d.Directive = text[0:sep]
		d.Params = strings.Trim(text[sep:], ";")

		if d.Directive == Server {
			upstream.Directives[d.Directive] = append(upstream.Directives[d.Directive], d)
		} else if upstream.Name == "" {
			upstream.Name = d.Directive
		}
	}
	// attach comments which are over the current upstream
	upstream.Comments = c.commentQueue.DequeueAllComments()

	c.Upstreams = append(c.Upstreams, upstream)
}

func (s *NgxServer) parseDirective(d NgxDirective) {
	orig := d.Orig()
	// handle inline comments
	str, comments, _ := strings.Cut(orig, "#")

	regExp := regexp.MustCompile("(\\S+?)\\s+{?(.+?)[;|}]")
	matchSlice := regExp.FindAllStringSubmatch(str, -1)

	for k, v := range matchSlice {
		// [[gzip_min_length 256; gzip_min_length 256] [gzip_proxied expired no-cache no-store private no_last_modified no_etag auth; gzip_proxied expired no-cache no-store private no_last_modified no_etag auth] [gzip on; gzip on] [gzip_vary on; gzip_vary on] [location /x/ {} location /x/ {] [gzip_comp_level 4; gzip_comp_level 4]]
		if len(v) > 0 {
			scanner := bufio.NewScanner(strings.NewReader(v[0]))
			if scanner.Scan() {
				d = parseDirective(scanner)
				// inline location
				if d.Directive == Location {
					s.parseLocation(d.Orig())
				} else {

					if k == 0 {
						d.Comments = s.commentQueue.DequeueAllComments()
					} else if k == len(matchSlice)-1 {
						d.Comments = comments
					}

					// trim right ';'
					d.TrimParams()
					// map[directive]=>[]Params
					s.Directives[d.Directive] = append(s.Directives[d.Directive], d)
				}

			}
		}
	}
}

func (s *NgxServer) parseLocation(str string) {
	path, content, _ := strings.Cut(str, "{")
	content = strings.TrimSpace(content)
	content = strings.Trim(content, "}")
	location := NgxLocation{
		Path:    path,
		Content: content,
	}
	location.Comments = s.commentQueue.DequeueAllComments()
	s.Locations = append(s.Locations, location)
}
