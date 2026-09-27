package site

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v5/certcrypto"
	"gorm.io/gorm"
)

// HTTPS onboarding event types.
const (
	HTTPSEventStep       = "step"
	HTTPSEventLog        = "log"
	HTTPSEventDiagnostic = "diagnostic"
	HTTPSEventDone       = "done"
)

// HTTPS onboarding steps.
const (
	HTTPSStepPlan     = "plan"
	HTTPSStepStage    = "stage"
	HTTPSStepProbe    = "probe"
	HTTPSStepIssue    = "issue"
	HTTPSStepFinalize = "finalize"
	HTTPSStepRollback = "rollback"
)

// HTTPS onboarding statuses.
const (
	HTTPSStatusRunning = "running"
	HTTPSStatusSuccess = "success"
	HTTPSStatusWarning = "warning"
	HTTPSStatusError   = "error"
	HTTPSStatusSkipped = "skipped"
)

// Stable hint codes produced by the orchestrator itself.
const (
	HTTPSHintAdvancedConfigUnsupported = "advanced_config_unsupported"
	HTTPSHintRemoteDeployUnsupported   = "remote_deploy_unsupported"
	HTTPSHintSiteInMaintenance         = "site_in_maintenance"
	// HTTPSHintOnboardingInProgress: another onboarding run holds the site.
	HTTPSHintOnboardingInProgress = "onboarding_in_progress"
	// HTTPSHintConfigChangedDuringOnboarding: the site file no longer holds
	// what this run wrote, so finalize refuses to overwrite it.
	HTTPSHintConfigChangedDuringOnboarding = "config_changed_during_onboarding"
)

// httpsOnboardingRuns serializes onboarding runs per site. It is a set of the
// sites with a run in flight rather than a map of mutexes so entries can be
// dropped on release without racing a concurrent acquire, and so a second
// run fails fast instead of queueing behind a certificate issuance.
var httpsOnboardingRuns = struct {
	sync.Mutex
	active map[string]struct{}
}{active: map[string]struct{}{}}

// tryAcquireHTTPSOnboarding claims key for one onboarding run. It returns false
// when another run holds it.
func tryAcquireHTTPSOnboarding(key string) (release func(), ok bool) {
	httpsOnboardingRuns.Lock()
	defer httpsOnboardingRuns.Unlock()
	if _, busy := httpsOnboardingRuns.active[key]; busy {
		return nil, false
	}
	httpsOnboardingRuns.active[key] = struct{}{}
	var once sync.Once
	return func() {
		once.Do(func() {
			httpsOnboardingRuns.Lock()
			defer httpsOnboardingRuns.Unlock()
			delete(httpsOnboardingRuns.active, key)
		})
	}, true
}

// resyncSiteSave replicates a site configuration to the site's sync nodes, the
// same way Save does after a successful write. It is a variable so tests can
// observe the replication without real nodes.
var resyncSiteSave = func(name, content string) {
	go syncSave(name, content)
}

// HTTPSRequest is the message a client sends to start an HTTPS onboarding run.
type HTTPSRequest struct {
	Domains                           []string           `json:"domains"`
	ChallengeMethod                   string             `json:"challenge_method"`
	DNSCredentialID                   uint64             `json:"dns_credential_id"`
	RedirectHTTPToHTTPS               bool               `json:"redirect_http_to_https"`
	KeyType                           certcrypto.KeyType `json:"key_type"`
	ACMEUserID                        uint64             `json:"acme_user_id"`
	Profile                           string             `json:"profile"`
	MustStaple                        bool               `json:"must_staple"`
	LegoDisableCNAMESupport           bool               `json:"lego_disable_cname_support"`
	DisableAuthoritativeNSPropagation bool               `json:"disable_authoritative_ns_propagation"`
	EnableCommonName                  bool               `json:"enable_common_name"`
	RevokeOld                         bool               `json:"revoke_old"`
	// CertificateID selects a certificate from the certificate manager to
	// install instead of issuing one. 0 issues a new certificate; the ACME
	// fields and the challenge method are then ignored.
	CertificateID uint64 `json:"certificate_id"`
}

