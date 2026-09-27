package sites

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// Check codes of the HTTPS pre-flight check.
const (
	httpsCheckConfigParse    = "config_parse"
	httpsCheckChallengeRoute = "challenge_route"
	httpsCheckDNS            = "dns"
	httpsCheckCertificate    = "certificate"
)

const (
	// httpsCheckProbeTimeout bounds the loopback probe, including the wait
	// for the certificate lock while another issuance is running.
	httpsCheckProbeTimeout = 20 * time.Second
	// httpsCheckDiagnoseTimeout bounds the DNS diagnostics.
	httpsCheckDiagnoseTimeout = 10 * time.Second
)

var (
	httpsCheckConfigOKMessage       = translation.C("The site configuration can be updated automatically")
	httpsCheckConfigBlockedMessage  = translation.C("Resolve the site configuration problem first")
	httpsCheckRouteAfterStageMsg    = translation.C("Verified after the site is staged")
	httpsCheckRouteDNSMessage       = translation.C("The DNS challenge does not use the HTTP challenge route")
	httpsCheckRouteReachableMessage = translation.C("HTTP challenge route is reachable")
	httpsCheckDNSOKMessage          = translation.C("No DNS problem found")
	httpsCheckExistingCertMessage   = translation.C("An existing certificate is used")
	httpsCheckCertificateOKMessage  = translation.C("The certificate %{name} covers the domains and is valid until %{not_after}")
	httpsCheckCertificatePartialMsg = translation.C("The certificate %{name} does not cover some of the domains; they are left out of HTTPS: %{uncovered}")
)

// HTTPSCheckRequest is the body of POST sites/:name/https/check.
type HTTPSCheckRequest struct {
	Domains         []string `json:"domains"`
	ChallengeMethod string   `json:"challenge_method"`
	// ACMEUserID selects the CA used for the CAA check. Optional.
	ACMEUserID uint64 `json:"acme_user_id"`
	// CertificateID checks an existing certificate from the certificate
	// manager instead of the issuance. Optional.
	CertificateID uint64 `json:"certificate_id"`
}

// HTTPSCheck is one pre-flight check result.
type HTTPSCheck struct {
	Code string `json:"code"`
	// Status is success, warning, error or skipped.
	Status string `json:"status"`
	// Message is an English source string (translated by the client).
	Message string `json:"message"`
	// Params are the translation arguments of Message plus structured
	// details (for example "diagnostics").
	Params map[string]any `json:"params,omitempty"`
	// Detail is a human-readable detail line.
	Detail string `json:"detail,omitempty"`
	// Hint explains an error in actionable terms.
	Hint *site.HTTPSHint `json:"hint,omitempty"`
}

// HTTPSCheckResponse is the response of POST sites/:name/https/check.
type HTTPSCheckResponse struct {
	Checks []HTTPSCheck `json:"checks"`
}

// CheckSiteHTTPS runs the side-effect free pre-flight checks of the HTTPS
// onboarding: whether the configuration can be planned, whether the HTTP-01
// challenge route already works, and what DNS says about the identifiers.
func CheckSiteHTTPS(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))
	if rejectInvalidSiteName(c, name) {
		return
	}

	var req HTTPSCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	path, err := site.ResolveAvailablePath(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if _, err := nginx.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	c.JSON(http.StatusOK, HTTPSCheckResponse{
		Checks: runHTTPSChecks(c.Request.Context(), name, req),
	})
}

