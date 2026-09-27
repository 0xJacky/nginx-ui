package cert

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

const maintenanceConfigSuffix = "_nginx_ui_maintenance"

// ValidateHTTP01ChallengeConfig verifies the configuration that Nginx has
// enabled for a site before lego contacts the ACME server. This keeps a stale
// editor state or a missing symlink from becoming an opaque CA-side 404.
//
// It is advisory: it cannot follow include files, upstream names or
// hostnames in proxy_pass, so issuance is gated by the active probe in
// http01_probe.go and this function only explains a failed probe.
func ValidateHTTP01ChallengeConfig(configName string, identifiers []string) error {
	if strings.TrimSpace(configName) == "" {
		return NewHTTP01ChallengePreflightError("site configuration name is empty")
	}

	candidates, err := enabledSiteConfigCandidates(configName)
	if err != nil {
		return NewHTTP01ChallengePreflightError("site configuration path is invalid")
	}

	var sawEnabledConfig bool
	var routeError error
	for _, candidate := range candidates {
		exists, existsErr := nginx.Exists(candidate)
		if existsErr != nil {
			return NewHTTP01ChallengePreflightError("enabled site configuration cannot be checked")
		}
		if !exists {
			continue
		}
		sawEnabledConfig = true
		parsed, parseErr := nginx.ParseNgxConfig(candidate)
		if parseErr != nil {
			return NewHTTP01ChallengePreflightError("enabled site configuration cannot be parsed")
		}
		routeError = validateHTTP01ChallengeConfig(parsed, settings.CertSettings.HTTPChallengePort, identifiers)
		if routeError == nil {
			return nil
		}
	}

	if !sawEnabledConfig {
		return NewHTTP01ChallengePreflightError("site is not enabled in Nginx")
	}
	if routeError != nil {
		return routeError
	}
	return NewHTTP01ChallengePreflightError(fmt.Sprintf("active configuration has no HTTP-01 proxy to port %s", settings.CertSettings.HTTPChallengePort))
}

// enabledSiteConfigCandidates returns the sites-enabled paths Nginx may load
// for configName: the regular symlink and its maintenance-mode variant.
func enabledSiteConfigCandidates(configName string) ([]string, error) {
	path, err := config.ResolveConfPathInDirPreserveLeaf("sites-enabled", configName)
	if err != nil {
		return nil, err
	}
	return []string{
		nginx.GetConfSymlinkPath(path),
		nginx.GetConfSymlinkPath(path + maintenanceConfigSuffix),
	}, nil
}

// validateHTTP01ChallengeConfig statically looks for a port-80 server per
// identifier whose acme-challenge location proxies to the challenge port,
// directly or behind a redirect to an HTTPS server that routes it.
// It only understands the layout nginx-ui generates itself (no include
// files, no upstream names, no regex server_name), so callers must treat a
// failure as a possible explanation, never as proof that the route is broken.
func validateHTTP01ChallengeConfig(config *nginx.NgxConfig, challengePort string, identifiers []string) error {
	if config == nil || strings.TrimSpace(challengePort) == "" {
		return NewHTTP01ChallengePreflightError("HTTP-01 challenge port is not configured")
	}

	proxyPattern := regexp.MustCompile(`(?m)\bproxy_pass\s+http://127\.0\.0\.1:` + regexp.QuoteMeta(challengePort) + `(?:/)?\s*;`)
	for _, identifier := range identifiers {
		if reason := diagnoseHTTP01Route(config, proxyPattern, challengePort, identifier); reason != "" {
			return NewHTTP01ChallengePreflightError(fmt.Sprintf("HTTP-01 route is unavailable for %s on port 80: %s", identifier, reason))
		}
	}
	if len(identifiers) > 0 {
		return nil
	}

	return NewHTTP01ChallengePreflightError("active configuration has no HTTP-01 proxy location")
}

var (
	httpsRedirectReturnPattern   = regexp.MustCompile(`^\s*30[1278]\s+"?https://`)
	httpsRedirectLocationPattern = regexp.MustCompile(`(?m)^\s*return\s+30[1278]\s+"?https://`)
)

