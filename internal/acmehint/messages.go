// Package acmehint turns raw ACME (lego) failures into actionable hints and
// runs lightweight DNS diagnostics for the identifiers of a certificate.
//
// The package is transport agnostic: it has no gin or websocket dependency so
// both the legacy issuance websocket and the HTTPS onboarding orchestrator can
// reuse Classify and Diagnose. It never calls external HTTP services; the only
// network traffic it produces is DNS lookups through an injectable resolver.
package acmehint

import "github.com/0xJacky/Nginx-UI/internal/translation"

// Hint codes returned by Classify. They are stable machine codes; clients may
// translate by code and fall back to Hint.Message.
const (
	CodeDNSPointsElsewhere  = "dns_points_elsewhere"
	CodeChallengeNotServed  = "challenge_not_served"
	CodeChallengeRedirected = "challenge_redirected"
	CodePort80Unreachable   = "port80_unreachable"
	CodeDNSNotResolved      = "dns_not_resolved"
	CodeCAAForbidden        = "caa_forbidden"
	CodeRateLimited         = "rate_limited"
	CodeIdentifierRejected  = "identifier_rejected"
	CodeUnknown             = "unknown"
)

// Diagnostic codes returned by Diagnose. CodeDNSPointsElsewhere is shared with
// the hint codes because it describes the same fact.
const (
	CodeDNSOK                 = "dns_ok"
	CodeDNSResolved           = "dns_resolved"
	CodeDNSNoRecords          = "dns_no_records"
	CodeDNSLookupFailed       = "dns_lookup_failed"
	CodeAAAAWithoutIPv6Listen = "aaaa_without_ipv6_listen"
	CodeCAABlocksCA           = "caa_blocks_ca"
)

// Diagnostic levels.
const (
	LevelInfo    = "info"
	LevelWarning = "warning"
)

// messages holds the English source sentence for every hint and diagnostic
// code. They are declared through translation.C so the gettext extractor picks
// them up and clients can translate the plain message string.
var messages = map[string]*translation.Container{
	// Hints.
	CodeDNSPointsElsewhere:  translation.C("The certificate authority reached a server that did not return the challenge file, and the domain resolves to an address that is not on this server; point the domain's A/AAAA records at this server, or, behind NAT or a CDN, make sure port 80 is forwarded to it."),
	CodeChallengeNotServed:  translation.C("The certificate authority reached the server but the challenge file was not served; make sure the port 80 server block for this domain is enabled and routes /.well-known/acme-challenge/ to Nginx UI, and that no other server block or proxy answers first."),
	CodeChallengeRedirected: translation.C("The certificate authority followed a redirect, but the redirected request did not reach the challenge route; make sure the HTTPS server for this host also proxies /.well-known/acme-challenge/ to the challenge port, and that the redirect stays on the same host and on port 80 or 443."),
	CodePort80Unreachable:   translation.C("The certificate authority could not connect to port 80 of this domain; open TCP port 80 in the firewall or cloud security group and make sure any NAT or port forwarding sends it to this server."),
	CodeDNSNotResolved:      translation.C("The certificate authority could not resolve the domain; create an A or AAAA record that points at this server and wait for DNS to propagate."),
	CodeCAAForbidden:        translation.C("A CAA record on the domain does not allow this certificate authority; add a CAA issue record for the certificate authority or remove the restrictive record."),
	CodeRateLimited:         translation.C("The certificate authority rate limit was reached; wait until the retry time before trying again, and test configuration changes against the staging environment."),
	CodeIdentifierRejected:  translation.C("The certificate authority refuses to issue a certificate for this name; use a publicly resolvable domain name that the certificate authority supports."),
	CodeUnknown:             translation.C("Certificate issuance failed; check the log for the response of the certificate authority."),

	// Diagnostics.
	CodeDNSOK:                 translation.C("The domain resolves to an address assigned to this server."),
	CodeDNSResolved:           translation.C("The domain has DNS address records, but the local addresses of this server could not be compared."),
	CodeDNSNoRecords:          translation.C("The domain has no A or AAAA record; create one that points at this server before requesting a certificate."),
	CodeDNSLookupFailed:       translation.C("The DNS lookup for the domain failed; make sure its name servers are answering."),
	CodeAAAAWithoutIPv6Listen: translation.C("The domain has an AAAA record but the site does not listen on IPv6; Let's Encrypt prefers IPv6, so add an IPv6 listen directive on port 80 or remove the AAAA record."),
	CodeCAABlocksCA:           translation.C("A CAA record on the domain does not authorize the selected certificate authority; add a CAA issue record for it or issuance will fail."),
}

// dnsPointsElsewhereDiagnostic is the diagnostic flavour of
// CodeDNSPointsElsewhere. It must mention that the mismatch can be expected.
var dnsPointsElsewhereDiagnostic = translation.C("The domain resolves to an address that is not assigned to this server; this can be expected behind NAT, a load balancer, or a CDN, otherwise update the domain's A/AAAA records.")

// Message returns the English source sentence for a hint or diagnostic code.
// Unknown codes yield the generic CodeUnknown sentence.
func Message(code string) string {
	if c, ok := messages[code]; ok {
		return c.Message
	}
	return messages[CodeUnknown].Message
}
