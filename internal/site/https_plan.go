package site

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	internalTemplate "github.com/0xJacky/Nginx-UI/internal/template"
)

// Challenge methods accepted by the HTTPS onboarding planner.
const (
	HTTPSChallengeHTTP01 = "http01"
	HTTPSChallengeDNS01  = "dns01"
)

// Diagnostic codes produced while planning.
const (
	HTTPSDiagnosticRedirectWithoutApp  = "redirect_without_app"
	HTTPSDiagnosticServerNamesExtended = "server_names_extended"
	// HTTPSDiagnosticNamesNotInCertificate lists server names the certificate
	// does not cover.
	HTTPSDiagnosticNamesNotInCertificate = "names_not_in_certificate"
	// HTTPSDiagnosticRedirectNotAppliedUncoveredNames reports a port-80 server
	// left serving plain HTTP because some of its names are not covered by the
	// certificate.
	HTTPSDiagnosticRedirectNotAppliedUncoveredNames = "redirect_not_applied_uncovered_names"
	// HTTPSDiagnosticRedirectNotAppliedCombinedServer reports a server that
	// listens on HTTP and HTTPS in one block and is not restructured.
	HTTPSDiagnosticRedirectNotAppliedCombinedServer = "redirect_not_applied_combined_server"
)

// Hint codes of the planning errors that ask the user to change the
// configuration first.
const (
	HTTPSHintCertificateFromInclude    = "certificate_from_include"
	HTTPSHintDomainsNotInServerName    = "domains_not_in_server_name"
	HTTPSHintDomainsNotOnStandardPorts = "domains_not_on_standard_ports"
)

const (
	httpsMessageNamesNotInCertificate         = "Some server names are not covered by the certificate and were removed from the HTTPS server"
	httpsMessageNamesNotInCertificateCombined = "Some server names are not covered by the certificate; the server also serves plain HTTP so they were kept, and HTTPS requests for them will show a certificate error"
	httpsMessageRedirectUncoveredNames        = "The HTTP to HTTPS redirect was not applied to this server because some of its names are not covered by the certificate"
	httpsMessageRedirectCombinedServer        = "The HTTP to HTTPS redirect was not applied because the server listens on HTTP and HTTPS in one block; add an if ($scheme != \"https\") redirect to it manually"
	httpsMessageCertificateFromInclude        = "The HTTPS server loads its certificate from an include file; issue the certificate from the certificate page and point the include at the new certificate files"
	httpsMessageDomainsNotInServerName        = "None of the domains is in a server_name of this site; add the domains to server_name first"
	httpsMessageDomainsNotOnStandardPorts     = "The servers matching these domains listen neither on port 80 nor with ssl; add a server listening on port 80 for them first"
)

const acmeChallengePath = "/.well-known/acme-challenge"

var (
	// ErrHTTPSNoDomains is returned when the plan request carries no domain.
	ErrHTTPSNoDomains = errors.New("at least one domain is required")
	// ErrHTTPSInvalidDomain is returned for a domain that cannot be written into
	// a server_name directive safely.
	ErrHTTPSInvalidDomain = errors.New("domain contains invalid characters")
	// ErrHTTPSInvalidChallenge is returned for an unknown challenge method.
	ErrHTTPSInvalidChallenge = errors.New("challenge method must be http01 or dns01")
	// ErrHTTPSStreamConfig is returned for a stream {} site, which cannot serve HTTPS.
	ErrHTTPSStreamConfig = errors.New("stream configurations cannot be onboarded to HTTPS")
	// ErrHTTPSMissingCertificatePaths is returned by BuildFinal without paths.
	ErrHTTPSMissingCertificatePaths = errors.New("certificate and key paths are required")
	// ErrHTTPSCertificateFromInclude is returned for a TLS server whose
	// certificate may come from an include file the planner cannot see.
	ErrHTTPSCertificateFromInclude = errors.New("the TLS server gets its certificate from an include file")
	// ErrHTTPSDomainsNotInServerName is returned when no server of the site
	// serves any of the requested domains.
	ErrHTTPSDomainsNotInServerName = errors.New("none of the domains is in a server_name of the site")
	// ErrHTTPSDomainsNotOnStandardPorts is returned when the servers covering
	// the domains listen neither on port 80 nor with ssl.
	ErrHTTPSDomainsNotOnStandardPorts = errors.New("no server for the domains listens on port 80 or with ssl")
)

// HTTPSPlanRequest describes the HTTPS configuration a site should end up with.
type HTTPSPlanRequest struct {
	Domains             []string
	ChallengeMethod     string
	RedirectHTTPToHTTPS bool
	// SiteDisabled must be set when the site is not enabled. Only then is a TLS
	// server without ssl_certificate directives treated as pending (not live).
	// On an enabled site the live configuration passed nginx -t, so such a
	// server gets its certificate from the http{} block and keeps serving; the
	// zero value therefore never drops a TLS server that may be live.
	SiteDisabled bool
	// ChallengeLocation is the HTTP-01 location installed on the servers. When
	// nil the letsencrypt.conf block template is used.
	ChallengeLocation *nginx.NgxLocation
}

// HTTPSDiagnostic is a non-fatal observation, shared with the DNS diagnostics
// of the acmehint package so both reach the client as `diagnostic` events.
type HTTPSDiagnostic = acmehint.Diagnostic

// HTTPSPlan is the result of PlanHTTPS. Staged is the configuration that is
// applied before the certificate exists; BuildFinal produces the configuration
// applied once the certificate has been issued.
type HTTPSPlan struct {
	// Request is the normalized request the plan was built from.
	Request HTTPSPlanRequest
	// Staged serves plain HTTP (plus the challenge route for HTTP-01). It never
	// references a missing certificate and never redirects to an HTTPS server
	// that cannot answer yet.
	Staged *nginx.NgxConfig
	// PendingTLSServers counts the TLS servers left out of Staged because they
	// have no certificate yet.
	PendingTLSServers int
	// HasCertifiedTLS reports whether a TLS server with a certificate already
	// covers the requested names (a reissue).
	HasCertifiedTLS bool
	// AddedHTTPServer reports whether a port-80 server was added for HTTP-01.
	AddedHTTPServer bool
	// Diagnostics lists non-fatal observations.
	Diagnostics []HTTPSDiagnostic

	slots     []*httpsSlot
	challenge *nginx.NgxLocation
	missing   []string
	// siteNames are the literal names served by the site plus the requested
	// domains; a redirect to one of them targets this site.
	siteNames []string
}

type httpsSlotKind int

const (
	// httpsSlotOther is a server that does not cover the requested names on
	// port 80 or with TLS.
	httpsSlotOther httpsSlotKind = iota
	// httpsSlotHTTP is a plain port-80 server covering the requested names.
	httpsSlotHTTP
	// httpsSlotPendingTLS is a TLS server without certificate.
	httpsSlotPendingTLS
	// httpsSlotCertifiedTLS is a TLS server that already has a certificate.
	httpsSlotCertifiedTLS
	// httpsSlotPendingMixed listens on 80 and on an ssl port in one block and
	// has no certificate yet. Its plain HTTP half is staged on its own.
	httpsSlotPendingMixed
	// httpsSlotCertifiedMixed listens on 80 and on an ssl port in one block and
	// already has a certificate. It is the HTTP slot of its names as well.
	httpsSlotCertifiedMixed
)

