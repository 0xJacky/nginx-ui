package sites

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// Hint codes produced by the HTTP-01 probe mapping.
const (
	httpsHintChallengePortUnavailable  = "challenge_port_unavailable"
	httpsHintChallengeRouteUnavailable = "challenge_route_unavailable"
)

var httpsChallengePortUnavailableMessage = translation.C("Nginx UI could not listen on the HTTP challenge port; make sure no other process uses it, or change the HTTP challenge port in the certificate settings.")

// Seams for tests. Production code must treat them as read-only.
var (
	probeHTTP01Routes = cert.ProbeHTTP01Routes
	issueWithRecord   = cert.IssueWithRecord
	diagnoseHTTPS     = diagnoseHTTPSDomains
	// loadHTTPSCertificate loads the existing certificate a request selects.
	loadHTTPSCertificate = site.LoadHTTPSExistingCertificate
)

// httpsRequestReadTimeout bounds the wait for the request message, so a
// client that opens the socket and never sends does not hold the handler.
const httpsRequestReadTimeout = 30 * time.Second

// EnableSiteHTTPS streams the backend-orchestrated HTTPS onboarding of a site
// (plan -> stage -> probe -> issue -> finalize). The client sends one
// site.HTTPSRequest after the socket opens and receives site.HTTPSEvent
// messages, ending with exactly one "done" event.
func EnableSiteHTTPS(c *gin.Context) {
	enableSiteHTTPS(c, httpsRequestReadTimeout)
}

func enableSiteHTTPS(c *gin.Context, requestReadTimeout time.Duration) {
	name := helper.UnescapeURL(c.Param("name"))
	if rejectInvalidSiteName(c, name) {
		return
	}

	upGrader := websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
	}
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer ws.Close()

	var req site.HTTPSRequest
	if err := ws.SetReadDeadline(time.Now().Add(requestReadTimeout)); err != nil {
		logger.Error(err)
		return
	}
	if err := ws.ReadJSON(&req); err != nil {
		if helper.IsUnexpectedWebsocketError(err) {
			logger.Error(err)
		}
		return
	}
	// Clear the request deadline; the keepalive reader manages its own.
	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		logger.Error(err)
		return
	}

	// The handler only writes from here on; the keepalive owns the reader so
	// pongs are processed and a vanished peer is detected.
	keepalive := helper.StartWebSocketKeepaliveReader(ws)
	defer keepalive.Stop()

	wsWriter := helper.NewSafeWebSocketWriter(ws)
	emit := func(event site.HTTPSEvent) {
		if err := wsWriter.WriteJSON(event); err != nil && helper.IsUnexpectedWebsocketError(err) {
			logger.Error(err)
		}
	}

	// The run is not tied to the peer: once started, the transaction finishes
	// (or rolls back) even when the browser goes away, like the certificate
	// issuance websocket.
	newHTTPSOnboarding(name, req, emit).Run(c.Request.Context())

	_ = wsWriter.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

// newHTTPSOnboarding wires the orchestrator to the certificate and ACME hint
// packages.
func newHTTPSOnboarding(name string, req site.HTTPSRequest, emit func(site.HTTPSEvent)) *site.HTTPSOnboarding {
	return &site.HTTPSOnboarding{
		Name:      name,
		Request:   req,
		Emit:      emit,
		Normalize: normalizeHTTPSIdentifiers,
		Diagnose: func(ctx context.Context, domains []string, hasIPv6Listen bool) []site.HTTPSDiagnostic {
			return diagnoseHTTPS(ctx, domains, hasIPv6Listen, req.ACMEUserID)
		},
		Probe: func(ctx context.Context, domains []string) (site.HTTPSProbeResult, error) {
			return probeHTTPSChallengeRoute(ctx, name, domains)
		},
		Issue:           issueHTTPSCertificate,
		Hint:            httpsFailureHint,
		LoadCertificate: loadHTTPSCertificate,
	}
}