// runHTTPSChecks returns the config_parse, challenge_route and dns checks, in
// that order. With an existing certificate it returns config_parse,
// certificate, challenge_route and dns; the last two are skipped since
// nothing is validated by a CA.
func runHTTPSChecks(ctx context.Context, name string, req HTTPSCheckRequest) []HTTPSCheck {
	onboarding := &site.HTTPSOnboarding{
		Name: name,
		Request: site.HTTPSRequest{
			Domains:         req.Domains,
			ChallengeMethod: req.ChallengeMethod,
			ACMEUserID:      req.ACMEUserID,
			CertificateID:   req.CertificateID,
		},
		Normalize:         normalizeHTTPSIdentifiers,
		ChallengeLocation: httpsCheckChallengeLocation,
		LoadCertificate:   loadHTTPSCertificate,
	}
	existing := onboarding.Request.UsesExistingCertificate()

	preview, err := onboarding.Preview()
	if err != nil {
		checks := []HTTPSCheck{errorCheck(httpsCheckConfigParse, err)}
		if existing {
			checks = append(checks, skippedCheck(httpsCheckCertificate, httpsCheckConfigBlockedMessage))
		}
		return append(checks,
			skippedCheck(httpsCheckChallengeRoute, httpsCheckConfigBlockedMessage),
			skippedCheck(httpsCheckDNS, httpsCheckConfigBlockedMessage),
		)
	}

	if existing {
		return []HTTPSCheck{
			configParseCheck(preview.Plan),
			certificateCheck(preview.Certificate),
			skippedCheck(httpsCheckChallengeRoute, httpsCheckExistingCertMessage),
			skippedCheck(httpsCheckDNS, httpsCheckExistingCertMessage),
		}
	}

	return []HTTPSCheck{
		configParseCheck(preview.Plan),
		challengeRouteCheck(ctx, name, preview),
		dnsCheck(ctx, preview, req.ACMEUserID),
	}
}

// httpsCheckChallengeLocation overrides the HTTP-01 location template in
// tests. Nil uses the letsencrypt.conf block template.
var httpsCheckChallengeLocation *nginx.NgxLocation

func configParseCheck(plan *site.HTTPSPlan) HTTPSCheck {
	if len(plan.Diagnostics) == 0 {
		return HTTPSCheck{
			Code:    httpsCheckConfigParse,
			Status:  site.HTTPSStatusSuccess,
			Message: httpsCheckConfigOKMessage.Message,
		}
	}
	check := diagnosticsCheck(httpsCheckConfigParse, plan.Diagnostics)
	if check.Status == site.HTTPSStatusSuccess {
		check.Message = httpsCheckConfigOKMessage.Message
	}
	return check
}

func challengeRouteCheck(ctx context.Context, name string, preview *site.HTTPSPreview) HTTPSCheck {
	plan := preview.Plan
	if plan.Request.ChallengeMethod != site.HTTPSChallengeHTTP01 {
		return skippedCheck(httpsCheckChallengeRoute, httpsCheckRouteDNSMessage)
	}
	if preview.Status != site.StatusEnabled || !site.RoutesHTTP01Challenge(preview.Live, plan.Request.Domains) {
		return skippedCheck(httpsCheckChallengeRoute, httpsCheckRouteAfterStageMsg)
	}

	pctx, cancel := context.WithTimeout(ctx, httpsCheckProbeTimeout)
	defer cancel()
	result, err := probeHTTPSChallengeRoute(pctx, name, plan.Request.Domains)
	if err != nil {
		return errorCheck(httpsCheckChallengeRoute, err)
	}
	return probeResultCheck(result)
}

// certificateCheck reports whether the existing certificate is usable and
// which requested domains it covers. Params always carry name, not_after
// and uncovered when they are known.
func certificateCheck(review *site.HTTPSCertificateReview) HTTPSCheck {
	if review == nil {
		return skippedCheck(httpsCheckCertificate, httpsCheckConfigBlockedMessage)
	}

	params := make(map[string]any)
	for key, value := range review.Params() {
		params[key] = value
	}

	if review.Err != nil {
		check := errorCheck(httpsCheckCertificate, review.Err)
		for key, value := range check.Params {
			params[key] = value
		}
		check.Params = params
		return check
	}

	check := HTTPSCheck{
		Code:    httpsCheckCertificate,
		Status:  site.HTTPSStatusSuccess,
		Message: httpsCheckCertificateOKMessage.Message,
		Params:  params,
	}
	if len(review.Uncovered) > 0 {
		check.Status = site.HTTPSStatusWarning
		check.Message = httpsCheckCertificatePartialMsg.Message
	}
	return check
}

// probeResultCheck converts a successful probe outcome into a check.
func probeResultCheck(result site.HTTPSProbeResult) HTTPSCheck {
	check := HTTPSCheck{
		Code:    httpsCheckChallengeRoute,
		Status:  result.Status,
		Message: result.Message,
	}
	switch check.Status {
	case site.HTTPSStatusWarning, site.HTTPSStatusSkipped:
	default:
		check.Status = site.HTTPSStatusSuccess
	}
	if check.Message == "" {
		check.Message = httpsCheckRouteReachableMessage.Message
	}
	return check
}