type httpsSlot struct {
	kind     httpsSlotKind
	original *nginx.NgxServer
	// staged is the server as it appears in Staged, nil when left out.
	staged *nginx.NgxServer
	// redirect reports whether BuildFinal turns the port-80 part of the slot
	// into the HTTP->HTTPS redirect (splitting a pending combined server).
	redirect bool
	// tlsNames is the server_name of the TLS server built from the slot in
	// the final configuration: only names the certificate covers.
	tlsNames []string
}

// PlanHTTPS computes the staged and final configuration for enabling HTTPS on
// the given names. It does not touch the filesystem; cfg is not modified.
//
// Configurations whose shape cannot be handled without risking a working
// HTTPS server are refused with an *HTTPSHintError telling the user what to
// change first.
func PlanHTTPS(cfg *nginx.NgxConfig, req HTTPSPlanRequest) (*HTTPSPlan, error) {
	if cfg == nil {
		return nil, errors.New("nginx configuration is nil")
	}
	if cfg.RootBlock == nginx.Stream {
		return nil, ErrHTTPSStreamConfig
	}

	domains, err := normalizeHTTPSDomains(req.Domains)
	if err != nil {
		return nil, err
	}
	req.Domains = domains

	req.ChallengeMethod = strings.ToLower(strings.TrimSpace(req.ChallengeMethod))
	if req.ChallengeMethod == "" {
		req.ChallengeMethod = HTTPSChallengeHTTP01
	}
	if req.ChallengeMethod != HTTPSChallengeHTTP01 && req.ChallengeMethod != HTTPSChallengeDNS01 {
		return nil, ErrHTTPSInvalidChallenge
	}

	plan := &HTTPSPlan{Request: req}
	if req.ChallengeMethod == HTTPSChallengeHTTP01 {
		challenge := req.ChallengeLocation
		if challenge == nil {
			challenge, err = letsEncryptChallengeLocation()
			if err != nil {
				return nil, err
			}
		}
		plan.challenge = cloneLocation(challenge)
		plan.Request.ChallengeLocation = cloneLocation(challenge)
	}

	if err := plan.classify(cfg); err != nil {
		return nil, err
	}

	pendingSource := plan.firstPendingTLS()

	hasHTTPSlot, hasTLSSlot := false, false
	for _, slot := range plan.slots {
		switch slot.kind {
		case httpsSlotOther:
			slot.staged = cloneServer(slot.original)
		case httpsSlotCertifiedTLS:
			hasTLSSlot = true
			slot.staged = plan.stageCertifiedServer(slot.original, false)
		case httpsSlotCertifiedMixed:
			hasTLSSlot, hasHTTPSlot = true, true
			slot.staged = plan.stageCertifiedServer(slot.original, true)
		case httpsSlotPendingTLS:
			hasTLSSlot = true
			slot.staged = nil
		case httpsSlotHTTP:
			hasHTTPSlot = true
			slot.staged = plan.stageHTTPServer(slot.original, pendingSource)
		case httpsSlotPendingMixed:
			hasTLSSlot, hasHTTPSlot = true, true
			slot.staged = plan.stageHTTPServer(plainHTTPPart(slot.original), nil)
		}
	}

	if !hasHTTPSlot && !hasTLSSlot {
		// Only servers on other ports cover the names: a new port-80 or TLS
		// server would take the names over from whatever serves them now.
		return nil, planHintError(ErrHTTPSDomainsNotOnStandardPorts, HTTPSHintDomainsNotOnStandardPorts,
			httpsMessageDomainsNotOnStandardPorts, map[string]string{"domains": strings.Join(domains, " ")})
	}

	if !hasHTTPSlot && plan.challenge != nil {
		plan.addHTTPServer(pendingSource)
	}

	if len(plan.missing) > 0 {
		if slot := plan.firstSlot(httpsSlotHTTP, httpsSlotPendingMixed, httpsSlotCertifiedMixed); slot != nil {
			appendServerNames(slot.staged, plan.missing)
		}
		plan.Diagnostics = append(plan.Diagnostics, HTTPSDiagnostic{
			Level:   acmehint.LevelWarning,
			Code:    HTTPSDiagnosticServerNamesExtended,
			Message: "Some requested domains were not in any server_name and have been added",
			Params:  map[string]string{"domains": strings.Join(plan.missing, " ")},
		})
	}

	plan.decideFinal()

	plan.Staged = cloneConfigShell(cfg)
	for _, slot := range plan.slots {
		if slot.staged != nil {
			plan.Staged.Servers = append(plan.Staged.Servers, cloneServer(slot.staged))
		}
	}

	return plan, nil
}

// classify sorts every server of cfg into a slot and refuses the shapes that
// cannot be onboarded safely.
func (p *HTTPSPlan) classify(cfg *nginx.NgxConfig) error {
	domains := p.Request.Domains
	covered := make(map[string]bool, len(domains))
	for _, server := range cfg.Servers {
		if server == nil {
			continue
		}
		p.siteNames = appendLiteralNames(p.siteNames, serverNameList(server))

		slot := &httpsSlot{original: cloneServer(server)}
		if serverCoversDomains(server, domains) {
			markCoveredDomains(server, domains, covered)
			tls, plain80 := serverListenKinds(server)
			if tls {
				certified := serverHasCertificate(server)
				if !certified && len(directiveNamed(server, "include")) > 0 {
					// The certificate may well be in the include file; replacing
					// or dropping it blindly could break a working server.
					return planHintError(ErrHTTPSCertificateFromInclude, HTTPSHintCertificateFromInclude,
						httpsMessageCertificateFromInclude, map[string]string{"server_name": serverNames(server)})
				}
				if !certified && !p.Request.SiteDisabled {
					// The site is live and passed nginx -t: the certificate comes
					// from the http{} block.
					certified = true
				}
				switch {
				case certified && plain80:
					slot.kind = httpsSlotCertifiedMixed
				case certified:
					slot.kind = httpsSlotCertifiedTLS
				case plain80:
					slot.kind = httpsSlotPendingMixed
				default:
					slot.kind = httpsSlotPendingTLS
				}
				if certified {
					p.HasCertifiedTLS = true
				} else {
					p.PendingTLSServers++
				}
			} else if plain80 {
				slot.kind = httpsSlotHTTP
			}
		}
		p.slots = append(p.slots, slot)
	}

	for _, domain := range domains {
		if !covered[domain] {
			p.missing = append(p.missing, domain)
		}
	}
	if len(p.missing) == len(domains) {
		// A new server for names nobody serves here would take them over from
		// the server that answers them today (the default server, another
		// site), so the user has to decide where they belong.
		return planHintError(ErrHTTPSDomainsNotInServerName, HTTPSHintDomainsNotInServerName,
			httpsMessageDomainsNotInServerName, map[string]string{"domains": strings.Join(domains, " ")})
	}
	p.siteNames = appendLiteralNames(p.siteNames, domains)
	return nil
}

func planHintError(err error, code, message string, params map[string]string) error {
	return &HTTPSHintError{
		Hint: HTTPSHint{Code: code, Message: message, Params: params},
		Err:  err,
	}
}

// addHTTPServer adds the port-80 server HTTP-01 needs when only TLS servers
// cover the names. It never serves the challenge alone: it redirects to the
// working TLS server, or serves the pending TLS server's application.
func (p *HTTPSPlan) addHTTPServer(pendingSource *nginx.NgxServer) {
	server := newPlainHTTPServer(p.Request.Domains)
	switch {
	case p.HasCertifiedTLS:
		server.Locations = append(server.Locations, newRootRedirectLocation())
	case pendingSource != nil:
		directives, locations := p.appContent(pendingSource)
		server.Directives = append(server.Directives, directives...)
		server.Locations = append(server.Locations, locations...)
	}
	ensureChallengeLocation(server, p.challenge)
	p.slots = append(p.slots, &httpsSlot{kind: httpsSlotHTTP, original: cloneServer(server), staged: server})
	p.AddedHTTPServer = true
}

