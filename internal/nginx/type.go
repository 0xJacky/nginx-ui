package nginx

import (
	"github.com/tufanbarisyildirim/gonginx"
	"path"
	"strings"
)

type NgxConfig struct {
	FileName  string         `json:"file_name"`
	Name      string         `json:"name"`
	Upstreams []*NgxUpstream `json:"upstreams"`
	Servers   []*NgxServer   `json:"servers"`
	Custom    string         `json:"custom"`
	c         *gonginx.Config
}

type NgxServer struct {
	Directives []*NgxDirective `json:"directives"`
	Locations  []*NgxLocation  `json:"locations"`
	Comments   string          `json:"comments"`
}

type NgxUpstream struct {
	Name       string          `json:"name"`
	Directives []*NgxDirective `json:"directives"`
	Comments   string          `json:"comments"`
}

type NgxDirective struct {
	Directive string `json:"directive"`
	Params    string `json:"params"`
	Comments  string `json:"comments"`
}

type NgxLocation struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Comments string `json:"comments"`
}

func (d *NgxDirective) Orig() string {
	return d.Directive + " " + d.Params
}

func (d *NgxDirective) TrimParams() {
	d.Params = strings.TrimRight(strings.TrimSpace(d.Params), ";")
	return
}

func NewNgxServer() *NgxServer {
	return &NgxServer{
		Locations:  make([]*NgxLocation, 0),
		Directives: make([]*NgxDirective, 0),
	}
}

func NewNgxConfig(filename string) *NgxConfig {
	return &NgxConfig{
		FileName:  filename,
		Upstreams: make([]*NgxUpstream, 0),
		Name:      path.Base(filename),
	}
}
