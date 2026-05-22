package nginx

import (
	"path/filepath"
	"strings"

	"github.com/tufanbarisyildirim/gonginx/config"
)

type NgxConfig struct {
	FileName  string         `json:"file_name"`
	Name      string         `json:"name"`
	RootBlock string         `json:"root_block,omitempty"`
	Upstreams []*NgxUpstream `json:"upstreams"`
	Servers   []*NgxServer   `json:"servers"`
	Custom    string         `json:"custom"`
	c         *config.Config
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
	// Raw, when non-empty, is the verbatim source text of the directive (including
	// any block body). BuildConfig prefers Raw over Directive/Params so that block
	// directives (e.g. ssl_certificate_by_lua_block) and quoted parameters survive
	// a maintenance-config rebuild without being flattened. Keep Directive and
	// Params populated for callers that consume this struct via JSON.
	Raw string `json:"-"`
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
		Name:      filepath.Base(filename),
	}
}