// decideFinal fixes, per slot, the server names of the final TLS server and
// whether the HTTP->HTTPS redirect applies, and reports what is left out.
func (p *HTTPSPlan) decideFinal() {
	hasTLSSource := p.HasCertifiedTLS || p.PendingTLSServers > 0
	redirect := p.Request.RedirectHTTPToHTTPS
	generated := map[string]bool{}

	for _, slot := range p.slots {
		switch slot.kind {
		case httpsSlotPendingTLS, httpsSlotCertifiedTLS:
			slot.tlsNames = p.tlsServerNames(slot.original, false)

		case httpsSlotPendingMixed:
			names := p.finalServerNames(slot.original)
			uncovered := p.uncoveredNames(names)
			switch {
			case redirect && len(uncovered) == 0:
				// Split into a port-80 redirect and a TLS server.
				slot.redirect = true
				slot.tlsNames = p.tlsServerNames(slot.original, false)
			case redirect:
				p.redirectNotApplied(slot.original, uncovered)
				p.tlsServerNames(slot.original, true)
			default:
				p.tlsServerNames(slot.original, true)
			}

		case httpsSlotCertifiedMixed:
			p.tlsServerNames(slot.original, true)
			if redirect && !hasSchemeGuardedHTTPSRedirect(slot.staged) {
				p.Diagnostics = append(p.Diagnostics, HTTPSDiagnostic{
					Level:   acmehint.LevelWarning,
					Code:    HTTPSDiagnosticRedirectNotAppliedCombinedServer,
					Message: httpsMessageRedirectCombinedServer,
					Params:  map[string]string{"server_name": serverNames(slot.original)},
				})
			}

		case httpsSlotHTTP:
			if !hasTLSSource {
				// The TLS server is generated from this server. A name already
				// given to a generated TLS server would only conflict.
				names := p.tlsServerNames(slot.staged, false)
				kept := names[:0:0]
				for _, name := range names {
					if !generated[strings.ToLower(name)] {
						generated[strings.ToLower(name)] = true
						kept = append(kept, name)
					}
				}
				slot.tlsNames = kept
			}
			if !redirect {
				continue
			}
			uncovered := p.uncoveredNames(serverNameList(slot.staged))
			if len(uncovered) == 0 {
				slot.redirect = true
			} else if !serverRedirectsOnlyToHTTPS(slot.staged, anyHTTPSTarget) {
				p.redirectNotApplied(slot.staged, uncovered)
			}
		}
	}
}

func (p *HTTPSPlan) redirectNotApplied(server *nginx.NgxServer, uncovered []string) {
	p.Diagnostics = append(p.Diagnostics, HTTPSDiagnostic{
		Level:   acmehint.LevelWarning,
		Code:    HTTPSDiagnosticRedirectNotAppliedUncoveredNames,
		Message: httpsMessageRedirectUncoveredNames,
		Params: map[string]string{
			"server_name": serverNames(server),
			"names":       strings.Join(uncovered, " "),
		},
	})
}

