package nginx

import (
	"github.com/pkg/errors"
	"github.com/tufanbarisyildirim/gonginx"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"strings"
)

const (
	Server   = "server"
	Location = "location"
	Upstream = "upstream"
)

func (s *NgxServer) parseServer(directive gonginx.IDirective) {
	if directive.GetBlock() == nil {
		return
	}
	for _, d := range directive.GetBlock().GetDirectives() {
		switch d.GetName() {
		case Location:
			location := &NgxLocation{
				Path:     strings.Join(d.GetParameters(), " "),
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

func (l *NgxLocation) parseLocation(directive gonginx.IDirective, deep int) {
	if directive.GetBlock() == nil {
		return
	}
	for _, location := range directive.GetBlock().GetDirectives() {
		if len(location.GetComment()) > 0 {
			for _, c := range location.GetComment() {
				l.Content += strings.Repeat("\t", deep) + c + "\n"
			}
		}
		l.Content += strings.Repeat("\t", deep) + location.GetName() + " " + strings.Join(location.GetParameters(), " ") + ";\n"
		l.parseLocation(location, deep+1)
	}
}

func (d *NgxDirective) parseDirective(directive gonginx.IDirective, deep int) {
	if directive.GetBlock() != nil {
		d.Params += directive.GetName() + " "
		d.Directive = ""
	}
	d.Params += strings.Join(directive.GetParameters(), " ")
	if directive.GetBlock() != nil {
		d.Params += " {\n"
		for _, location := range directive.GetBlock().GetDirectives() {
			if len(location.GetComment()) > 0 {
				for _, c := range location.GetComment() {
					d.Params += strings.Repeat("\t", deep) + c + "\n"
				}
			}
			d.Params += strings.Repeat("\t", deep+1) + location.GetName() + " " +
				strings.Join(location.GetParameters(), " ") + ";\n"
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

func (u *NgxUpstream) parseUpstream(directive gonginx.IDirective) {
	if directive.GetBlock() == nil {
		return
	}
	for _, us := range directive.GetBlock().GetDirectives() {
		d := &NgxDirective{
			Directive: us.GetName(),
			Params:    strings.Join(us.GetParameters(), " "),
			Comments:  buildComment(us.GetComment()),
		}
		u.Directives = append(u.Directives, d)
	}
}

func (c *NgxConfig) parseCustom(directive gonginx.IDirective) {
	if directive.GetBlock() == nil {
		return
	}
	c.Custom += "{\n"
	for _, v := range directive.GetBlock().GetDirectives() {
		c.Custom += strings.Join(v.GetComment(), "\n") + "\n" +
			v.GetName() + " " + strings.Join(v.GetParameters(), " ") + ";\n"
	}
	c.Custom += "}\n"
}

func buildComment(c []string) string {
	return strings.ReplaceAll(strings.Join(c, "\n"), "#", "")
}

func parse(block gonginx.IBlock, ngxConfig *NgxConfig) {
	if block == nil {
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
			upstream := &NgxUpstream{}
			upstream.Comments = comments
			upstream.parseUpstream(v)
			ngxConfig.Upstreams = append(ngxConfig.Upstreams, upstream)
		default:
			ngxConfig.Custom += strings.Join(v.GetComment(), "\n") + "\n" +
				v.GetName() + " " + strings.Join(v.GetParameters(), " ") + "\n"
			ngxConfig.parseCustom(v)
		}
	}
}

func ParseNgxConfigByContent(content string) (ngxConfig *NgxConfig) {
	p := parser.NewStringParser(content)
	c := p.Parse()
	ngxConfig = NewNgxConfig("")
	ngxConfig.c = c
	parse(c.Block, ngxConfig)
	return
}

func ParseNgxConfig(filename string) (ngxConfig *NgxConfig, err error) {
	p, err := parser.NewParser(filename)
	if err != nil {
		return nil, errors.Wrap(err, "error ParseNgxConfig")
	}
	c := p.Parse()
	ngxConfig = NewNgxConfig(filename)
	ngxConfig.c = c
	parse(c.Block, ngxConfig)
	return
}