// HTTPSIssueRequest is handed to the Issue dependency. Domains and
// ChallengeMethod are normalized by the planner.
type HTTPSIssueRequest struct {
	SiteName string
	HTTPSRequest
}

// HTTPSIssueResult is what the Issue dependency returns on success.
type HTTPSIssueResult struct {
	SSLCertificate    string
	SSLCertificateKey string
	KeyType           certcrypto.KeyType
	Profile           string
	CertID            uint64
}

// HTTPSHint is an actionable explanation attached to a failure.
type HTTPSHint = acmehint.Hint

// HTTPSHintError carries a hint together with the error it explains.
type HTTPSHintError struct {
	Hint HTTPSHint
	Err  error
}

func (e *HTTPSHintError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Hint.Message
}

func (e *HTTPSHintError) Unwrap() error {
	return e.Err
}

// HTTPSEvent is one message of the onboarding stream.
type HTTPSEvent struct {
	Type    string `json:"type"`
	Step    string `json:"step,omitempty"`
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	// Args are the translation arguments of Message, when it is a template.
	Args map[string]any `json:"args,omitempty"`

	// Diagnostic fields.
	Level  string            `json:"level,omitempty"`
	Code   string            `json:"code,omitempty"`
	Params map[string]string `json:"params,omitempty"`

	// Done fields.
	SSLCertificate    string             `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string             `json:"ssl_certificate_key,omitempty"`
	KeyType           certcrypto.KeyType `json:"key_type,omitempty"`
	Profile           string             `json:"profile,omitempty"`
	CertID            uint64             `json:"cert_id,omitempty"`
	Hint              *HTTPSHint         `json:"hint,omitempty"`
}

// HTTPSOnboarding enables HTTPS on a site as one backend transaction:
// plan -> stage -> probe -> issue -> finalize, rolling back where the contract
// requires it. All external effects other than the site save path are
// injected so the flow can be tested without an ACME server.
type HTTPSOnboarding struct {
	// Name is the site name (file name in sites-available).
	Name    string
	Request HTTPSRequest

	// Normalize validates and canonicalizes the requested identifiers (for
	// example IDN to punycode) before planning. Optional.
	Normalize func(domains []string, challengeMethod string) ([]string, error)
	// Diagnose collects DNS diagnostics during the HTTP-01 probe step. They
	// are emitted as diagnostic events and never block. Optional.
	Diagnose func(ctx context.Context, domains []string, hasIPv6Listen bool) []HTTPSDiagnostic
	// Probe verifies that every domain routes the HTTP-01 challenge to this
	// instance. A returned error fails the step. Nil skips the probe step.
	Probe func(ctx context.Context, domains []string) (HTTPSProbeResult, error)
	// Issue obtains the certificate, including the cert record bookkeeping.
	// logf streams issuance log lines (translation source and arguments).
	Issue func(ctx context.Context, req HTTPSIssueRequest, logf func(message string, args map[string]any)) (HTTPSIssueResult, error)
	// Emit receives every event. Calls are serialized.
	Emit func(event HTTPSEvent)
	// Hint maps a step failure to a hint, given the diagnostics collected so
	// far. Optional; errors wrapping an *HTTPSHintError use their own hint.
	Hint func(step string, err error, diagnostics []HTTPSDiagnostic) *HTTPSHint
	// ChallengeLocation overrides the HTTP-01 location template (tests).
	ChallengeLocation *nginx.NgxLocation
	// LoadCertificate loads the certificate selected by
	// Request.CertificateID. Nil uses LoadHTTPSExistingCertificate.
	LoadCertificate func(id uint64) (*HTTPSExistingCertificate, error)

	emitMu      sync.Mutex
	diagnostics []HTTPSDiagnostic
}

// HTTPSProbeResult is the outcome of a probe that did not fail.
type HTTPSProbeResult struct {
	// Status is HTTPSStatusSuccess, HTTPSStatusWarning or HTTPSStatusSkipped.
	Status string
	// Message summarizes the probe for the step event.
	Message string
}

// httpsRunState is everything the steps need after planning.
type httpsRunState struct {
	path            string
	originalRaw     []byte
	originalSnap    configFileSnapshot
	originalBuilt   string
	wasEnabled      bool
	namespaceID     uint64
	syncNodeIDs     []uint64
	plan            *HTTPSPlan
	stagedContent   string
	stagingRequired bool
	// expectedRaw is the file content this run last wrote (or found, when it
	// wrote nothing). Finalize refuses to overwrite anything else.
	expectedRaw []byte
	// stagedSaved reports that stage saved the staged content through Save,
	// which also replicated it to the sync nodes.
	stagedSaved bool
	// existing is the certificate installed instead of issuing one.
	existing *HTTPSExistingCertificate
	// certificateDiagnostics are the warnings about the existing certificate.
	certificateDiagnostics []HTTPSDiagnostic
}

// httpsSiteState is the site as found before planning.
type httpsSiteState struct {
	path          string
	siteModel     *model.Site
	status        Status
	snapshot      configFileSnapshot
	raw           []byte
	cfg           *nginx.NgxConfig
	originalBuilt string
	// domains are the requested identifiers, normalized.
	domains []string
}

// Run executes the onboarding and returns the terminal done event, which has
// also been emitted.
func (o *HTTPSOnboarding) Run(ctx context.Context) HTTPSEvent {
	if ctx == nil {
		ctx = context.Background()
	}

	o.step(HTTPSStepPlan, HTTPSStatusRunning, translation.C("Planning the HTTPS configuration"))
	release, err := o.acquireRun()
	if err != nil {
		o.stepError(HTTPSStepPlan, err)
		return o.fail(HTTPSStepPlan, err)
	}
	defer release()

	state, err := o.prepare()
	if err != nil {
		o.stepError(HTTPSStepPlan, err)
		return o.fail(HTTPSStepPlan, err)
	}
	o.emitDiagnostics(state.certificateDiagnostics)
	o.emitDiagnostics(state.plan.Diagnostics)
	o.step(HTTPSStepPlan, HTTPSStatusSuccess, translation.C("HTTPS configuration planned"))

	// stage
	if state.stagingRequired {
		o.step(HTTPSStepStage, HTTPSStatusRunning, translation.C("Applying the HTTP configuration for validation"))
		if err := o.stage(state); err != nil {
			o.stepError(HTTPSStepStage, err)
			o.step(HTTPSStepRollback, HTTPSStatusRunning, translation.C("Restoring the original site configuration"))
			if rbErr := o.restoreOriginal(state); rbErr != nil {
				o.stepError(HTTPSStepRollback, rbErr)
				err = errors.Join(err, fmt.Errorf("rollback failed: %w", rbErr))
			} else {
				o.step(HTTPSStepRollback, HTTPSStatusSuccess, translation.C("Original site configuration restored"))
			}
			return o.fail(HTTPSStepStage, err)
		}
		o.step(HTTPSStepStage, HTTPSStatusSuccess, translation.C("HTTP configuration applied"))
	} else {
		o.step(HTTPSStepStage, HTTPSStatusSkipped, translation.C("The site already serves the required HTTP configuration"))
	}

	// probe
	switch {
	case state.existing != nil:
		o.step(HTTPSStepProbe, HTTPSStatusSkipped, httpsMessageExistingCertificateProbeSkipped)
	case state.plan.Request.ChallengeMethod != HTTPSChallengeHTTP01:
		o.step(HTTPSStepProbe, HTTPSStatusSkipped, translation.C("The DNS challenge does not use the HTTP challenge route"))
	case o.Probe == nil:
		o.step(HTTPSStepProbe, HTTPSStatusSkipped, translation.C("HTTP challenge route probe is unavailable"))
	default:
		o.step(HTTPSStepProbe, HTTPSStatusRunning, translation.C("Checking the HTTP challenge route"))
		if o.Diagnose != nil {
			o.diagnostics = o.Diagnose(ctx, state.plan.Request.Domains, state.plan.StagedListensIPv6())
			o.emitDiagnostics(o.diagnostics)
		}
		err := ctx.Err()
		var probe HTTPSProbeResult
		if err == nil {
			probe, err = o.Probe(ctx, state.plan.Request.Domains)
		}
		if err != nil {
			o.stepError(HTTPSStepProbe, err)
			return o.fail(HTTPSStepProbe, err)
		}
		status := probe.Status
		if status != HTTPSStatusWarning && status != HTTPSStatusSkipped {
			status = HTTPSStatusSuccess
		}
		message := probe.Message
		if message == "" {
			message = translation.C("HTTP challenge route is reachable").ToString()
		}
		o.emit(HTTPSEvent{Type: HTTPSEventStep, Step: HTTPSStepProbe, Status: status, Message: message})
	}

	// issue
	var result HTTPSIssueResult
	if state.existing != nil {
		o.step(HTTPSStepIssue, HTTPSStatusSkipped, translation.C("Using the existing certificate %{name}",
			map[string]any{"name": state.existing.Name}))
		result = existingCertificateResult(state.existing)
	} else {
		o.step(HTTPSStepIssue, HTTPSStatusRunning, translation.C("Issuing the certificate"))
		result, err = o.issue(ctx, state)
		if err != nil {
			o.stepError(HTTPSStepIssue, err)
			return o.fail(HTTPSStepIssue, err)
		}
		o.step(HTTPSStepIssue, HTTPSStatusSuccess, translation.C("[Nginx UI] Issued certificate successfully"))
	}

	// finalize
	o.step(HTTPSStepFinalize, HTTPSStatusRunning, translation.C("Enabling HTTPS in the site configuration"))
	if revert, err := o.finalize(state, result); err != nil {
		o.stepError(HTTPSStepFinalize, err)
		if revert != nil {
			o.step(HTTPSStepRollback, HTTPSStatusRunning, translation.C("Restoring the HTTP configuration"))
			if rbErr := revert(); rbErr != nil {
				o.stepError(HTTPSStepRollback, rbErr)
				err = errors.Join(err, fmt.Errorf("rollback failed: %w", rbErr))
			} else {
				o.step(HTTPSStepRollback, HTTPSStatusSuccess, translation.C("HTTP configuration restored"))
			}
		}
		return o.fail(HTTPSStepFinalize, err)
	}
	o.step(HTTPSStepFinalize, HTTPSStatusSuccess, translation.C("HTTPS enabled"))

	done := HTTPSEvent{
		Type:              HTTPSEventDone,
		Status:            HTTPSStatusSuccess,
		Message:           translation.C("HTTPS enabled").ToString(),
		SSLCertificate:    result.SSLCertificate,
		SSLCertificateKey: result.SSLCertificateKey,
		KeyType:           result.KeyType,
		Profile:           result.Profile,
		CertID:            result.CertID,
	}
	o.emit(done)
	return done
}

// acquireRun claims the site for this run. The key is the resolved
// sites-available path so different spellings of one site share it; an
// unresolvable name falls back to itself and fails in prepare.
func (o *HTTPSOnboarding) acquireRun() (release func(), err error) {
	key := o.Name
	if path, pathErr := ResolveAvailablePath(o.Name); pathErr == nil {
		key = path
	}
	release, ok := tryAcquireHTTPSOnboarding(key)
	if !ok {
		return nil, &HTTPSHintError{
			Hint: HTTPSHint{
				Code:    HTTPSHintOnboardingInProgress,
				Message: "HTTPS is already being enabled for this site; wait for that run to finish and try again",
			},
			Err: errors.New("another HTTPS onboarding run is in progress for this site"),
		}
	}
	return release, nil
}

// prepare runs the plan step. It has no side effects, so the pre-flight check
// (Preview) calls it without taking the per-site run lock.
func (o *HTTPSOnboarding) prepare() (*httpsRunState, error) {
	siteState, err := o.inspectSite()
	if err != nil {
		return nil, err
	}

	domains := siteState.domains
	var review *HTTPSCertificateReview
	if o.Request.UsesExistingCertificate() {
		review = o.reviewCertificate(domains)
		if review.Err != nil {
			return nil, review.Err
		}
		domains = review.Covered
	}
	return o.plan(siteState, domains, review)
}

// inspectSite runs the checks of the plan step that do not depend on the
// requested certificate: site lookup, remote-deploy, advanced-editor and
// maintenance checks, parsing and identifier normalization.
func (o *HTTPSOnboarding) inspectSite() (*httpsSiteState, error) {
	path, err := ResolveAvailablePath(o.Name)
	if err != nil {
		return nil, err
	}

	siteModel, err := lookupSiteModel(path)
	if err != nil {
		return nil, err
	}
	if siteModel != nil && siteModel.Namespace.IsRemoteDeploy() {
		return nil, &HTTPSHintError{
			Hint: HTTPSHint{
				Code:    HTTPSHintRemoteDeployUnsupported,
				Message: "HTTPS onboarding is not available for sites deployed to remote nodes only; issue the certificate on the nodes that serve the site",
			},
			Err: errors.New("site belongs to a remote-deploy namespace"),
		}
	}
	if siteModel != nil && siteModel.Advanced {
		return nil, advancedConfigError(errors.New("site uses the advanced editor"))
	}

	status := GetSiteStatus(o.Name)
	if status == StatusMaintenance {
		return nil, &HTTPSHintError{
			Hint: HTTPSHint{
				Code:    HTTPSHintSiteInMaintenance,
				Message: "Disable maintenance mode before enabling HTTPS",
			},
			Err: ErrSiteIsInMaintenance,
		}
	}

	snapshot, err := captureConfigFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := nginx.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg, err := nginx.ParseNgxConfigByContent(string(raw))
	if err != nil {
		return nil, advancedConfigError(err)
	}
	cfg.FileName = path
	cfg.Name = o.Name

	originalBuilt, err := cfg.BuildConfig()
	if err != nil {
		return nil, advancedConfigError(err)
	}

	domains, err := o.normalizeDomains()
	if err != nil {
		return nil, err
	}
	domains, err = normalizeHTTPSDomains(domains)
	if err != nil {
		return nil, err
	}

	return &httpsSiteState{
		path:          path,
		siteModel:     siteModel,
		status:        status,
		snapshot:      snapshot,
		raw:           raw,
		cfg:           cfg,
		originalBuilt: originalBuilt,
		domains:       domains,
	}, nil
}

// plan builds the staged configuration for domains. With an existing
// certificate (review not nil) the site is planned like DNS-01: no challenge
// route is installed, since nothing is validated.
func (o *HTTPSOnboarding) plan(siteState *httpsSiteState, domains []string, review *HTTPSCertificateReview) (*httpsRunState, error) {
	challengeMethod := o.Request.ChallengeMethod
	if review != nil {
		challengeMethod = HTTPSChallengeDNS01
	}

	plan, err := PlanHTTPS(siteState.cfg, HTTPSPlanRequest{
		Domains:             domains,
		ChallengeMethod:     challengeMethod,
		RedirectHTTPToHTTPS: o.Request.RedirectHTTPToHTTPS,
		SiteDisabled:        siteState.status != StatusEnabled,
		ChallengeLocation:   o.ChallengeLocation,
	})
	if err != nil {
		return nil, err
	}

	stagedContent, err := plan.Staged.BuildConfig()
	if err != nil {
		return nil, err
	}

	state := &httpsRunState{
		path:          siteState.path,
		originalRaw:   siteState.raw,
		originalSnap:  siteState.snapshot,
		originalBuilt: siteState.originalBuilt,
		wasEnabled:    siteState.status == StatusEnabled,
		plan:          plan,
		stagedContent: stagedContent,
		expectedRaw:   siteState.raw,
	}
	if siteState.siteModel != nil {
		state.namespaceID = siteState.siteModel.NamespaceID
		state.syncNodeIDs = siteState.siteModel.SyncNodeIDs
	}
	if review != nil && review.Err == nil {
		state.existing = review.Certificate
		state.certificateDiagnostics = review.Diagnostics()
	}
	state.stagingRequired = !state.wasEnabled || stagedContent != state.originalBuilt
	return state, nil
}

func lookupSiteModel(path string) (*model.Site, error) {
	s := query.Site
	siteModel, err := s.Where(s.Path.Eq(path)).Preload(s.Namespace).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return siteModel, err
}

func advancedConfigError(err error) error {
	return &HTTPSHintError{
		Hint: HTTPSHint{
			Code:    HTTPSHintAdvancedConfigUnsupported,
			Message: "The site configuration cannot be edited automatically; add the certificate in the advanced editor instead",
		},
		Err: err,
	}
}

// stage writes the staged configuration through the regular site save path
// and enables the site.
func (o *HTTPSOnboarding) stage(state *httpsRunState) error {
	if state.stagedContent != state.originalBuilt {
		if err := Save(o.Name, state.stagedContent, true, state.namespaceID, state.syncNodeIDs,
			model.PostSyncActionReloadNginx); err != nil {
			return err
		}
		state.stagedSaved = true
		state.expectedRaw = []byte(state.stagedContent)
	}
	if !state.wasEnabled {
		if err := Enable(o.Name); err != nil {
			return err
		}
	}
	return nil
}

// restoreOriginal puts back the original file content and enabled state.
// When stage had already saved the staged content, Save replicated it to the
// sync nodes (for a disabled site Save skips nginx -t, so it succeeds before
// Enable fails), and the original content is replicated again so the nodes
// do not keep the staged configuration.
func (o *HTTPSOnboarding) restoreOriginal(state *httpsRunState) error {
	restored, err := o.restoreOriginalLocally(state)
	if restored && state.stagedSaved {
		// The staged replication runs in its own goroutine, so a slow node
		// can in theory still receive it after this one; the window is the
		// Enable test and reload that failed in between.
		resyncSiteSave(o.Name, string(state.originalRaw))
	}
	return err
}

// restoreOriginalLocally restores the local file and enabled state. restored
// reports whether the original file content was written back.
func (o *HTTPSOnboarding) restoreOriginalLocally(state *httpsRunState) (restored bool, err error) {
	release := config.LockApply()
	defer release()

	current, err := nginx.ReadFile(state.path)
	if err != nil {
		return false, err
	}
	contentChanged := !bytes.Equal(current, state.originalRaw)

	if !state.wasEnabled {
		enabledPath, err := resolveEnabledSymlinkPath(o.Name)
		if err != nil {
			return false, err
		}
		if contentChanged {
			if err := state.originalSnap.Restore(state.path); err != nil {
				return false, err
			}
		}
		linked, err := nginx.Exists(enabledPath)
		if err != nil {
			return contentChanged, err
		}
		if linked {
			return contentChanged, removeEnabledLinkAndReload(enabledPath)
		}
		return contentChanged, nil
	}

	if !contentChanged {
		return false, nil
	}
	if err := restoreConfigAndReload(state.path, state.originalSnap); err != nil {
		return false, err
	}
	return true, nil
}

func (o *HTTPSOnboarding) issue(ctx context.Context, state *httpsRunState) (HTTPSIssueResult, error) {
	if err := ctx.Err(); err != nil {
		return HTTPSIssueResult{}, err
	}
	if o.Issue == nil {
		return HTTPSIssueResult{}, errors.New("certificate issuer is not configured")
	}

	req := HTTPSIssueRequest{SiteName: o.Name, HTTPSRequest: o.Request}
	req.Domains = append([]string(nil), state.plan.Request.Domains...)
	req.ChallengeMethod = state.plan.Request.ChallengeMethod

	result, err := o.Issue(ctx, req, func(message string, args map[string]any) {
		o.emit(HTTPSEvent{Type: HTTPSEventLog, Message: message, Args: args})
	})
	if err != nil {
		return HTTPSIssueResult{}, err
	}
	if strings.TrimSpace(result.SSLCertificate) == "" || strings.TrimSpace(result.SSLCertificateKey) == "" {
		return HTTPSIssueResult{}, errors.New("certificate issuer returned no certificate paths")
	}
	return result, nil
}

// finalize saves the final configuration. When the save fails it returns the
// function that puts the file back to the content it had before (the staged
// configuration, which keeps serving plain HTTP).
func (o *HTTPSOnboarding) finalize(state *httpsRunState, result HTTPSIssueResult) (revert func() error, err error) {
	final, err := state.plan.BuildFinal(result.SSLCertificate, result.SSLCertificateKey)
	if err != nil {
		return nil, err
	}
	content, err := final.BuildConfig()
	if err != nil {
		return nil, err
	}

	before, err := captureConfigFile(state.path)
	if err != nil {
		return nil, err
	}
	beforeRaw, err := nginx.ReadFile(state.path)
	if err != nil {
		return nil, err
	}
	// Refuse to overwrite an edit made while the certificate was being issued
	// (the editor, another tab, a sync from a node). Save takes the apply lock
	// itself and there is no lower-level save that runs under a caller's
	// lock, so a write landing between this read and Save's snapshot is still
	// possible; that window is a few file operations wide.
	if !bytes.Equal(beforeRaw, state.expectedRaw) {
		return nil, configChangedError()
	}

	if err := Save(o.Name, content, true, state.namespaceID, state.syncNodeIDs, model.PostSyncActionReloadNginx); err != nil {
		return func() error {
			return o.restoreStaged(state.path, before, beforeRaw)
		}, err
	}
	return nil, nil
}

func configChangedError() error {
	return &HTTPSHintError{
		Hint: HTTPSHint{
			Code:    HTTPSHintConfigChangedDuringOnboarding,
			Message: "The site configuration changed while HTTPS was being enabled; reload the page and try again",
		},
		Err: errors.New("the site configuration changed during HTTPS onboarding"),
	}
}

func (o *HTTPSOnboarding) restoreStaged(path string, snapshot configFileSnapshot, raw []byte) error {
	release := config.LockApply()
	defer release()

	current, err := nginx.ReadFile(path)
	if err != nil {
		return err
	}
	// Save restores its snapshot on a rejected test or reload, so there is
	// normally nothing left to do here.
	if bytes.Equal(current, raw) {
		return nil
	}
	return restoreConfigAndReload(path, snapshot)
}

// --- events ---

func (o *HTTPSOnboarding) emit(event HTTPSEvent) {
	if o.Emit == nil {
		return
	}
	o.emitMu.Lock()
	defer o.emitMu.Unlock()
	o.Emit(event)
}

func (o *HTTPSOnboarding) emitDiagnostics(diagnostics []HTTPSDiagnostic) {
	for _, diagnostic := range diagnostics {
		o.emit(HTTPSEvent{
			Type:    HTTPSEventDiagnostic,
			Level:   diagnostic.Level,
			Code:    diagnostic.Code,
			Message: diagnostic.Message,
			Params:  diagnostic.Params,
		})
	}
}

// step emits a step event. A message with arguments is sent as its source
// string plus Args so the client can translate it.
func (o *HTTPSOnboarding) step(step, status string, message *translation.Container) {
	event := HTTPSEvent{Type: HTTPSEventStep, Step: step, Status: status, Message: message.ToString()}
	if len(message.Args) > 0 {
		event.Message = message.Message
		event.Args = message.Args
	}
	o.emit(event)
}

func (o *HTTPSOnboarding) stepError(step string, err error) {
	o.emit(HTTPSEvent{Type: HTTPSEventStep, Step: step, Status: HTTPSStatusError, Message: err.Error()})
}

func (o *HTTPSOnboarding) hintFor(step string, err error) *HTTPSHint {
	var hintErr *HTTPSHintError
	if errors.As(err, &hintErr) {
		hint := hintErr.Hint
		return &hint
	}
	if o.Hint != nil {
		return o.Hint(step, err, o.diagnostics)
	}
	return nil
}

func (o *HTTPSOnboarding) fail(step string, err error) HTTPSEvent {
	done := HTTPSEvent{
		Type:    HTTPSEventDone,
		Status:  HTTPSStatusError,
		Step:    step,
		Message: err.Error(),
		Hint:    o.hintFor(step, err),
	}
	o.emit(done)
	return done
}