// finalServerNames returns the names a TLS server built from server carries
// before certificate filtering: its own names plus the requested domains no
// server covered.
func (p *HTTPSPlan) finalServerNames(server *nginx.NgxServer) []string {
	names := serverNameList(server)
	for _, name := range p.missing {
		if !containsFold(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// tlsServerNames returns the certificate-covered server names of the TLS
// server built from server, reporting the names it has to leave out. When
// keepAll is set the server also serves plain HTTP and keeps every name; the
// uncovered ones are only reported.
func (p *HTTPSPlan) tlsServerNames(server *nginx.NgxServer, keepAll bool) []string {
	names := p.finalServerNames(server)
	uncovered := p.uncoveredNames(names)
	if len(uncovered) > 0 {
		message := httpsMessageNamesNotInCertificate
		if keepAll {
			message = httpsMessageNamesNotInCertificateCombined
		}
		p.Diagnostics = append(p.Diagnostics, HTTPSDiagnostic{
			Level:   acmehint.LevelWarning,
			Code:    HTTPSDiagnosticNamesNotInCertificate,
			Message: message,
			Params: map[string]string{
				"server_name": serverNames(server),
				"names":       strings.Join(uncovered, " "),
			},
		})
	}
	if keepAll {
		return names
	}

	var kept []string
	for _, name := range names {
		if nameCoveredByCertificate(name, p.Request.Domains) && !containsFold(kept, name) {
			kept = append(kept, normalizeServerName(name))
		}
	}
	// A requested domain reached only through a dropped entry (a regex, say)
	// is listed literally so the TLS server still answers it.
	for _, domain := range p.Request.Domains {
		if containsFold(kept, domain) {
			continue
		}
		for _, name := range names {
			if serverNameMatches(name, domain) {
				kept = append(kept, domain)
				break
			}
		}
	}
	return kept
}

func (p *HTTPSPlan) uncoveredNames(names []string) []string {
	var uncovered []string
	for _, name := range names {
		if !nameCoveredByCertificate(name, p.Request.Domains) && !containsFold(uncovered, name) {
			uncovered = append(uncovered, name)
		}
	}
	return uncovered
}

// BuildFinal returns the configuration to apply once the certificate has been
// issued to certPath/keyPath.
func (p *HTTPSPlan) BuildFinal(certPath, keyPath string) (*nginx.NgxConfig, error) {
	certPath = strings.TrimSpace(certPath)
	keyPath = strings.TrimSpace(keyPath)
	if certPath == "" || keyPath == "" {
		return nil, ErrHTTPSMissingCertificatePaths
	}
	if strings.ContainsAny(certPath+keyPath, ";{}\r\n") {
		return nil, errors.New("certificate paths contain invalid characters")
	}

	final := cloneConfigShell(p.Staged)
	hasTLSSource := p.HasCertifiedTLS || p.PendingTLSServers > 0
	hasHTTPSlot := false

	for _, slot := range p.slots {
		switch slot.kind {
		case httpsSlotOther:
			final.Servers = append(final.Servers, cloneServer(slot.staged))
		case httpsSlotPendingTLS:
			final.Servers = append(final.Servers, p.finalizeTLSServer(cloneServer(slot.original), slot.tlsNames, certPath, keyPath))
		case httpsSlotCertifiedTLS:
			final.Servers = append(final.Servers, p.finalizeTLSServer(cloneServer(slot.staged), slot.tlsNames, certPath, keyPath))
		case httpsSlotPendingMixed:
			hasHTTPSlot = true
			if slot.redirect {
				httpPart, tlsPart := splitCombinedServer(slot.original)
				final.Servers = append(final.Servers,
					p.redirectServer(httpPart),
					p.finalizeTLSServer(tlsPart, slot.tlsNames, certPath, keyPath))
				continue
			}
			final.Servers = append(final.Servers, p.finalizeTLSServer(cloneServer(slot.original), nil, certPath, keyPath))
		case httpsSlotCertifiedMixed:
			hasHTTPSlot = true
			final.Servers = append(final.Servers, p.finalizeTLSServer(cloneServer(slot.staged), nil, certPath, keyPath))
		case httpsSlotHTTP:
			hasHTTPSlot = true
			staged := cloneServer(slot.staged)
			if slot.redirect {
				final.Servers = append(final.Servers, p.redirectServer(staged))
			} else {
				final.Servers = append(final.Servers, staged)
			}
			if !hasTLSSource && len(slot.tlsNames) > 0 {
				final.Servers = append(final.Servers, p.generateTLSServer(slot.staged, slot.tlsNames, certPath, keyPath))
			}
		}
	}

	if p.Request.RedirectHTTPToHTTPS && !hasHTTPSlot {
		// DNS-01 on a site whose names are only served by TLS servers.
		final.Servers = append(final.Servers, p.redirectServer(newPlainHTTPServer(p.Request.Domains)))
	}

	return final, nil
}

// StagedListensIPv6 reports whether a port-80 server of Staged covering the
// requested names listens on IPv6.
func (p *HTTPSPlan) StagedListensIPv6() bool {
	for _, slot := range p.slots {
		switch slot.kind {
		case httpsSlotHTTP, httpsSlotPendingMixed, httpsSlotCertifiedMixed:
		default:
			continue
		}
		if slot.staged == nil {
			continue
		}
		for _, directive := range slot.staged.Directives {
			if directive.Directive != "listen" {
				continue
			}
			fields := strings.Fields(trimParams(directive.Params))
			if len(fields) > 0 && strings.HasPrefix(fields[0], "[") && !listenHasSSL(fields) {
				return true
			}
		}
	}
	return false
}

// RoutesHTTP01Challenge reports whether cfg already routes the HTTP-01
// challenge of every domain: a plain port-80 server covers the name, carries
// the challenge location and has no server-level return in front of it.
func RoutesHTTP01Challenge(cfg *nginx.NgxConfig, domains []string) bool {
	if cfg == nil || len(domains) == 0 {
		return false
	}
	for _, domain := range domains {
		routed := false
		for _, server := range cfg.Servers {
			if server == nil || !serverCoversDomains(server, []string{domain}) {
				continue
			}
			if _, plain80 := serverListenKinds(server); !plain80 {
				continue
			}
			if len(directiveNamed(server, "return")) > 0 {
				continue
			}
			for _, location := range server.Locations {
				if isChallengeLocation(location) {
					routed = true
				}
			}
		}
		if !routed {
			return false
		}
	}
	return true
}

func directiveNamed(server *nginx.NgxServer, name string) []*nginx.NgxDirective {
	var directives []*nginx.NgxDirective
	for _, directive := range server.Directives {
		if directive.Directive == name {
			directives = append(directives, directive)
		}
	}
	return directives
}

func (p *HTTPSPlan) firstPendingTLS() *nginx.NgxServer {
	for _, slot := range p.slots {
		if slot.kind == httpsSlotPendingTLS || slot.kind == httpsSlotPendingMixed {
			return slot.original
		}
	}
	return nil
}

func (p *HTTPSPlan) firstSlot(kinds ...httpsSlotKind) *httpsSlot {
	for _, slot := range p.slots {
		for _, kind := range kinds {
			if slot.kind == kind && slot.staged != nil {
				return slot
			}
		}
	}
	return nil
}

// stageCertifiedServer keeps a working TLS server as it is. The CA follows the
// port-80 redirect, so the TLS side answers the challenge as well.
func (p *HTTPSPlan) stageCertifiedServer(original *nginx.NgxServer, combined bool) *nginx.NgxServer {
	server := cloneServer(original)
	if len(p.missing) > 0 {
		appendServerNames(server, p.missing)
	}
	if p.challenge != nil {
		if combined {
			// The server is also the port-80 server of its names.
			moveServerReturnsIntoRoot(server)
		}
		ensureChallengeLocation(server, p.challenge)
	}
	return server
}

// stageHTTPServer prepares a covering port-80 server for the staged config.
func (p *HTTPSPlan) stageHTTPServer(original *nginx.NgxServer, pendingSource *nginx.NgxServer) *nginx.NgxServer {
	server := cloneServer(original)
	redirectOnly := serverRedirectsOnlyToHTTPS(server, p.isSiteTarget)

	if p.challenge != nil {
		// A server-level return runs before location matching and would
		// answer the challenge itself.
		moveServerReturnsIntoRoot(server)
	}

	if !p.HasCertifiedTLS {
		// Nothing can answer HTTPS yet, so a redirect would only lead to an
		// error page (and fail the HTTP-01 validation that follows it).
		stripRedirects(server, func(r httpsRedirect) bool { return p.isSiteTarget(r.target) })
		if redirectOnly {
			if pendingSource != nil {
				p.mergeAppContent(server, pendingSource)
			} else {
				p.Diagnostics = append(p.Diagnostics, HTTPSDiagnostic{
					Level:   acmehint.LevelWarning,
					Code:    HTTPSDiagnosticRedirectWithoutApp,
					Message: "The HTTP server only redirected to HTTPS and no HTTPS server content was found; it serves nothing but the challenge route until the certificate is issued",
					Params:  map[string]string{"server_name": serverNames(server)},
				})
			}
		}
	}

	if p.challenge != nil {
		ensureChallengeLocation(server, p.challenge)
	}
	return server
}

// finalizeTLSServer installs the certificate and removes the unguarded HTTPS
// redirects to the server itself, which would loop (F2). names, when not nil,
// replaces the server_name with the names the certificate covers.
func (p *HTTPSPlan) finalizeTLSServer(server *nginx.NgxServer, names []string, certPath, keyPath string) *nginx.NgxServer {
	own := appendLiteralNames(nil, p.finalServerNames(server))
	stripRedirects(server, func(r httpsRedirect) bool { return isSelfRedirectLoop(r, own) })
	setCertificatePaths(server, certPath, keyPath)
	if len(p.missing) > 0 {
		appendServerNames(server, p.missing)
	}
	if names != nil {
		setServerNames(server, names)
	}
	if p.challenge != nil {
		ensureChallengeLocation(server, p.challenge)
	}
	return server
}

// generateTLSServer builds a TLS server for names from the app content of a
// port-80 server.
func (p *HTTPSPlan) generateTLSServer(source *nginx.NgxServer, names []string, certPath, keyPath string) *nginx.NgxServer {
	server := nginx.NewNgxServer()
	server.Directives = append(server.Directives,
		&nginx.NgxDirective{Directive: "listen", Params: "443 ssl"},
		&nginx.NgxDirective{Directive: "listen", Params: "[::]:443 ssl"},
		&nginx.NgxDirective{Directive: "server_name", Params: strings.Join(names, " ")},
		&nginx.NgxDirective{Directive: "ssl_certificate", Params: certPath},
		&nginx.NgxDirective{Directive: "ssl_certificate_key", Params: keyPath},
	)

	if source != nil {
		directives, locations := p.appContent(source)
		server.Directives = append(server.Directives, directives...)
		server.Locations = append(server.Locations, locations...)
	}
	if p.challenge != nil {
		ensureChallengeLocation(server, p.challenge)
	}
	return server
}

// redirectServer turns a port-80 server into the canonical HTTP->HTTPS
// redirect, keeping the challenge route reachable over plain HTTP.
func (p *HTTPSPlan) redirectServer(server *nginx.NgxServer) *nginx.NgxServer {
	if locationRootRedirectsOnlyToHTTPS(server) && !hasServerLevelHTTPSRedirect(server) {
		// Already the canonical shape (typically a reissue).
		if p.challenge != nil {
			ensureChallengeLocation(server, p.challenge)
		}
		return server
	}

	redirect := nginx.NewNgxServer()
	redirect.Comments = server.Comments
	for _, directive := range server.Directives {
		if directive.Directive == "listen" || directive.Directive == "server_name" {
			redirect.Directives = append(redirect.Directives, cloneDirective(directive))
		}
	}
	if p.challenge == nil {
		// DNS-01 does not install a challenge route, but an existing one keeps
		// working for HTTP-01 renewals configured elsewhere.
		for _, location := range server.Locations {
			if isChallengeLocation(location) {
				redirect.Locations = append(redirect.Locations, cloneLocation(location))
			}
		}
	}
	redirect.Locations = append(redirect.Locations, newRootRedirectLocation())
	if p.challenge != nil {
		ensureChallengeLocation(redirect, p.challenge)
	}
	return redirect
}

func newRootRedirectLocation() *nginx.NgxLocation {
	return &nginx.NgxLocation{
		Path:    "/",
		Content: "return 301 https://$host$request_uri;\n",
	}
}

func letsEncryptChallengeLocation() (*nginx.NgxLocation, error) {
	block, err := internalTemplate.ParseTemplate("block", "letsencrypt.conf", nil)
	if err != nil {
		return nil, err
	}
	if len(block.Locations) == 0 {
		return nil, errors.New("letsencrypt.conf contains no location")
	}
	return block.Locations[0], nil
}

var reUnsafeServerName = regexp.MustCompile(`[\s;{}"'\\$#]`)

func normalizeHTTPSDomains(domains []string) ([]string, error) {
	result := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
		if domain == "" {
			continue
		}
		if reUnsafeServerName.MatchString(domain) {
			return nil, fmt.Errorf("%w: %q", ErrHTTPSInvalidDomain, domain)
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
	}
	if len(result) == 0 {
		return nil, ErrHTTPSNoDomains
	}
	return result, nil
}

func newPlainHTTPServer(domains []string) *nginx.NgxServer {
	server := nginx.NewNgxServer()
	server.Directives = append(server.Directives,
		&nginx.NgxDirective{Directive: "listen", Params: "80"},
		&nginx.NgxDirective{Directive: "listen", Params: "[::]:80"},
		&nginx.NgxDirective{Directive: "server_name", Params: strings.Join(domains, " ")},
	)
	return server
}

// --- server classification ---

func trimParams(params string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(params), ";"))
}

func serverNameList(server *nginx.NgxServer) []string {
	var names []string
	for _, directive := range server.Directives {
		if directive.Directive == "server_name" {
			names = append(names, strings.Fields(trimParams(directive.Params))...)
		}
	}
	return names
}

func serverNames(server *nginx.NgxServer) string {
	return strings.Join(serverNameList(server), " ")
}

func serverCoversDomains(server *nginx.NgxServer, domains []string) bool {
	for _, name := range serverNameList(server) {
		for _, domain := range domains {
			if serverNameMatches(name, domain) {
				return true
			}
		}
	}
	return false
}

func markCoveredDomains(server *nginx.NgxServer, domains []string, covered map[string]bool) {
	for _, name := range serverNameList(server) {
		for _, domain := range domains {
			if serverNameMatches(name, domain) {
				covered[domain] = true
			}
		}
	}
}

func normalizeServerName(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.Trim(name, `"'`)), ".")
}

