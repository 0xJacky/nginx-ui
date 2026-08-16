package template

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	internalTemplate "github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
)

const (
	QuickConfigTypeReverseProxy = "reverse_proxy"
	QuickConfigTypeStatic       = "static"
	QuickConfigTypeRedirect     = "redirect"
)

type QuickConfigRequest struct {
	Type                string   `json:"type" binding:"required,oneof=reverse_proxy static redirect"`
	Domains             []string `json:"domains"`
	EnableTLS           bool     `json:"enable_tls"`
	RedirectHTTPToHTTPS bool     `json:"redirect_http_to_https"`

	// Reverse proxy
	Scheme            string `json:"scheme"`
	Host              string `json:"host"`
	Port              string `json:"port"`
	EnableWebSocket   bool   `json:"enable_websocket"`
	ClientMaxBodySize string `json:"client_max_body_size"`

	// Static site
	WebRoot     string `json:"web_root"`
	Index       string `json:"index"`
	SpaFallback bool   `json:"spa_fallback"`

	// Redirect
	TargetURL      string `json:"target_url"`
	RedirectStatus string `json:"redirect_status"`
}

func (r *QuickConfigRequest) fillDefaults() {
	if r.Scheme == "" {
		r.Scheme = "http"
	}
	if r.Host == "" {
		r.Host = "127.0.0.1"
	}
	if r.Port == "" {
		r.Port = "9000"
	}
	if r.ClientMaxBodySize == "" {
		r.ClientMaxBodySize = "1000m"
	}
	if r.Index == "" {
		r.Index = "index.html"
	}
	if r.RedirectStatus == "" {
		r.RedirectStatus = "301"
	}
}

func (r *QuickConfigRequest) validate() error {
	if len(r.Domains) == 0 {
		return errors.New("domains is required")
	}

	for _, domain := range r.Domains {
		if strings.TrimSpace(domain) == "" {
			return errors.New("domain cannot be empty")
		}
		if !isSafeNginxToken(domain) {
			return errors.New("domain contains invalid characters")
		}
	}

	switch r.Type {
	case QuickConfigTypeReverseProxy:
		if r.Scheme != "http" && r.Scheme != "https" {
			return errors.New("scheme must be http or https")
		}
		if !isSafeNginxToken(r.Host) {
			return errors.New("host contains invalid characters")
		}
		port, err := strconv.Atoi(r.Port)
		if err != nil || port < 1 || port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
		if !reNginxSize.MatchString(r.ClientMaxBodySize) {
			return errors.New("client_max_body_size must be a non-negative integer with an optional k, m, or g suffix")
		}
	case QuickConfigTypeStatic:
		if strings.TrimSpace(r.WebRoot) == "" {
			return errors.New("web_root is required")
		}
		if !isSafeNginxToken(r.WebRoot) {
			return errors.New("web_root contains invalid characters")
		}
		if !isSafeNginxValue(r.Index) {
			return errors.New("index contains invalid characters")
		}
	case QuickConfigTypeRedirect:
		if strings.TrimSpace(r.TargetURL) == "" {
			return errors.New("target_url is required")
		}
		if !isSafeNginxToken(r.TargetURL) {
			return errors.New("target_url contains invalid characters")
		}
		target, err := url.ParseRequestURI(r.TargetURL)
		if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
			return errors.New("target_url must be an absolute HTTP or HTTPS URL")
		}
		if r.RedirectStatus != "301" && r.RedirectStatus != "302" && r.RedirectStatus != "308" {
			return errors.New("redirect_status must be 301, 302, or 308")
		}
	}

	return nil
}

var (
	reUnsafeNginxValue = regexp.MustCompile(`[;{}\r\n]`)
	reNginxSize        = regexp.MustCompile(`^\d+[kKmMgG]?$`)
)

func isSafeNginxValue(value string) bool {
	return value == strings.TrimSpace(value) && value != "" && !reUnsafeNginxValue.MatchString(value)
}

func isSafeNginxToken(value string) bool {
	return isSafeNginxValue(value) && !strings.ContainsAny(value, " \t")
}

// quickApp holds the per-type server-level content of a quick config.
type quickApp struct {
	directives []*nginx.NgxDirective
	locations  []*nginx.NgxLocation
	custom     string
}

