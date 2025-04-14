package nginx

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

const (
	Server   = "server"
	Location = "location"
	Upstream = "upstream"
)

func (s *NgxServer) ParseServer(directive config.IDirective) {
	s.parseServer(directive)
}

func (s *NgxServer) parseServer(directive config.IDirective) {
	if directive.GetBlock() == nil {
		return
	}
	for _, d := range directive.GetBlock().GetDirectives() {
		switch d.GetName() {
		case Location:
			var params []string
			for _, param := range d.GetParameters() {
				params = append(params, param.Value)
			}
			location := &NgxLocation{
				Path:     strings.Join(params, " "),
				Comments: buildComment(d.GetComment()),
			}
			location.parseLocation(d, 0)
			s.Locations = append(s.Locations, location)
		default:
			dir := &NgxDirective{
				Directive: d.GetName(),
				Comments:  buildComment(d.GetComment()),
			}
			dir.parseDirective(d, 0)
			s.Directives = append(s.Directives, dir)
		}
	}
}
func (l *NgxLocation) ParseLocation(directive config.IDirective, deep int) {
	l.parseLocation(directive, deep)
}
func (l *NgxLocation) parseLocation(directive config.IDirective, deep int) {
	if directive.GetBlock() == nil {
		return
	}
	if directive.GetBlock().GetCodeBlock() != "" {
		// deep copy
		style := *dumper.IndentedStyle
		style.StartIndent = deep * style.Indent
		l.Content += dumper.DumpLuaBlock(directive.GetBlock(), &style) + "\n"
		return
	}
	for _, location := range directive.GetBlock().GetDirectives() {
		if len(location.GetComment()) > 0 {
			for _, c := range location.GetComment() {
				l.Content += strings.Repeat("\t", deep) + c + "\n"
			}
		}
		var params []string
		for _, param := range location.GetParameters() {
			params = append(params, param.Value)
		}
		l.Content += strings.Repeat("\t", deep) + location.GetName() + " " + strings.Join(params, " ")
		if location.GetBlock() != nil && location.GetBlock().GetDirectives() != nil {
			l.Content += " { \n"
			l.parseLocation(location, deep+1)
			l.Content += strings.Repeat("\t", deep) + "} \n"
		} else {
			l.Content += ";\n"
		}
	}
}

func (d *NgxDirective) ParseDirective(directive config.IDirective, deep int) {
	d.parseDirective(directive, deep)
}

func (d *NgxDirective) parseDirective(directive config.IDirective, deep int) {
	if directive.GetBlock() != nil {
		d.Params += directive.GetName() + " "
		d.Directive = ""
	}

	var params []string
	for _, param := range directive.GetParameters() {
		params = append(params, param.Value)
	}

	d.Params += strings.Join(params, " ")
	if directive.GetBlock() != nil {
		d.Params += " {\n"
		for _, location := range directive.GetBlock().GetDirectives() {
			if len(location.GetComment()) > 0 {
				for _, c := range location.GetComment() {
					d.Params += strings.Repeat("\t", deep) + c + "\n"
				}
			}
			var params []string
			for _, param := range location.GetParameters() {
				params = append(params, param.Value)
			}
			d.Params += strings.Repeat("\t", deep+1) + location.GetName() + " " +
				strings.Join(params, " ") + ";\n"
			// d.parseDirective(location, deep+1)
			if location.GetBlock() == nil {
				continue
			}
			for _, v := range location.GetBlock().GetDirectives() {
				d.parseDirective(v, deep+1)
			}
		}
		d.Params += "}\n"
		return
	}
}

func (u *NgxUpstream) parseUpstream(directive config.IDirective) {
	if directive.GetBlock() == nil {
		return
	}
	for _, us := range directive.GetBlock().GetDirectives() {
		var params []string
		for _, param := range us.GetParameters() {
			params = append(params, param.Value)
		}
		d := &NgxDirective{
			Directive: us.GetName(),
			Params:    strings.Join(params, " "),
			Comments:  buildComment(us.GetComment()),
		}
		u.Directives = append(u.Directives, d)
	}
}

func (c *NgxConfig) parseCustom(directive config.IDirective) {
	if directive.GetBlock() == nil {
		// fix #699
		c.Custom += ";\n"
		return
	}
	c.Custom += "\n{\n"
	for _, v := range directive.GetBlock().GetDirectives() {
		var params []string
		for _, param := range v.GetParameters() {
			params = append(params, param.Value)
		}

		inlineComment := ""
		for _, inline := range v.GetInlineComment() {
			inlineComment += inline.Value + " "
		}

		c.Custom += strings.Join(v.GetComment(), "\n") + "\n" +
			v.GetName() + " " + strings.Join(params, " ") + ";" + inlineComment + "\n"
	}
	c.Custom += "}\n"
}

func buildComment(c []string) string {
	return strings.ReplaceAll(strings.Join(c, "\n"), "#", "")
}

func parse(block config.IBlock, ngxConfig *NgxConfig) (err error) {
	if block == nil {
		err = ErrBlockIsNil
		return
	}
	for _, v := range block.GetDirectives() {
		comments := buildComment(v.GetComment())
		switch v.GetName() {
		case Server:
			server := NewNgxServer()
			server.Comments = comments
			server.parseServer(v)
			ngxConfig.Servers = append(ngxConfig.Servers, server)
		case Upstream:
			var params []string
			for _, param := range v.GetParameters() {
				params = append(params, param.Value)
			}
			upstream := &NgxUpstream{
				Name: strings.Join(params, " "),
			}
			upstream.Comments = comments
			upstream.parseUpstream(v)
			ngxConfig.Upstreams = append(ngxConfig.Upstreams, upstream)
		default:
			var params []string
			for _, param := range v.GetParameters() {
				params = append(params, param.Value)
			}
			ngxConfig.Custom += strings.Join(v.GetComment(), "\n") + "\n" +
				v.GetName() + " " + strings.Join(params, " ")
			ngxConfig.parseCustom(v)
		}
	}
	if strings.TrimSpace(ngxConfig.Custom) == "" {
		return
	}

	custom, err := FmtCode(ngxConfig.Custom)
	if err != nil {
		return
	}
	ngxConfig.Custom = custom
	return
}

func ParseNgxConfigByContent(content string) (ngxConfig *NgxConfig, err error) {
	p := parser.NewStringParser(content, parser.WithSkipValidDirectivesErr())
	c, err := p.Parse()
	if err != nil {
		return
	}
	ngxConfig = NewNgxConfig("")
	ngxConfig.c = c
	err = parse(c.Block, ngxConfig)
	return
}

func ParseNgxConfig(filename string) (ngxConfig *NgxConfig, err error) {
	p, err := parser.NewParser(filename, parser.WithSkipValidDirectivesErr())
	if err != nil {
		return nil, errors.Wrap(err, "error ParseNgxConfig")
	}
	c, err := p.Parse()
	if err != nil {
		return
	}
	ngxConfig = NewNgxConfig(filename)
	ngxConfig.c = c
	err = parse(c.Block, ngxConfig)
	return
}