// serverNameMatches reports whether an nginx server_name entry matches domain.
func serverNameMatches(name, domain string) bool {
	name = strings.Trim(name, `"'`)
	if strings.HasPrefix(name, "~") {
		re, err := regexp.Compile("(?i)" + strings.TrimPrefix(name, "~"))
		return err == nil && re.MatchString(domain)
	}
	name = normalizeServerName(name)
	switch {
	case name == domain:
		return true
	case strings.HasPrefix(name, "*."):
		return strings.HasSuffix(domain, name[1:]) && !strings.HasPrefix(domain, "*.")
	case strings.HasPrefix(name, "."):
		return domain == name[1:] || strings.HasSuffix(domain, name)
	case strings.HasSuffix(name, ".*"):
		return strings.HasPrefix(domain, strings.TrimSuffix(name, "*"))
	}
	return false
}

// nameCoveredByCertificate reports whether a certificate for domains is valid
// for every host an nginx server_name entry matches. `_`, regex names and
// wildcards are covered only by an identical wildcard domain.
func nameCoveredByCertificate(name string, domains []string) bool {
	name = strings.Trim(name, `"'`)
	if name == "" || name == "_" || strings.HasPrefix(name, "~") {
		return false
	}
	name = normalizeServerName(name)
	if strings.HasSuffix(name, ".*") {
		return false
	}
	if strings.HasPrefix(name, ".") {
		// .example.com is example.com plus *.example.com.
		return nameCoveredByCertificate(name[1:], domains) && nameCoveredByCertificate("*"+name, domains)
	}
	for _, domain := range domains {
		if domain == name {
			return true
		}
		if strings.HasPrefix(domain, "*.") && !strings.HasPrefix(name, "*.") {
			label, ok := strings.CutSuffix(name, domain[1:])
			if ok && label != "" && !strings.Contains(label, ".") {
				return true
			}
		}
	}
	return false
}

// appendLiteralNames adds the plain host names among names (no regex,
// wildcard or catch-all) to list.
func appendLiteralNames(list []string, names []string) []string {
	for _, name := range names {
		name = normalizeServerName(name)
		if name == "" || name == "_" || strings.ContainsAny(name, "~*") || strings.HasPrefix(name, ".") {
			continue
		}
		if !containsFold(list, name) {
			list = append(list, name)
		}
	}
	return list
}

// serverListenKinds reports whether the server listens with TLS and whether it
// serves plain HTTP on port 80. A server without listen directive listens on 80.
func serverListenKinds(server *nginx.NgxServer) (tls bool, plain80 bool) {
	hasListen := false
	for _, directive := range server.Directives {
		switch directive.Directive {
		case "listen":
			hasListen = true
			fields := strings.Fields(trimParams(directive.Params))
			if len(fields) == 0 {
				continue
			}
			if listenHasSSL(fields) {
				tls = true
				continue
			}
			if listenPort(fields[0]) == "80" {
				plain80 = true
			}
		case "ssl":
			if strings.EqualFold(trimParams(directive.Params), "on") {
				tls = true
			}
		}
	}
	if !hasListen {
		plain80 = true
	}
	return tls, plain80
}

func listenHasSSL(fields []string) bool {
	for _, field := range fields[1:] {
		if field == "ssl" {
			return true
		}
	}
	return false
}

func listenPort(address string) string {
	if strings.HasPrefix(address, "unix:") {
		return ""
	}
	if idx := strings.LastIndex(address, "]:"); idx >= 0 {
		return address[idx+2:]
	}
	if strings.HasPrefix(address, "[") {
		return "80"
	}
	if idx := strings.LastIndex(address, ":"); idx >= 0 {
		return address[idx+1:]
	}
	if strings.Trim(address, "0123456789") == "" {
		return address
	}
	return "80"
}

func serverHasCertificate(server *nginx.NgxServer) bool {
	var hasCert, hasKey bool
	for _, directive := range server.Directives {
		switch directive.Directive {
		case "ssl_certificate":
			hasCert = hasCert || trimParams(directive.Params) != ""
		case "ssl_certificate_key":
			hasKey = hasKey || trimParams(directive.Params) != ""
		}
	}
	return hasCert && hasKey
}