func buildQuickApp(r *QuickConfigRequest) (app *quickApp, err error) {
	app = &quickApp{}

	switch r.Type {
	case QuickConfigTypeReverseProxy:
		block, err := internalTemplate.ParseTemplate("block", "reverse-proxy.conf", map[string]internalTemplate.Variable{
			"enableWebSocket":   {Value: r.EnableWebSocket},
			"clientMaxBodySize": {Value: r.ClientMaxBodySize},
			"scheme":            {Value: r.Scheme},
			"host":              {Value: r.Host},
			"port":              {Value: r.Port},
		})
		if err != nil {
			return nil, err
		}
		app.directives = block.Directives
		app.locations = block.Locations
		app.custom = block.Custom
	case QuickConfigTypeStatic:
		app.directives = append(app.directives,
			&nginx.NgxDirective{Directive: "root", Params: r.WebRoot},
			&nginx.NgxDirective{Directive: "index", Params: r.Index},
		)
		if r.SpaFallback {
			block, err := internalTemplate.ParseTemplate("block", "vue-router-history-mode.conf", nil)
			if err != nil {
				return nil, err
			}
			app.locations = block.Locations
		}
	case QuickConfigTypeRedirect:
		block, err := internalTemplate.ParseTemplate("block", "redirect.conf", map[string]internalTemplate.Variable{
			"status": {Value: r.RedirectStatus},
			"target": {Value: r.TargetURL},
		})
		if err != nil {
			return nil, err
		}
		app.locations = block.Locations
	}

	return app, nil
}

// letsEncryptLocation returns the HTTP challenge location block parsed from the
// letsencrypt.conf template.
func letsEncryptLocation() (*nginx.NgxLocation, error) {
	block, err := internalTemplate.ParseTemplate("block", "letsencrypt.conf", nil)
	if err != nil {
		return nil, err
	}
	if len(block.Locations) == 0 {
		return nil, errors.New("letsencrypt.conf contains no location")
	}
	return block.Locations[0], nil
}

func buildQuickConfig(r *QuickConfigRequest) (ngxConfig *nginx.NgxConfig, err error) {
	app, err := buildQuickApp(r)
	if err != nil {
		return nil, err
	}

	serverName := strings.Join(r.Domains, " ")

	port80 := nginx.NewNgxServer()
	port80.Directives = []*nginx.NgxDirective{
		{Directive: "listen", Params: "80"},
		{Directive: "listen", Params: "[::]:80"},
		{Directive: "server_name", Params: serverName},
	}

	var tlsServer *nginx.NgxServer
	if r.EnableTLS {
		tlsServer = nginx.NewNgxServer()
		tlsServer.Directives = []*nginx.NgxDirective{
			{Directive: "listen", Params: "443 ssl"},
			{Directive: "listen", Params: "[::]:443 ssl"},
			{Directive: "server_name", Params: serverName},
			{Directive: "ssl_certificate"},
			{Directive: "ssl_certificate_key"},
		}
		tlsServer.Directives = append(tlsServer.Directives, app.directives...)
		tlsServer.Locations = append(tlsServer.Locations, app.locations...)

		challengeLocation, err := letsEncryptLocation()
		if err != nil {
			return nil, err
		}
		tlsServer.Locations = append(tlsServer.Locations, challengeLocation)

		if r.RedirectHTTPToHTTPS {
			port80.Directives = append(port80.Directives,
				&nginx.NgxDirective{Directive: "return", Params: "301 https://$host$request_uri"})
			port80.Locations = append(port80.Locations, challengeLocation)
		} else {
			port80.Directives = append(port80.Directives, app.directives...)
			port80.Locations = append(port80.Locations, app.locations...)
			port80.Locations = append(port80.Locations, challengeLocation)
		}
	} else {
		port80.Directives = append(port80.Directives, app.directives...)
		port80.Locations = append(port80.Locations, app.locations...)
	}

	servers := []*nginx.NgxServer{port80}
	if tlsServer != nil {
		servers = append(servers, tlsServer)
	}

	return &nginx.NgxConfig{
		Name:      r.Domains[0],
		Custom:    app.custom,
		Upstreams: make([]*nginx.NgxUpstream, 0),
		Servers:   servers,
	}, nil
}

func GetQuickConfig(c *gin.Context) {
	var req QuickConfigRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	req.fillDefaults()

	if err := req.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ngxConfig, err := buildQuickConfig(&req)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	content, err := ngxConfig.BuildConfig()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "ok",
		"template":  content,
		"tokenized": ngxConfig,
	})
}

var (
	reProxyPass      = regexp.MustCompile(`proxy_pass\s+(\S+)`)
	reReturn         = regexp.MustCompile(`return\s+(\d{3})\s+(\S+)`)
	reRoot           = regexp.MustCompile(`root\s+(\S+)`)
	reIndex          = regexp.MustCompile(`index\s+(\S+)`)
	reClientBodySize = regexp.MustCompile(`client_max_body_size\s+(\S+)`)
	reTryFilesSpa    = regexp.MustCompile(`try_files\s+.*\s+/index\.html`)
	reProxyPassURL   = regexp.MustCompile(`^(https?)://([^:/]+)(?::(\d+))?`)
)

func findDirective(directives []*nginx.NgxDirective, name string) string {
	for _, d := range directives {
		if d.Directive == name {
			return d.Params
		}
	}
	return ""
}