// normalizeHTTPSIdentifiers validates and canonicalizes the identifiers the
// same way the certificate issuance does (IDN to punycode, IP rules).
func normalizeHTTPSIdentifiers(domains []string, challengeMethod string) ([]string, error) {
	payload := &cert.ConfigPayload{
		ServerName:      domains,
		ChallengeMethod: strings.ToLower(strings.TrimSpace(challengeMethod)),
	}
	if err := cert.NormalizeAndValidateIdentifiers(payload); err != nil {
		return nil, err
	}
	return payload.ServerName, nil
}

// diagnoseHTTPSDomains runs the DNS diagnostics for the identifiers, checking
// CAA records against the CA of the selected ACME user.
func diagnoseHTTPSDomains(ctx context.Context, domains []string, hasIPv6Listen bool, acmeUserID uint64) []site.HTTPSDiagnostic {
	return acmehint.Diagnose(ctx, domains, acmehint.Options{
		HasIPv6Listen: hasIPv6Listen,
		CAAIdentities: acmehint.CAAIdentitiesForDirectory(httpsCADir(acmeUserID)),
	})
}

// httpsCADir returns the ACME directory of the ACME user, falling back to the
// directory configured in the certificate settings.
func httpsCADir(acmeUserID uint64) string {
	user, err := (&cert.ConfigPayload{ACMEUserID: acmeUserID}).GetACMEUser()
	if err == nil && user != nil && user.CADir != "" {
		return user.CADir
	}
	return settings.CertSettings.GetCADir()
}

// probeHTTPSChallengeRoute runs the active loopback probe for the site's
// domains. The probe discovers the endpoints of every domain from the
// effective configuration; the site name is only a fallback source of listen
// addresses when nginx -T cannot be read.
func probeHTTPSChallengeRoute(ctx context.Context, name string, domains []string) (site.HTTPSProbeResult, error) {
	results, err := probeHTTP01Routes(ctx, domains, cert.WithHTTP01ProbeConfigName(name))
	return httpsProbeOutcome(name, results, err)
}

// httpsProbeOutcome maps the loopback probe results to the orchestrator's
// probe result. Only failures, which always carry HTTP evidence (the local
// Nginx answered without the token), stop the run with a hint through
// *site.HTTPSHintError. Connection-level problems (refused, timeout, TLS
// handshake), a domain without a port-80 server block and an unreadable
// configuration are warnings, and the run continues.
func httpsProbeOutcome(name string, results []cert.HTTP01ProbeResult, err error) (site.HTTPSProbeResult, error) {
	if err != nil {
		if cosyErr, ok := matchCosyError(err, cert.ErrHTTP01ChallengePortUnavailable); ok {
			return site.HTTPSProbeResult{}, &site.HTTPSHintError{
				Hint: site.HTTPSHint{
					Code:    httpsHintChallengePortUnavailable,
					Message: httpsChallengePortUnavailableMessage.Message,
					Params:  cosyParams(cosyErr, "port", "reason"),
				},
				Err: err,
			}
		}
		return site.HTTPSProbeResult{}, err
	}

	if routeErr := cert.HTTP01RouteCheckError(results, name); routeErr != nil {
		hint := site.HTTPSHint{
			Code:    httpsHintChallengeRouteUnavailable,
			Message: routeErr.Error(),
		}
		var cosyErr *cosy.Error
		if errors.As(routeErr, &cosyErr) {
			hint.Params = cosyParams(cosyErr, "domain", "reason")
			if reason := hint.Params["reason"]; reason != "" {
				hint.Message = reason
			}
		}
		return site.HTTPSProbeResult{}, &site.HTTPSHintError{Hint: hint, Err: routeErr}
	}

	if cert.HTTP01ProbeSkipped(results) {
		return site.HTTPSProbeResult{Status: site.HTTPSStatusSkipped, Message: results[0].SkipReason}, nil
	}
	for _, result := range results {
		if result.Status == cert.HTTP01ProbeStatusWarning {
			return site.HTTPSProbeResult{
				Status:  site.HTTPSStatusWarning,
				Message: cert.SummarizeHTTP01ProbeResults(results),
			}, nil
		}
	}
	return site.HTTPSProbeResult{Status: site.HTTPSStatusSuccess}, nil
}