// --- redirects ---

var redirectStatusCodes = map[string]bool{"301": true, "302": true, "303": true, "307": true, "308": true}

// httpsRedirectTarget returns the https:// URL a `return` or `rewrite`
// statement redirects to. A rewrite whose replacement starts with https://
// always redirects, whatever its flag.
func httpsRedirectTarget(statement string) (string, bool) {
	fields := strings.Fields(trimParams(statement))
	if len(fields) < 2 {
		return "", false
	}
	var target string
	switch fields[0] {
	case "return":
		switch {
		case len(fields) >= 3 && redirectStatusCodes[fields[1]]:
			target = fields[2]
		case len(fields) == 2:
			// `return URL;` is a 302.
			target = fields[1]
		default:
			return "", false
		}
	case "rewrite":
		if len(fields) < 3 {
			return "", false
		}
		target = fields[2]
	default:
		return "", false
	}
	target = strings.Trim(target, `"'`)
	if !strings.HasPrefix(strings.ToLower(target), "https://") {
		return "", false
	}
	return target, true
}

// redirectTargetHost returns the host part of an https:// target: a lower-case
// literal host, or a variable reference such as "$host".
func redirectTargetHost(target string) string {
	rest := target[len("https://"):]
	if strings.HasPrefix(rest, "${") {
		if end := strings.Index(rest, "}"); end > 0 {
			return "$" + rest[2:end]
		}
		return rest
	}
	if strings.HasPrefix(rest, "$") {
		end := 1
		for end < len(rest) && (rest[end] == '_' || rest[end] >= 'a' && rest[end] <= 'z' ||
			rest[end] >= 'A' && rest[end] <= 'Z' || rest[end] >= '0' && rest[end] <= '9') {
			end++
		}
		return rest[:end]
	}
	end := strings.IndexAny(rest, "/?:$")
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSuffix(strings.ToLower(rest[:end]), ".")
}

func isSelfHostVariable(host string) bool {
	switch host {
	case "$host", "$http_host", "$server_name":
		return true
	}
	return false
}

// isSelfTarget reports whether target redirects to the requested host itself
// or to one of names.
func isSelfTarget(target string, names []string) bool {
	host := redirectTargetHost(target)
	if isSelfHostVariable(host) {
		return true
	}
	return !strings.HasPrefix(host, "$") && containsFold(names, host)
}

// isSelfRedirectLoop reports an HTTPS redirect that loops once the server
// answers HTTPS itself: an unguarded redirect to its own host that applies to
// every request (server level) or keeps the requested URI.
func isSelfRedirectLoop(r httpsRedirect, names []string) bool {
	if isProtectiveGuard(r.guard) || !isSelfTarget(r.target, names) {
		return false
	}
	if !r.inLocation {
		return true
	}
	return keepsRequestURI(r.target)
}

// keepsRequestURI reports whether an https:// target sends the client back to
// the URI it asked for.
func keepsRequestURI(target string) bool {
	rest := target[len("https://"):]
	host := redirectTargetHost(target)
	if strings.HasPrefix(host, "$") {
		if strings.HasPrefix(rest, "${") {
			rest = rest[strings.Index(rest, "}")+1:]
		} else {
			rest = rest[len(host):]
		}
	} else if idx := strings.IndexAny(rest, "/$"); idx >= 0 {
		rest = rest[idx:]
	} else {
		rest = ""
	}
	for _, variable := range []string{"$request_uri", "${request_uri}", "$uri", "${uri}"} {
		if strings.HasPrefix(rest, variable) {
			return true
		}
	}
	return false
}

// isSiteTarget reports whether target redirects to a name this site serves or
// is being onboarded for. Redirects to other hosts do not depend on the TLS
// server of this site and are left alone.
func (p *HTTPSPlan) isSiteTarget(target string) bool {
	return isSelfTarget(target, p.siteNames)
}

func anyHTTPSTarget(string) bool { return true }

// isProtectiveGuard reports whether an if condition keeps an HTTPS redirect
// from looping on the TLS server: it tests the scheme or port, or it is a
// negated comparison such as `$host != $server_name` (canonical host).
func isProtectiveGuard(condition string) bool {
	if condition == "" {
		return false
	}
	for _, variable := range []string{"$scheme", "$https", "$server_port", "$http_x_forwarded_proto"} {
		if strings.Contains(condition, variable) {
			return true
		}
	}
	return strings.Contains(condition, "!=") || strings.Contains(condition, "!~")
}

func isSchemeGuard(condition string) bool {
	for _, variable := range []string{"$scheme", "$https", "$server_port"} {
		if strings.Contains(condition, variable) {
			return true
		}
	}
	return false
}

// ifCondition returns the condition of an `if (...)` block header.
func ifCondition(header string) (string, bool) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "if") {
		return "", false
	}
	rest := strings.TrimSpace(header[2:])
	if !strings.HasPrefix(rest, "(") {
		return "", false
	}
	rest = strings.TrimPrefix(rest, "(")
	if idx := strings.LastIndex(rest, ")"); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest), true
}

// httpsRedirect is one HTTPS redirect statement of a server.
type httpsRedirect struct {
	target string
	// guard is the condition of the innermost enclosing if block, "" for an
	// unconditional statement.
	guard string
	// inLocation reports a redirect inside a location block.
	inLocation bool
}

// filterRedirects walks the text of an nginx block body line by line (the
// layout the config parser produces) and drops each HTTPS redirect statement
// for which drop returns true. If blocks emptied by the filter are removed.
func filterRedirects(content string, inLocation bool, drop func(httpsRedirect) bool) (string, bool) {
	type frame struct {
		start   int
		isIf    bool
		guard   string
		dropped bool
	}
	var (
		out     []string
		stack   []frame
		changed bool
	)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasSuffix(line, "{"):
			f := frame{start: len(out)}
			if condition, ok := ifCondition(strings.TrimSuffix(line, "{")); ok {
				f.isIf, f.guard = true, condition
			} else if len(stack) > 0 {
				f.guard = stack[len(stack)-1].guard
			}
			stack = append(stack, f)
			out = append(out, raw)
		case line == "}" && len(stack) > 0:
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if f.isIf && f.dropped && len(out) == f.start+1 {
				out = out[:f.start]
				if len(stack) > 0 {
					stack[len(stack)-1].dropped = true
				}
				continue
			}
			out = append(out, raw)
		default:
			guard := ""
			if len(stack) > 0 {
				guard = stack[len(stack)-1].guard
			}
			if target, ok := httpsRedirectTarget(line); ok && drop(httpsRedirect{target: target, guard: guard, inLocation: inLocation}) {
				changed = true
				if len(stack) > 0 {
					stack[len(stack)-1].dropped = true
				}
				continue
			}
			out = append(out, raw)
		}
	}
	return strings.Join(out, "\n"), changed
}

// visitRedirects calls visit for each HTTPS redirect statement of content.
func visitRedirects(content string, inLocation bool, visit func(httpsRedirect)) {
	filterRedirects(content, inLocation, func(r httpsRedirect) bool {
		visit(r)
		return false
	})
}

// isIfBlockDirective reports a server-level `if` block, which the parser keeps
// as an unnamed directive holding the whole block text.
func isIfBlockDirective(directive *nginx.NgxDirective) (string, bool) {
	text := directive.Params
	if directive.Raw != "" {
		text = directive.Raw
	}
	if directive.Directive != "" && directive.Directive != "if" {
		return "", false
	}
	trimmed := strings.TrimSpace(text)
	if directive.Directive == "if" {
		trimmed = "if " + trimmed
	}
	if _, ok := ifCondition(strings.SplitN(trimmed, "{", 2)[0]); !ok {
		return "", false
	}
	return trimmed, true
}

