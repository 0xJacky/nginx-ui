package nginx

import (
	"bufio"
	"fmt"
	"github.com/tufanbarisyildirim/gonginx"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"strings"
)

func buildComments(orig string, indent int) (content string) {
	scanner := bufio.NewScanner(strings.NewReader(orig))
	for scanner.Scan() {
		content += strings.Repeat("\t", indent) + "# " + strings.TrimSpace(scanner.Text()) + "\n"
	}
	content = strings.TrimLeft(content, "\n")
	return
}

func (c *NgxConfig) BuildConfig() (content string, err error) {
	// Custom
	if c.Custom != "" {
		content += c.Custom
		content += "\n\n"
	}

	// Upstreams
	for _, u := range c.Upstreams {

		upstream := ""
		var comments string
		for _, directive := range u.Directives {
			if directive.Comments != "" {
				comments = buildComments(directive.Comments, 1)
			}
			upstream += fmt.Sprintf("%s\t%s;\n", comments, directive.Orig())
		}
		comments = buildComments(u.Comments, 1)
		content += fmt.Sprintf("upstream %s {\n%s%s}\n\n", u.Name, comments, upstream)
	}

	// Servers
	for _, s := range c.Servers {
		server := ""

		// directives
		for _, directive := range s.Directives {
			var comments string
			if directive.Comments != "" {
				comments = buildComments(directive.Comments, 1)
			}
			if directive.Params != "" {
				server += fmt.Sprintf("%s\t%s;\n", comments, directive.Orig())
			}
		}

		if len(s.Directives) > 0 {
			server += "\n"
		}

		// locations
		locations := ""
		for _, location := range s.Locations {
			locationContent := ""
			scanner := bufio.NewScanner(strings.NewReader(location.Content))
			for scanner.Scan() {
				locationContent += "\t\t" + scanner.Text() + "\n"
			}
			var comments string
			if location.Comments != "" {
				comments = buildComments(location.Comments, 1)
			}
			locations += fmt.Sprintf("%s\tlocation %s {\n%s\t}\n\n", comments, location.Path, locationContent)
		}

		server += locations

		var comments string
		if s.Comments != "" {
			comments = buildComments(s.Comments, 0) + "\n"
		}

		content += fmt.Sprintf("%sserver {\n%s}\n\n", comments, server)
	}
	p := parser.NewStringParser(content, parser.WithSkipValidDirectivesErr())
	config, err := p.Parse()
	if err != nil {
		return
	}

	content = gonginx.DumpConfig(config, gonginx.IndentedStyle)
	return
}