func dnsCheck(ctx context.Context, preview *site.HTTPSPreview, acmeUserID uint64) HTTPSCheck {
	plan := preview.Plan
	dctx, cancel := context.WithTimeout(ctx, httpsCheckDiagnoseTimeout)
	defer cancel()

	diagnostics := diagnoseHTTPS(dctx, plan.Request.Domains, plan.StagedListensIPv6(), acmeUserID)
	if plan.Request.ChallengeMethod == site.HTTPSChallengeDNS01 {
		// The address records do not matter for DNS-01; only a CAA record
		// can still block the issuance.
		diagnostics = filterDiagnostics(diagnostics, acmehint.CodeCAABlocksCA)
	}
	return dnsCheckFromDiagnostics(diagnostics)
}

// dnsCheckFromDiagnostics aggregates DNS diagnostics into one check: the worst
// level decides the status and the messages of that level are joined.
func dnsCheckFromDiagnostics(diagnostics []site.HTTPSDiagnostic) HTTPSCheck {
	if len(diagnostics) == 0 {
		return HTTPSCheck{
			Code:    httpsCheckDNS,
			Status:  site.HTTPSStatusSuccess,
			Message: httpsCheckDNSOKMessage.Message,
		}
	}
	return diagnosticsCheck(httpsCheckDNS, diagnostics)
}

// diagnosticsCheck builds a check from non-empty diagnostics.
func diagnosticsCheck(code string, diagnostics []site.HTTPSDiagnostic) HTTPSCheck {
	worst := 0
	for _, diagnostic := range diagnostics {
		worst = max(worst, diagnosticRank(diagnostic.Level))
	}

	var messages []string
	seen := make(map[string]bool)
	for _, diagnostic := range diagnostics {
		if diagnosticRank(diagnostic.Level) != worst || diagnostic.Message == "" || seen[diagnostic.Message] {
			continue
		}
		seen[diagnostic.Message] = true
		messages = append(messages, diagnostic.Message)
	}

	status := site.HTTPSStatusSuccess
	switch worst {
	case 1:
		status = site.HTTPSStatusWarning
	case 2:
		status = site.HTTPSStatusError
	}

	return HTTPSCheck{
		Code:    code,
		Status:  status,
		Message: strings.Join(messages, " "),
		Params: map[string]any{
			"diagnostics": append([]site.HTTPSDiagnostic(nil), diagnostics...),
		},
		Detail: formatDiagnostics(diagnostics),
	}
}

// diagnosticRank orders diagnostic levels: info < warning < error.
func diagnosticRank(level string) int {
	switch level {
	case acmehint.LevelWarning:
		return 1
	case site.HTTPSStatusError:
		return 2
	default:
		return 0
	}
}

// formatDiagnostics renders one "code (key=value, ...)" entry per diagnostic.
func formatDiagnostics(diagnostics []site.HTTPSDiagnostic) string {
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		keys := make([]string, 0, len(diagnostic.Params))
		for key := range diagnostic.Params {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, key := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, diagnostic.Params[key]))
		}
		if len(pairs) == 0 {
			parts = append(parts, diagnostic.Code)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", diagnostic.Code, strings.Join(pairs, ", ")))
	}
	return strings.Join(parts, "; ")
}

func filterDiagnostics(diagnostics []site.HTTPSDiagnostic, codes ...string) []site.HTTPSDiagnostic {
	var out []site.HTTPSDiagnostic
	for _, diagnostic := range diagnostics {
		for _, code := range codes {
			if diagnostic.Code == code {
				out = append(out, diagnostic)
				break
			}
		}
	}
	return out
}

// errorCheck reports err, with its hint when it carries one.
func errorCheck(code string, err error) HTTPSCheck {
	check := HTTPSCheck{
		Code:    code,
		Status:  site.HTTPSStatusError,
		Message: err.Error(),
	}
	var hintErr *site.HTTPSHintError
	if errors.As(err, &hintErr) {
		hint := hintErr.Hint
		check.Hint = &hint
		check.Detail = hint.Message
		if len(hint.Params) > 0 {
			check.Params = make(map[string]any, len(hint.Params))
			for key, value := range hint.Params {
				check.Params[key] = value
			}
		}
	}
	return check
}

func skippedCheck(code string, message *translation.Container) HTTPSCheck {
	return HTTPSCheck{
		Code:    code,
		Status:  site.HTTPSStatusSkipped,
		Message: message.Message,
	}
}
