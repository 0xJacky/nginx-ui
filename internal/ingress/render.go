package ingress

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	internalTemplate "github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

// siteHeaderPrefix starts every generated site file. Apply uses it to
// recognize its own file when the site row is missing.
const siteHeaderPrefix = "# Managed by Nginx UI ingress "

// limitReqZoneSize is the shared memory of the per-ingress request limit.
const limitReqZoneSize = "10m"

// hstsMaxAge is the Strict-Transport-Security max-age in seconds (one year).
const hstsMaxAge = 31536000

// supportsHTTP2Directive reports whether the target nginx understands the
// `http2 on;` directive (1.25.1 and later). Older or unknown versions get the
// `listen ... http2` parameter, which newer versions still accept with a
// deprecation warning. It is a variable so tests do not depend on the
// installed nginx.
var supportsHTTP2Directive = func() bool {
	atLeast, ok := nginx.CoreVersionAtLeast(1, 25, 1)
	return ok && atLeast
}

// challengeLocation returns the ACME HTTP-01 location. The template uses a
// regex location, which a `^~` route prefix such as `/` would shadow, so the
// path becomes a `^~` prefix that always wins over the shorter route
// prefixes (Validate keeps routes out of the challenge path).
var challengeLocation = func() (*nginx.NgxLocation, error) {
	location, err := internalTemplate.LetsEncryptLocation()
	if err != nil {
		return nil, err
	}
	return &nginx.NgxLocation{Path: "^~ " + acmeChallengePrefix, Content: location.Content}, nil
}

// zoneName returns the identifier used for the per-ingress nginx variables
// and the request limit zone.
func zoneName(name string) string {
	return "ig_" + strings.ReplaceAll(name, "-", "_")
}

// directives accumulates the lines of a location body.
type directives []string

func (d *directives) add(name string, params ...string) {
	*d = append(*d, name+" "+strings.Join(params, " ")+";")
}

func (d directives) String() string {
	return strings.Join(d, "\n")
}

// Render turns an ingress into its site and upstream files. The output is
// deterministic: the same ingress and options always give the same bytes.
func Render(ing *model.Ingress, opts RenderOptions) (*Rendered, error) {
	if err := ValidateName(ing.Name); err != nil {
		return nil, err
	}
	spec := &ing.Spec
	if err := Validate(spec); err != nil {
		return nil, err
	}
	hosts, err := normalizeHosts(spec)
	if err != nil {
		return nil, err
	}

	upstreams := make(map[string]string, len(spec.Backends))
	for _, b := range spec.Backends {
		u := buildUpstream(ing.Name, b)
		content, err := managed.Prepare(u)
		if err != nil {
			return nil, cosy.WrapErrorWithParams(ErrInvalidBackend, b.Name, err.Error())
		}
		upstreams[u.Name] = content
	}

	cfg, err := buildSite(ing, hosts, opts)
	if err != nil {
		return nil, err
	}
	content, err := cfg.BuildConfig()
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrApplyFailed, "render site: "+err.Error())
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	rendered := &Rendered{
		SiteName:    SiteName(ing.Name),
		SiteContent: content,
		Upstreams:   upstreams,
	}
	rendered.Hash = hashRendered(rendered)
	return rendered, nil
}