func serverLevelStatement(directive *nginx.NgxDirective) string {
	return directive.Directive + " " + trimParams(directive.Params)
}

// serverRedirects visits the HTTPS redirects of the server-level directives.
func serverRedirects(server *nginx.NgxServer, visit func(httpsRedirect)) {
	for _, directive := range server.Directives {
		if text, ok := isIfBlockDirective(directive); ok {
			visitRedirects(text, false, visit)
			continue
		}
		if target, ok := httpsRedirectTarget(serverLevelStatement(directive)); ok {
			visit(httpsRedirect{target: target})
		}
	}
}

func hasServerLevelHTTPSRedirect(server *nginx.NgxServer) bool {
	found := false
	serverRedirects(server, func(httpsRedirect) { found = true })
	return found
}

// hasSchemeGuardedHTTPSRedirect reports an `if ($scheme ...) { return 301
// https://...; }` style redirect at server level or in `location /`.
func hasSchemeGuardedHTTPSRedirect(server *nginx.NgxServer) bool {
	found := false
	visit := func(r httpsRedirect) {
		if isSchemeGuard(r.guard) {
			found = true
		}
	}
	serverRedirects(server, visit)
	for _, location := range server.Locations {
		if isRootLocation(location) {
			visitRedirects(location.Content, true, visit)
		}
	}
	return found
}

// contentRedirectsOnly reports whether every statement of a location body is
// an HTTPS redirect accepted by match.
func contentRedirectsOnly(content string, match func(target string) bool) bool {
	found := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		target, ok := httpsRedirectTarget(line)
		if !ok || !match(target) {
			return false
		}
		found = true
	}
	return found
}

func contentRedirectsOnlyToHTTPS(content string) bool {
	return contentRedirectsOnly(content, anyHTTPSTarget)
}

func isRootLocation(location *nginx.NgxLocation) bool {
	return strings.TrimSpace(location.Path) == "/"
}

func locationRootRedirectsOnlyToHTTPS(server *nginx.NgxServer) bool {
	return locationRootRedirectsOnly(server, anyHTTPSTarget)
}

func locationRootRedirectsOnly(server *nginx.NgxServer, match func(target string) bool) bool {
	for _, location := range server.Locations {
		if isRootLocation(location) {
			return contentRedirectsOnly(location.Content, match)
		}
	}
	return false
}

// serverRedirectsOnlyToHTTPS reports whether the server's answer to ordinary
// requests is an HTTPS redirect to a target accepted by match: an
// unconditional server-level redirect, a `location /` that only redirects, or
// guarded server-level redirects on a server with no content of its own.
func serverRedirectsOnlyToHTTPS(server *nginx.NgxServer, match func(target string) bool) bool {
	unconditional, guarded := false, false
	serverRedirects(server, func(r httpsRedirect) {
		if !match(r.target) {
			return
		}
		if r.guard == "" {
			unconditional = true
		} else {
			guarded = true
		}
	})
	if unconditional || locationRootRedirectsOnly(server, match) {
		return true
	}
	return guarded && !serverHasAppContent(server)
}

// serverHasAppContent reports whether the server serves anything besides
// redirects and the challenge route.
func serverHasAppContent(server *nginx.NgxServer) bool {
	for _, directive := range server.Directives {
		switch directive.Directive {
		case "root", "alias", "proxy_pass", "fastcgi_pass", "uwsgi_pass", "scgi_pass", "grpc_pass", "try_files", "include":
			return true
		}
	}
	for _, location := range server.Locations {
		if !isChallengeLocation(location) && !contentRedirectsOnlyToHTTPS(location.Content) {
			return true
		}
	}
	return false
}

// stripRedirects removes the HTTPS redirects accepted by drop from the server
// and drops the locations and if blocks left empty.
func stripRedirects(server *nginx.NgxServer, drop func(httpsRedirect) bool) {
	directives := server.Directives[:0:0]
	for _, directive := range server.Directives {
		if text, ok := isIfBlockDirective(directive); ok {
			filtered, changed := filterRedirects(text, false, drop)
			if changed {
				if strings.TrimSpace(filtered) == "" {
					continue
				}
				directive.Directive = ""
				directive.Params = filtered
				directive.Raw = ""
			}
		} else if target, ok := httpsRedirectTarget(serverLevelStatement(directive)); ok && drop(httpsRedirect{target: target}) {
			continue
		}
		directives = append(directives, directive)
	}
	server.Directives = directives

	locations := server.Locations[:0:0]
	for _, location := range server.Locations {
		if isChallengeLocation(location) {
			locations = append(locations, location)
			continue
		}
		if filtered, changed := filterRedirects(location.Content, true, drop); changed {
			location.Content = filtered
			if strings.TrimSpace(location.Content) == "" {
				continue
			}
		}
		locations = append(locations, location)
	}
	server.Locations = locations
}

// moveServerReturnsIntoRoot moves server-level return directives into
// `location /` so the challenge location stays reachable.
func moveServerReturnsIntoRoot(server *nginx.NgxServer) {
	var returns []string
	directives := server.Directives[:0:0]
	for _, directive := range server.Directives {
		if directive.Directive == "return" {
			returns = append(returns, "return "+trimParams(directive.Params)+";")
			continue
		}
		directives = append(directives, directive)
	}
	if len(returns) == 0 {
		return
	}
	server.Directives = directives

	content := strings.Join(returns, "\n") + "\n"
	for _, location := range server.Locations {
		if isRootLocation(location) {
			location.Content = content + location.Content
			return
		}
	}
	server.Locations = append(server.Locations, &nginx.NgxLocation{Path: "/", Content: content})
}

// --- content helpers ---

func isChallengeLocation(location *nginx.NgxLocation) bool {
	return strings.Contains(location.Path, acmeChallengePath)
}

func ensureChallengeLocation(server *nginx.NgxServer, challenge *nginx.NgxLocation) {
	locations := server.Locations[:0:0]
	for _, location := range server.Locations {
		if !isChallengeLocation(location) {
			locations = append(locations, location)
		}
	}
	server.Locations = append(locations, cloneLocation(challenge))
}

// isTLSOnlyDirective reports directives that belong to the TLS server shell
// rather than to the application it serves.
func isTLSOnlyDirective(name string) bool {
	switch {
	case name == "listen", name == "server_name", name == "ssl", name == "http2", name == "http3":
		return true
	case strings.HasPrefix(name, "ssl_"), strings.HasPrefix(name, "quic_"):
		return true
	}
	return false
}

// appContent returns a copy of the application part of a server: directives
// except the listen/name/TLS shell and redirects to this site over HTTPS, and
// locations except the challenge route.
func (p *HTTPSPlan) appContent(server *nginx.NgxServer) (directives []*nginx.NgxDirective, locations []*nginx.NgxLocation) {
	source := cloneServer(server)
	stripRedirects(source, func(r httpsRedirect) bool { return p.isSiteTarget(r.target) })
	for _, directive := range source.Directives {
		if isTLSOnlyDirective(directive.Directive) {
			continue
		}
		directives = append(directives, directive)
	}
	for _, location := range source.Locations {
		if isChallengeLocation(location) {
			continue
		}
		locations = append(locations, location)
	}
	return directives, locations
}

