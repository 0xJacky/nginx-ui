package cert

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
)

const maintenanceConfigSuffix = "_nginx_ui_maintenance"

// ValidateHTTP01ChallengeConfig statically checks the effective Nginx
// configuration (nginx -T) for every identifier: the server block Nginx
// selects for it on each port-80 socket must route
// /.well-known/acme-challenge/ to the challenge port, directly or behind a
// redirect to an HTTPS server block that routes it.
//
// It is advisory: it cannot follow upstream names or hostnames in
// proxy_pass, so it only explains a failed active probe (http01_probe.go).
func ValidateHTTP01ChallengeConfig(identifiers []string) error {
	blocks, err := http01ProbeServerBlocks()
	if err != nil || len(blocks) == 0 {
		return NewHTTP01ChallengePreflightError(http01ProbeConfigUnreadable)
	}
	return validateHTTP01ChallengeConfig(blocks, settings.CertSettings.HTTPChallengePort, identifiers)
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

// validateHTTP01ChallengeConfig is the pure part of
// ValidateHTTP01ChallengeConfig: it explains, per identifier, why the server
// blocks would not route the challenge, and returns nil when they all do.
func validateHTTP01ChallengeConfig(blocks []nginx.ServerBlock, challengePort string, identifiers []string) error {
	if strings.TrimSpace(challengePort) == "" {
		return NewHTTP01ChallengePreflightError("HTTP-01 challenge port is not configured")
	}
	if len(identifiers) == 0 {
		return NewHTTP01ChallengePreflightError("no identifiers to check")
	}
	for _, identifier := range identifiers {
		if reason := explainHTTP01RouteBlocks(blocks, challengePort, identifier); reason != "" {
			return NewHTTP01ChallengePreflightError(fmt.Sprintf("HTTP-01 route is unavailable for %s on port 80: %s", identifier, reason))
		}
	}
	return nil
}

var (
	httpsRedirectReturnPattern = regexp.MustCompile(`^\s*30[1278]\s+"?https://`)
)

// decidingServerSockets returns the sockets on port whose selected server
// block decides how host is answered. When any socket selects a block by
// server_name, only those sockets count: a socket where host merely falls
// through to an unrelated default server (for example an IPv6-only default
// next to the domain's IPv4 block) says nothing about the domain's route.
// Only when no socket matches by name do the default-server sockets decide.
func decidingServerSockets(blocks []nginx.ServerBlock, host, port string) []nginx.ServerSocket {
	sockets := nginx.ResolveServerSockets(blocks, host, port)
	var byName []nginx.ServerSocket
	for _, socket := range sockets {
		if socket.ByName {
			byName = append(byName, socket)
		}
	}
	if len(byName) > 0 {
		return byName
	}
	return sockets
}

// explainHTTP01RouteBlocks returns an empty string when, on every deciding
// port-80 socket, the server block Nginx selects for identifier routes the challenge
// to the challenge port. Otherwise it returns the reason for the first socket
// that does not.
//
// ACME validators follow redirects to HTTPS without checking the certificate,
// so a port-80 block that redirects to HTTPS is fine as long as a server
// block Nginx selects for the same name on port 443 routes the challenge.
func explainHTTP01RouteBlocks(blocks []nginx.ServerBlock, challengePort, identifier string) string {
	sockets := decidingServerSockets(blocks, identifier, "80")
	if len(sockets) == 0 {
		return fmt.Sprintf("no server block listens on port 80 for %s", identifier)
	}
	for _, socket := range sockets {
		if reason := explainHTTP01Socket(blocks, challengePort, identifier, socket); reason != "" {
			return reason
		}
	}
	return ""
}

func explainHTTP01Socket(blocks []nginx.ServerBlock, challengePort, identifier string, socket nginx.ServerSocket) string {
	block := socket.Block
	subject := fmt.Sprintf("the server block for %s on %s in %s", identifier, serverListenLabel(socket.Listen), serverBlockFile(block))
	if !socket.ByName {
		subject = fmt.Sprintf("no server_name matches %s on %s, so Nginx answers with the default server block in %s, which",
			identifier, serverListenLabel(socket.Listen), serverBlockFile(block))
	}
	target := net.JoinHostPort("127.0.0.1", challengePort)
	httpsUnrouted := fmt.Sprintf("no HTTPS server block for %s routes /.well-known/acme-challenge/ to %s", identifier, target)

	if block.Return != "" {
		// A server-level return answers before any location is reached.
		if httpsRedirectReturnPattern.MatchString(block.Return) {
			if httpsRoutesHTTP01Challenge(blocks, challengePort, identifier) {
				return ""
			}
			return fmt.Sprintf("%s redirects to HTTPS at server level (return %s), and %s", subject, block.Return, httpsUnrouted)
		}
		return fmt.Sprintf("%s has a server-level return (return %s), which answers before any location is reached", subject, block.Return)
	}

	var challenge *nginx.ServerLocation
	for i := range block.Locations {
		location := block.Locations[i]
		if !isHTTP01ChallengeLocation(location) {
			continue
		}
		if location.Return == "" && proxiesToHTTP01ChallengePort(location.ProxyPass, challengePort) {
			return ""
		}
		if challenge == nil {
			challenge = &block.Locations[i]
		}
	}
	if challenge != nil {
		detail := "no proxy_pass"
		switch {
		case challenge.Return != "":
			detail = "return " + challenge.Return
		case challenge.ProxyPass != "":
			detail = "proxy_pass " + challenge.ProxyPass
		}
		return fmt.Sprintf("%s has a %s location that does not proxy to %s (%s)", subject, challenge.Path, target, detail)
	}

	if rootLocationRedirectsToHTTPS(block) {
		if httpsRoutesHTTP01Challenge(blocks, challengePort, identifier) {
			return ""
		}
		return fmt.Sprintf("%s redirects to HTTPS in location /, and %s", subject, httpsUnrouted)
	}
	return fmt.Sprintf("%s has no /.well-known/acme-challenge location proxying to %s", subject, target)
}

// httpsRoutesHTTP01Challenge reports whether a server block Nginx selects for
// identifier on port 443 routes the challenge location to the challenge port.
func httpsRoutesHTTP01Challenge(blocks []nginx.ServerBlock, challengePort, identifier string) bool {
	for _, socket := range decidingServerSockets(blocks, identifier, "443") {
		if socket.Block.Return != "" {
			continue
		}
		for _, location := range socket.Block.Locations {
			if isHTTP01ChallengeLocation(location) && location.Return == "" &&
				proxiesToHTTP01ChallengePort(location.ProxyPass, challengePort) {
				return true
			}
		}
	}
	return false
}

func isHTTP01ChallengeLocation(location nginx.ServerLocation) bool {
	return strings.Contains(location.Path, "acme-challenge") || strings.Contains(location.Path, "/.well-known")
}

// proxiesToHTTP01ChallengePort reports whether a proxy_pass target is the
// plain-HTTP challenge server on a loopback address.
func proxiesToHTTP01ChallengePort(proxyPass, challengePort string) bool {
	target := strings.TrimSuffix(strings.TrimSpace(proxyPass), ";")
	if !strings.HasPrefix(strings.ToLower(target), "http://") {
		return false
	}
	hostPort := target[len("http://"):]
	if i := strings.IndexByte(hostPort, '/'); i >= 0 {
		hostPort = hostPort[:i]
	}
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil || port != challengePort {
		return false
	}
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

func rootLocationRedirectsToHTTPS(block nginx.ServerBlock) bool {
	for _, location := range block.Locations {
		if strings.TrimSpace(location.Path) == "/" && httpsRedirectReturnPattern.MatchString(location.Return) {
			return true
		}
	}
	return false
}

func serverBlockFile(block nginx.ServerBlock) string {
	if block.File == "" {
		return "the Nginx configuration"
	}
	return block.File
}

func serverListenLabel(listen nginx.ServerListen) string {
	port := listen.Port
	if port == "" {
		port = "80"
	}
	switch {
	case listen.Addr != "":
		return net.JoinHostPort(strings.Trim(listen.Addr, "[]"), port)
	case listen.IPv6:
		return "[::]:" + port
	default:
		return "*:" + port
	}
}