// hashRendered is the sha256 over the site content, then every upstream
// name and content in name order.
func hashRendered(r *Rendered) string {
	h := sha256.New()
	h.Write([]byte(r.SiteContent))
	for _, name := range sortedKeys(r.Upstreams) {
		h.Write([]byte{0})
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(r.Upstreams[name]))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildUpstream(name string, b model.IngressBackend) *managed.Upstream {
	u := &managed.Upstream{
		Name:      UpstreamName(name, b.Name),
		Keepalive: b.Keepalive,
		Zone:      true,
		Servers:   make([]managed.Server, 0, len(b.Servers)),
	}
	switch b.LB {
	case model.IngressLBLeastConn:
		u.Method = managed.MethodLeastConn
	case model.IngressLBIPHash:
		u.Method = managed.MethodIPHash
	default:
		u.Method = managed.MethodRoundRobin
	}
	for _, s := range b.Servers {
		server := managed.Server{
			Address:     canonicalAddress(s.Address),
			FailTimeout: s.FailTimeout,
			Down:        s.Down,
			Backup:      s.Backup,
		}
		if s.Weight > 0 {
			weight := s.Weight
			server.Weight = &weight
		}
		if s.MaxFails > 0 {
			maxFails := s.MaxFails
			server.MaxFails = &maxFails
		}
		u.Servers = append(u.Servers, server)
	}
	return u
}

// canonicalAddress returns the nginx spelling of a validated server address.
func canonicalAddress(address string) string {
	if addr, err := parseServerAddress(address); err == nil {
		return addr.String()
	}
	return address
}

// tlsActive reports whether the HTTPS listener is rendered.
func tlsActive(spec *model.IngressSpec, cert CertState) bool {
	return spec.TLS.Mode != model.IngressTLSModeNone && cert.Ready
}

func buildSite(ing *model.Ingress, hosts []string, opts RenderOptions) (*nginx.NgxConfig, error) {
	spec := &ing.Spec
	serverName := strings.Join(hosts, " ")
	tls := tlsActive(spec, opts.Cert)
	if tls {
		for _, path := range []string{opts.Cert.CertPath, opts.Cert.KeyPath} {
			if !internalTemplate.IsSafeNginxToken(path) || strings.ContainsAny(path, `"'#\`) {
				return nil, cosy.WrapErrorWithParams(ErrCertNotReady, "certificate path "+strconv.Quote(path)+" cannot be used in nginx")
			}
		}
	}

	locations := buildRouteLocations(ing)

	port80 := nginx.NewNgxServer()
	port80.Directives = []*nginx.NgxDirective{
		{Directive: "listen", Params: "80"},
		{Directive: "listen", Params: "[::]:80"},
		{Directive: "server_name", Params: serverName},
	}
	if spec.TLS.Mode == model.IngressTLSModeACME && challengeOf(&spec.TLS) == model.IngressChallengeHTTP01 {
		challenge, err := challengeLocation()
		if err != nil {
			return nil, cosy.WrapErrorWithParams(ErrApplyFailed, "render challenge location: "+err.Error())
		}
		port80.Locations = append(port80.Locations, challenge)
	}
	if tls && spec.TLS.RedirectHTTPS {
		// A server-level return would run before location selection and
		// redirect the ACME challenge too, so the redirect is a location.
		port80.Locations = append(port80.Locations, &nginx.NgxLocation{
			Path:    "/",
			Content: "return 301 https://$host$request_uri;",
		})
	} else {
		port80.Locations = append(port80.Locations, locations...)
	}

	servers := []*nginx.NgxServer{port80}
	if tls {
		servers = append(servers, buildTLSServer(spec, serverName, opts.Cert, locations))
	}

	return &nginx.NgxConfig{
		Name:      SiteName(ing.Name),
		Custom:    buildSiteHeader(ing),
		Upstreams: make([]*nginx.NgxUpstream, 0),
		Servers:   servers,
	}, nil
}

func buildTLSServer(spec *model.IngressSpec, serverName string, cert CertState, locations []*nginx.NgxLocation) *nginx.NgxServer {
	server := nginx.NewNgxServer()
	if supportsHTTP2Directive() {
		server.Directives = append(server.Directives,
			&nginx.NgxDirective{Directive: "listen", Params: "443 ssl"},
			&nginx.NgxDirective{Directive: "listen", Params: "[::]:443 ssl"},
			&nginx.NgxDirective{Directive: "http2", Params: "on"},
		)
	} else {
		server.Directives = append(server.Directives,
			&nginx.NgxDirective{Directive: "listen", Params: "443 ssl http2"},
			&nginx.NgxDirective{Directive: "listen", Params: "[::]:443 ssl http2"},
		)
	}
	server.Directives = append(server.Directives,
		&nginx.NgxDirective{Directive: "server_name", Params: serverName},
		&nginx.NgxDirective{Directive: "ssl_certificate", Params: cert.CertPath},
		&nginx.NgxDirective{Directive: "ssl_certificate_key", Params: cert.KeyPath},
	)
	if spec.TLS.HSTS {
		server.Directives = append(server.Directives, &nginx.NgxDirective{
			Directive: "add_header",
			Params:    fmt.Sprintf(`Strict-Transport-Security "max-age=%d" always`, hstsMaxAge),
		})
	}
	server.Locations = append(server.Locations, locations...)
	return server
}

// buildSiteHeader returns the top-level part of the site file: the managed
// marker, the websocket Connection map and the request limit zone. Site
// files are included inside http {}, so both are valid there.
func buildSiteHeader(ing *model.Ingress) string {
	zone := zoneName(ing.Name)
	var b strings.Builder
	b.WriteString(siteHeaderPrefix + ing.Name + "\n")
	b.WriteString("# Generated from the ingress spec; manual changes are overwritten.\n")
	// Apply reads this line back to delete the upstream files of removed
	// backends by their exact names.
	b.WriteString(upstreamsMarker + strings.Join(upstreamNames(ing), " ") + "\n")
	if usesWebSocket(&ing.Spec) {
		fmt.Fprintf(&b, "map $http_upgrade $%s_connection {\n    default upgrade;\n    '' close;\n}\n", zone)
	}
	if rl := ing.Spec.Policy.RateLimit; rl != nil {
		fmt.Fprintf(&b, "limit_req_zone $binary_remote_addr zone=%s:%s rate=%dr/s;\n", zone, limitReqZoneSize, rl.RPS)
	}
	return strings.TrimRight(b.String(), "\n")
}

// upstreamsMarker starts the site file line listing the generated upstreams.
const upstreamsMarker = "# Upstreams: "

// upstreamNames returns the upstream names of the backends, sorted.
func upstreamNames(ing *model.Ingress) []string {
	names := make([]string, 0, len(ing.Spec.Backends))
	for _, b := range ing.Spec.Backends {
		names = append(names, UpstreamName(ing.Name, b.Name))
	}
	sort.Strings(names)
	return names
}

func usesWebSocket(spec *model.IngressSpec) bool {
	for _, r := range spec.Routes {
		if r.WebSocket {
			return true
		}
	}
	return false
}

// buildRouteLocations renders one location per route, sorted by prefix.
// nginx picks the longest matching prefix regardless of the order.
func buildRouteLocations(ing *model.Ingress) []*nginx.NgxLocation {
	spec := &ing.Spec
	routes := append([]model.IngressRoute(nil), spec.Routes...)
	sort.Slice(routes, func(i, j int) bool { return routes[i].PathPrefix < routes[j].PathPrefix })

	locations := make([]*nginx.NgxLocation, 0, len(routes))
	for _, r := range routes {
		path := "^~ " + r.PathPrefix
		if r.PathPrefix == "/" {
			path = "/"
		}
		locations = append(locations, &nginx.NgxLocation{
			Path:    path,
			Content: buildRouteContent(ing.Name, spec, r).String(),
		})
	}
	return locations
}

func buildRouteContent(name string, spec *model.IngressSpec, r model.IngressRoute) directives {
	var d directives
	policy := &spec.Policy

	cidrs := policy.AllowCIDRs
	if len(r.AllowCIDRs) > 0 {
		cidrs = r.AllowCIDRs
	}
	if len(cidrs) > 0 {
		for _, cidr := range cidrs {
			canonical, _ := parseCIDR(cidr)
			d.add("allow", canonical)
		}
		d.add("deny", "all")
	}

	if rl := policy.RateLimit; rl != nil {
		params := []string{"zone=" + zoneName(name)}
		if rl.Burst > 0 {
			params = append(params, "burst="+strconv.Itoa(rl.Burst), "nodelay")
		}
		d.add("limit_req", params...)
	}

	bodySize := policy.ClientMaxBodySize
	if r.ClientMaxBodySize != "" {
		bodySize = r.ClientMaxBodySize
	}
	if bodySize != "" {
		d.add("client_max_body_size", bodySize)
	}

	d.add("proxy_pass", "http://"+UpstreamName(name, r.Backend))
	d.add("proxy_http_version", "1.1")
	d.add("proxy_set_header", "Host", "$host")
	d.add("proxy_set_header", "X-Real-IP", "$remote_addr")
	d.add("proxy_set_header", "X-Forwarded-For", "$proxy_add_x_forwarded_for")
	d.add("proxy_set_header", "X-Forwarded-Proto", "$scheme")
	if r.WebSocket {
		d.add("proxy_set_header", "Upgrade", "$http_upgrade")
		d.add("proxy_set_header", "Connection", "$"+zoneName(name)+"_connection")
	} else {
		// An empty Connection header lets upstream keepalive reuse connections.
		d.add("proxy_set_header", "Connection", `""`)
	}
	for _, h := range policy.ExtraHeaders {
		// Validate keeps quotes, backslashes and control characters out.
		d.add("proxy_set_header", h.Name, `"`+h.Value+`"`)
	}
	if r.ReadTimeout != "" {
		d.add("proxy_read_timeout", r.ReadTimeout)
	}
	if r.SendTimeout != "" {
		d.add("proxy_send_timeout", r.SendTimeout)
	}
	return d
}