func cleanParam(raw string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ";"))
}

func serverListensSSL(directives []*nginx.NgxDirective) bool {
	for _, d := range directives {
		if d.Directive == "listen" && strings.Contains(d.Params, "ssl") {
			return true
		}
	}
	return false
}

func parseProxyPass(raw string) (scheme, host, port string) {
	m := reProxyPassURL.FindStringSubmatch(raw)
	if m == nil {
		return
	}
	return m[1], m[2], m[3]
}

// analyzeNgxConfig best-effort derives a QuickConfigRequest from an existing
// nginx config so the quick-setup wizard can pre-fill the edit form. Configs
// that do not match the quick-setup shapes fall back to safe defaults.
func analyzeNgxConfig(ngxConfig *nginx.NgxConfig) (req QuickConfigRequest) {
	req.Type = QuickConfigTypeReverseProxy

	var domains []string
	seenDomains := make(map[string]struct{})
	var hasTLS, redirectHTTPToHTTPS bool

	var rpProxyPass, rpHasWebSocket bool
	var staticRoot, staticIndex string
	var staticSpa bool
	var bodySize string
	var redirectStatus, redirectTarget string

	for _, server := range ngxConfig.Servers {
		if server == nil {
			continue
		}

		if params := findDirective(server.Directives, "server_name"); params != "" {
			for _, name := range strings.Fields(params) {
				if name == "_" {
					continue
				}
				if _, ok := seenDomains[name]; ok {
					continue
				}
				seenDomains[name] = struct{}{}
				domains = append(domains, name)
			}
		}

		if serverListensSSL(server.Directives) {
			hasTLS = true
		}

		if params := findDirective(server.Directives, "return"); params != "" &&
			strings.Contains(params, "https://$host$request_uri") {
			redirectHTTPToHTTPS = true
		}

		if staticRoot == "" {
			staticRoot = cleanParam(findDirective(server.Directives, "root"))
		}
		if staticIndex == "" {
			staticIndex = cleanParam(findDirective(server.Directives, "index"))
		}
		if bodySize == "" {
			bodySize = cleanParam(findDirective(server.Directives, "client_max_body_size"))
		}

		for _, location := range server.Locations {
			// The ACME HTTP-01 challenge location proxies to the local
			// challenge port; ignore it when detecting the quick-setup type.
			if strings.Contains(location.Path, "acme-challenge") {
				continue
			}
			content := location.Content

			if m := reReturn.FindStringSubmatch(content); m != nil && m[2] != "https://$host$request_uri" {
				redirectStatus = cleanParam(m[1])
				redirectTarget = cleanParam(m[2])
			}

			if !rpProxyPass {
				if m := reProxyPass.FindStringSubmatch(content); m != nil {
					rpProxyPass = true
					req.Scheme, req.Host, req.Port = parseProxyPass(cleanParam(m[1]))
				}
			}

			if strings.Contains(content, "proxy_http_version 1.1") ||
				strings.Contains(content, "proxy_set_header Upgrade") {
				rpHasWebSocket = true
			}

			if staticRoot == "" {
				if m := reRoot.FindStringSubmatch(content); m != nil {
					staticRoot = cleanParam(m[1])
				}
			}
			if staticIndex == "" {
				if m := reIndex.FindStringSubmatch(content); m != nil {
					staticIndex = cleanParam(m[1])
				}
			}
			if reTryFilesSpa.MatchString(content) {
				staticSpa = true
			}
			if bodySize == "" {
				if m := reClientBodySize.FindStringSubmatch(content); m != nil {
					bodySize = cleanParam(m[1])
				}
			}
		}
	}

	req.Domains = domains
	req.EnableTLS = hasTLS
	req.RedirectHTTPToHTTPS = redirectHTTPToHTTPS

	switch {
	case redirectStatus != "" && redirectTarget != "":
		req.Type = QuickConfigTypeRedirect
		req.RedirectStatus = redirectStatus
		req.TargetURL = redirectTarget
	case rpProxyPass:
		req.Type = QuickConfigTypeReverseProxy
		req.EnableWebSocket = rpHasWebSocket
	default:
		if staticRoot != "" {
			req.Type = QuickConfigTypeStatic
			req.WebRoot = staticRoot
			req.Index = staticIndex
			req.SpaFallback = staticSpa
		}
	}

	if bodySize != "" {
		req.ClientMaxBodySize = bodySize
	}

	req.fillDefaults()
	return req
}

func AnalyzeQuickConfig(c *gin.Context) {
	var json struct {
		Config string `json:"config" binding:"required"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	ngxConfig, err := nginx.ParseNgxConfigByContent(json.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	req := analyzeNgxConfig(ngxConfig)

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"request": req,
	})
}