// matchCosyError reports whether err wraps a cosy error with the scope and
// code of target, and returns it.
func matchCosyError(err, target error) (*cosy.Error, bool) {
	var got, want *cosy.Error
	if !errors.As(err, &got) || !errors.As(target, &want) {
		return nil, false
	}
	return got, got.Scope == want.Scope && got.Code == want.Code
}

// cosyParams names the positional params of a cosy error.
func cosyParams(err *cosy.Error, names ...string) map[string]string {
	params := make(map[string]string, len(names))
	for i, name := range names {
		if i < len(err.Params) && err.Params[i] != "" {
			params[name] = err.Params[i]
		}
	}
	if len(params) == 0 {
		return nil
	}
	return params
}

// issueHTTPSCertificate issues the certificate with the regular cert record
// bookkeeping and streams the issuance log.
func issueHTTPSCertificate(_ context.Context, req site.HTTPSIssueRequest, logf func(message string, args map[string]any)) (site.HTTPSIssueResult, error) {
	payload := httpsIssuePayload(req)

	log, closeLog := cert.NewStreamLogger(func(c *translation.Container) {
		if logf != nil {
			logf(c.Message, c.Args)
		}
	})
	// Deferred so every buffered log line is emitted before the issue step
	// reports its outcome.
	defer closeLog()

	certModel, err := issueWithRecord(req.SiteName, payload, log)
	if err != nil {
		return site.HTTPSIssueResult{}, err
	}

	return httpsIssueResult(payload, certModel), nil
}

// httpsIssuePayload builds the issuance payload. ConfigName stays empty on
// purpose: the orchestrator has already probed the challenge route, so
// IssueCert must not run a second probe against the staged configuration.
func httpsIssuePayload(req site.HTTPSIssueRequest) *cert.ConfigPayload {
	payload := &cert.ConfigPayload{
		ServerName:                        append([]string(nil), req.Domains...),
		ChallengeMethod:                   req.ChallengeMethod,
		DNSCredentialID:                   req.DNSCredentialID,
		ACMEUserID:                        req.ACMEUserID,
		KeyType:                           req.KeyType,
		Profile:                           req.Profile,
		MustStaple:                        req.MustStaple,
		LegoDisableCNAMESupport:           req.LegoDisableCNAMESupport,
		DisableAuthoritativeNSPropagation: req.DisableAuthoritativeNSPropagation,
		EnableCommonName:                  req.EnableCommonName,
		RevokeOld:                         req.RevokeOld,
	}
	payload.KeyType = payload.GetKeyType()
	return payload
}

func httpsIssueResult(payload *cert.ConfigPayload, certModel *model.Cert) site.HTTPSIssueResult {
	result := site.HTTPSIssueResult{
		SSLCertificate:    payload.GetCertificatePath(),
		SSLCertificateKey: payload.GetCertificateKeyPath(),
		KeyType:           payload.GetKeyType(),
		Profile:           payload.Profile,
	}
	if certModel != nil {
		result.CertID = certModel.ID
	}
	return result
}

// httpsFailureHint classifies an issuance failure with the DNS evidence the
// probe step collected. Other steps either carry their hint in a
// *site.HTTPSHintError or have none.
func httpsFailureHint(step string, err error, diagnostics []site.HTTPSDiagnostic) *site.HTTPSHint {
	if step != site.HTTPSStepIssue || err == nil {
		return nil
	}
	return acmehint.Classify(err, acmehint.EvidenceFromDiagnostics(diagnostics))
}