// diagnoseHTTP01Route returns an empty string when a port-80 server for
// identifier routes the challenge to the challenge port, otherwise the most
// specific reason it can find.
//
// ACME validators follow redirects to HTTPS without checking the certificate,
// so a port-80 server that redirects to HTTPS is fine as long as an HTTPS
// server for the same name routes the challenge location.
func diagnoseHTTP01Route(config *nginx.NgxConfig, proxyPattern *regexp.Regexp, challengePort, identifier string) string {
	var matched []*nginx.NgxServer
	for _, server := range config.Servers {
		if server != nil && serverMatchesIdentifier(server, identifier) {
			matched = append(matched, server)
		}
	}

	httpsRoutesChallenge := false
	for _, server := range matched {
		if hasHTTPSListen(server) && serverReturnParams(server) == "" && routesChallenge(server, proxyPattern) {
			httpsRoutesChallenge = true
		}
	}

	var (
		matchedNonHTTP   bool
		matchedHTTP      bool
		hasPlainReturn   bool
		redirectsToHTTPS bool
		hasInclude       bool
		hasChallengePath bool
	)
	for _, server := range matched {
		if !hasHTTPListen(server) {
			matchedNonHTTP = true
			continue
		}
		matchedHTTP = true
		if params := serverReturnParams(server); params != "" {
			// A server-level return answers before any location is reached.
			if httpsRedirectReturnPattern.MatchString(params) {
				if httpsRoutesChallenge {
					return ""
				}
				redirectsToHTTPS = true
			} else {
				hasPlainReturn = true
			}
			continue
		}
		hasInclude = hasInclude || serverHasDirective(server, "include")
		var hasChallengeLocation bool
		for _, location := range server.Locations {
			if !strings.Contains(location.Path, "acme-challenge") {
				continue
			}
			hasChallengeLocation = true
			hasChallengePath = true
			if proxyPattern.MatchString(location.Content) {
				return ""
			}
		}
		if !hasChallengeLocation && rootLocationRedirectsToHTTPS(server) {
			if httpsRoutesChallenge {
				return ""
			}
			redirectsToHTTPS = true
		}
	}

	switch {
	case !matchedHTTP && matchedNonHTTP:
		return "no server listening on port 80 matches this name, only servers on other ports do"
	case !matchedHTTP:
		return "no port-80 server with this server_name was found in the enabled configuration"
	case hasChallengePath:
		return fmt.Sprintf("the acme-challenge location does not proxy_pass to http://127.0.0.1:%s", challengePort)
	case redirectsToHTTPS:
		return fmt.Sprintf("the port-80 server redirects to HTTPS, but no HTTPS server for this name has an acme-challenge location that proxies to http://127.0.0.1:%s", challengePort)
	case hasPlainReturn && !hasInclude:
		return "the port-80 server has a server-level return, which answers before any location is reached"
	case hasInclude:
		return "the port-80 server has no acme-challenge location (an included file may define one)"
	default:
		return "the port-80 server has no /.well-known/acme-challenge/ location"
	}
}

func serverMatchesIdentifier(server *nginx.NgxServer, identifier string) bool {
	var names []string
	var isDefault bool
	for _, directive := range server.Directives {
		switch directive.Directive {
		case "server_name":
			names = append(names, strings.Fields(directive.Params)...)
		case "listen":
			isDefault = isDefault || strings.Contains(directive.Params, "default_server")
		}
	}
	return containsIdentifier(names, identifier) || (isDefault && net.ParseIP(identifier) != nil)
}

func serverReturnParams(server *nginx.NgxServer) string {
	for _, directive := range server.Directives {
		if directive.Directive == "return" {
			if params := strings.TrimSpace(directive.Params); params != "" {
				return params
			}
			return "return"
		}
	}
	return ""
}

func serverHasDirective(server *nginx.NgxServer, name string) bool {
	for _, directive := range server.Directives {
		if directive.Directive == name {
			return true
		}
	}
	return false
}

func routesChallenge(server *nginx.NgxServer, proxyPattern *regexp.Regexp) bool {
	for _, location := range server.Locations {
		if strings.Contains(location.Path, "acme-challenge") && proxyPattern.MatchString(location.Content) {
			return true
		}
	}
	return false
}

func rootLocationRedirectsToHTTPS(server *nginx.NgxServer) bool {
	for _, location := range server.Locations {
		if strings.TrimSpace(location.Path) == "/" && httpsRedirectLocationPattern.MatchString(location.Content) {
			return true
		}
	}
	return false
}

// explainHTTP01RouteFailure runs the static analyzer for one identifier and
// returns its reason, or an empty string when it found nothing suspicious.
func explainHTTP01RouteFailure(configName, identifier string) string {
	if strings.TrimSpace(configName) == "" {
		return ""
	}
	err := ValidateHTTP01ChallengeConfig(configName, []string{identifier})
	if err == nil {
		return ""
	}
	var cosyErr *cosy.Error
	if errors.As(err, &cosyErr) && len(cosyErr.Params) > 0 {
		return cosyErr.Params[0]
	}
	return err.Error()
}

func containsIdentifier(names []string, identifier string) bool {
	for _, name := range names {
		if strings.EqualFold(name, identifier) {
			return true
		}
	}
	return false
}

func hasHTTPListen(server *nginx.NgxServer) bool {
	if server == nil {
		return false
	}
	for _, directive := range server.Directives {
		if directive.Directive != "listen" {
			continue
		}
		for _, token := range strings.Fields(directive.Params) {
			token = strings.TrimSuffix(token, ";")
			if token == "80" || strings.HasSuffix(token, ":80") {
				return true
			}
		}
	}
	return false
}

func hasHTTPSListen(server *nginx.NgxServer) bool {
	if server == nil {
		return false
	}
	for _, directive := range server.Directives {
		if directive.Directive != "listen" {
			continue
		}
		for _, token := range strings.Fields(directive.Params) {
			token = strings.TrimSuffix(token, ";")
			if token == "443" || token == "ssl" || strings.HasSuffix(token, ":443") {
				return true
			}
		}
	}
	return false
}