// mergeAppContent makes server serve the application content of source while
// keeping its own listen/name shell and non-conflicting directives.
func (p *HTTPSPlan) mergeAppContent(server *nginx.NgxServer, source *nginx.NgxServer) {
	directives, locations := p.appContent(source)

	appNames := make(map[string]struct{}, len(directives))
	for _, directive := range directives {
		if directive.Directive != "" {
			appNames[directive.Directive] = struct{}{}
		}
	}
	merged := server.Directives[:0:0]
	for _, directive := range server.Directives {
		if _, ok := appNames[directive.Directive]; ok && directive.Directive != "" {
			continue
		}
		merged = append(merged, directive)
	}
	server.Directives = append(merged, directives...)

	appPaths := make(map[string]struct{}, len(locations))
	for _, location := range locations {
		appPaths[strings.TrimSpace(location.Path)] = struct{}{}
	}
	kept := server.Locations[:0:0]
	for _, location := range server.Locations {
		if _, ok := appPaths[strings.TrimSpace(location.Path)]; ok {
			continue
		}
		kept = append(kept, location)
	}
	server.Locations = append(kept, locations...)
}

// plainHTTPPart returns the plain HTTP half of a server that listens on 80 and
// on an ssl port at once.
func plainHTTPPart(server *nginx.NgxServer) *nginx.NgxServer {
	part := cloneServer(server)
	directives := part.Directives[:0:0]
	for _, directive := range part.Directives {
		if directive.Directive == "listen" {
			fields := strings.Fields(trimParams(directive.Params))
			if len(fields) > 0 && listenHasSSL(fields) {
				continue
			}
			directives = append(directives, directive)
			continue
		}
		if isTLSOnlyDirective(directive.Directive) && directive.Directive != "server_name" {
			continue
		}
		directives = append(directives, directive)
	}
	part.Directives = directives
	return part
}

// splitCombinedServer splits a server listening on 80 and on an ssl port into
// a port-80 shell (its port-80 listens and names) and the TLS server with
// everything else.
func splitCombinedServer(server *nginx.NgxServer) (httpPart, tlsPart *nginx.NgxServer) {
	httpPart = nginx.NewNgxServer()
	tlsPart = cloneServer(server)
	directives := tlsPart.Directives[:0:0]
	for _, directive := range tlsPart.Directives {
		switch directive.Directive {
		case "listen":
			fields := strings.Fields(trimParams(directive.Params))
			if len(fields) > 0 && !listenHasSSL(fields) && listenPort(fields[0]) == "80" {
				httpPart.Directives = append(httpPart.Directives, cloneDirective(directive))
				continue
			}
		case "server_name":
			httpPart.Directives = append(httpPart.Directives, cloneDirective(directive))
		}
		directives = append(directives, directive)
	}
	tlsPart.Directives = directives
	return httpPart, tlsPart
}

func setCertificatePaths(server *nginx.NgxServer, certPath, keyPath string) {
	certSet, keySet := false, false
	directives := server.Directives[:0:0]
	for _, directive := range server.Directives {
		switch directive.Directive {
		case "ssl_certificate":
			if !certSet {
				directive.Params = certPath
				directive.Raw = ""
				certSet = true
			} else if trimParams(directive.Params) == "" {
				continue
			}
		case "ssl_certificate_key":
			if !keySet {
				directive.Params = keyPath
				directive.Raw = ""
				keySet = true
			} else if trimParams(directive.Params) == "" {
				continue
			}
		}
		directives = append(directives, directive)
	}
	server.Directives = directives

	var missing []*nginx.NgxDirective
	if !certSet {
		missing = append(missing, &nginx.NgxDirective{Directive: "ssl_certificate", Params: certPath})
	}
	if !keySet {
		missing = append(missing, &nginx.NgxDirective{Directive: "ssl_certificate_key", Params: keyPath})
	}
	if len(missing) == 0 {
		return
	}

	// Insert right after the listen/server_name shell.
	insertAt := 0
	for i, directive := range server.Directives {
		if directive.Directive == "listen" || directive.Directive == "server_name" {
			insertAt = i + 1
		}
	}
	result := make([]*nginx.NgxDirective, 0, len(server.Directives)+len(missing))
	result = append(result, server.Directives[:insertAt]...)
	result = append(result, missing...)
	result = append(result, server.Directives[insertAt:]...)
	server.Directives = result
}

func appendServerNames(server *nginx.NgxServer, names []string) {
	for _, directive := range server.Directives {
		if directive.Directive == "server_name" {
			existing := strings.Fields(trimParams(directive.Params))
			for _, name := range names {
				if !containsFold(existing, name) {
					existing = append(existing, name)
				}
			}
			directive.Params = strings.Join(existing, " ")
			directive.Raw = ""
			return
		}
	}
	server.Directives = append(server.Directives, &nginx.NgxDirective{
		Directive: "server_name",
		Params:    strings.Join(names, " "),
	})
}

// setServerNames replaces every server_name directive with one listing names.
func setServerNames(server *nginx.NgxServer, names []string) {
	directives := server.Directives[:0:0]
	set := false
	for _, directive := range server.Directives {
		if directive.Directive == "server_name" {
			if set {
				continue
			}
			directive.Params = strings.Join(names, " ")
			directive.Raw = ""
			set = true
		}
		directives = append(directives, directive)
	}
	server.Directives = directives
	if !set {
		appendServerNames(server, names)
	}
}

func containsFold(values []string, value string) bool {
	for _, v := range values {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

// --- cloning ---

func cloneDirective(directive *nginx.NgxDirective) *nginx.NgxDirective {
	copied := *directive
	return &copied
}

func cloneLocation(location *nginx.NgxLocation) *nginx.NgxLocation {
	if location == nil {
		return nil
	}
	copied := *location
	return &copied
}

func cloneServer(server *nginx.NgxServer) *nginx.NgxServer {
	if server == nil {
		return nil
	}
	copied := &nginx.NgxServer{
		Comments:   server.Comments,
		Directives: make([]*nginx.NgxDirective, 0, len(server.Directives)),
		Locations:  make([]*nginx.NgxLocation, 0, len(server.Locations)),
	}
	for _, directive := range server.Directives {
		if directive != nil {
			copied.Directives = append(copied.Directives, cloneDirective(directive))
		}
	}
	for _, location := range server.Locations {
		if location != nil {
			copied.Locations = append(copied.Locations, cloneLocation(location))
		}
	}
	return copied
}

// cloneConfigShell copies everything except the servers: top-level custom
// content, upstreams and the root block are preserved untouched.
func cloneConfigShell(cfg *nginx.NgxConfig) *nginx.NgxConfig {
	copied := &nginx.NgxConfig{
		FileName:  cfg.FileName,
		Name:      cfg.Name,
		RootBlock: cfg.RootBlock,
		Custom:    cfg.Custom,
		Upstreams: make([]*nginx.NgxUpstream, 0, len(cfg.Upstreams)),
		Servers:   make([]*nginx.NgxServer, 0, len(cfg.Servers)+1),
	}
	for _, upstream := range cfg.Upstreams {
		if upstream == nil {
			continue
		}
		u := &nginx.NgxUpstream{Name: upstream.Name, Comments: upstream.Comments}
		for _, directive := range upstream.Directives {
			if directive != nil {
				u.Directives = append(u.Directives, cloneDirective(directive))
			}
		}
		copied.Upstreams = append(copied.Upstreams, u)
	}
	return copied
}
