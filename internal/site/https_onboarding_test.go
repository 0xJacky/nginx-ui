package site

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const onboardingSiteName = "example.com"

type onboardingFixture struct {
	confDir       string
	availablePath string
	enabledPath   string
	original      []byte

	mu     sync.Mutex
	events []HTTPSEvent

	probeCalls atomic.Int32
	issueCalls atomic.Int32
	issueReq   HTTPSIssueRequest
}

// setupOnboardingTest prepares a temporary nginx tree with the quick-setup
// site in sites-available (disabled), stubbed nginx commands and a test DB.
func setupOnboardingTest(t *testing.T, siteContent string) *onboardingFixture {
	t.Helper()
	confDir, _ := setupSiteMutationTest(t)

	db := model.UseDB()
	require.NoError(t, db.AutoMigrate(&model.Namespace{}, &model.Node{}))

	// Save and Enable replicate to sync nodes in background goroutines that
	// read the settings restored by the cleanup above. Wait until the
	// database has been quiet for a while before letting cleanup run.
	var queries atomic.Int64
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:https_onboarding_queries",
		func(*gorm.DB) { queries.Add(1) }))
	t.Cleanup(func() {
		last := queries.Load()
		stableSince := time.Now()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
			if current := queries.Load(); current != last {
				last = current
				stableSince = time.Now()
				continue
			}
			if time.Since(stableSince) >= 300*time.Millisecond {
				return
			}
		}
	})

	f := &onboardingFixture{
		confDir:       confDir,
		availablePath: filepath.Join(confDir, "sites-available", onboardingSiteName),
		enabledPath:   filepath.Join(confDir, "sites-enabled", onboardingSiteName),
		original:      []byte(siteContent),
	}
	require.NoError(t, os.WriteFile(f.availablePath, f.original, 0o644))
	return f
}

func (f *onboardingFixture) onboarding(req HTTPSRequest) *HTTPSOnboarding {
	return &HTTPSOnboarding{
		Name:              onboardingSiteName,
		Request:           req,
		ChallengeLocation: testChallengeLocation,
		Probe: func(ctx context.Context, domains []string) (HTTPSProbeResult, error) {
			f.probeCalls.Add(1)
			return HTTPSProbeResult{}, nil
		},
		Issue: func(ctx context.Context, req HTTPSIssueRequest, logf func(string, map[string]any)) (HTTPSIssueResult, error) {
			f.issueCalls.Add(1)
			f.issueReq = req
			logf("[Nginx UI] Preparing lego configurations", nil)
			return HTTPSIssueResult{
				SSLCertificate:    testCertPath,
				SSLCertificateKey: testKeyPath,
				KeyType:           "P256",
				CertID:            7,
			}, nil
		},
		Emit: func(event HTTPSEvent) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.events = append(f.events, event)
		},
	}
}

// trail renders the events as "step:status" (steps), "log", "diagnostic" or
// "done:status" so the order can be compared at a glance.
func (f *onboardingFixture) trail() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	trail := make([]string, 0, len(f.events))
	for _, event := range f.events {
		switch event.Type {
		case HTTPSEventStep:
			trail = append(trail, event.Step+":"+event.Status)
		case HTTPSEventDone:
			trail = append(trail, "done:"+event.Status)
		default:
			trail = append(trail, event.Type)
		}
	}
	return trail
}

func (f *onboardingFixture) content(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(f.availablePath)
	require.NoError(t, err)
	return string(content)
}

func (f *onboardingFixture) enabled() bool {
	_, err := os.Lstat(f.enabledPath)
	return err == nil
}

func (f *onboardingFixture) assertServesStagedHTTP(t *testing.T) {
	t.Helper()
	content := f.content(t)
	assert.Contains(t, content, "proxy_pass http://127.0.0.1:9000")
	assert.Contains(t, content, "acme-challenge")
	assert.NotContains(t, content, "https://$host")
	assert.NotContains(t, content, "ssl_certificate")
	assert.NotContains(t, content, "443")
}

var http01Request = HTTPSRequest{
	Domains:             []string{"example.com"},
	ChallengeMethod:     HTTPSChallengeHTTP01,
	RedirectHTTPToHTTPS: true,
	KeyType:             "P256",
}

func TestHTTPSOnboardingSuccess(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:running", "probe:success",
		"issue:running", "log", "issue:success",
		"finalize:running", "finalize:success",
		"done:success",
	}, f.trail())
	assert.Equal(t, HTTPSStatusSuccess, done.Status)
	assert.Equal(t, testCertPath, done.SSLCertificate)
	assert.Equal(t, testKeyPath, done.SSLCertificateKey)
	assert.EqualValues(t, "P256", done.KeyType)
	assert.EqualValues(t, 7, done.CertID)

	assert.EqualValues(t, 1, f.probeCalls.Load())
	assert.Equal(t, []string{"example.com"}, f.issueReq.Domains)
	assert.Equal(t, onboardingSiteName, f.issueReq.SiteName)

	assert.True(t, f.enabled())
	content := f.content(t)
	assert.Contains(t, content, "ssl_certificate "+testCertPath)
	assert.Contains(t, content, "ssl_certificate_key "+testKeyPath)
	assert.Contains(t, content, "return 301 https://$host$request_uri")
}

func TestHTTPSOnboardingKeepsStagedConfigWhenIssueFails(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	o := f.onboarding(http01Request)
	o.Issue = func(ctx context.Context, req HTTPSIssueRequest, logf func(string, map[string]any)) (HTTPSIssueResult, error) {
		logf("acme: error: 403 :: urn:ietf:params:acme:error:unauthorized", nil)
		return HTTPSIssueResult{}, errors.New("acme: unauthorized")
	}
	o.Diagnose = func(ctx context.Context, domains []string, hasIPv6Listen bool) []HTTPSDiagnostic {
		return []HTTPSDiagnostic{{Level: "warning", Code: "dns_points_elsewhere", Message: "elsewhere",
			Params: map[string]string{"domain": domains[0]}}}
	}
	var hintDiagnostics []HTTPSDiagnostic
	o.Hint = func(step string, err error, diagnostics []HTTPSDiagnostic) *HTTPSHint {
		hintDiagnostics = diagnostics
		return &HTTPSHint{Code: "dns_points_elsewhere", Message: "hint for " + step}
	}

	done := o.Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:running", "diagnostic", "probe:success",
		"issue:running", "log", "issue:error",
		"done:error",
	}, f.trail())
	require.Len(t, hintDiagnostics, 1)
	assert.Equal(t, "example.com", hintDiagnostics[0].Params["domain"])
	assert.Equal(t, HTTPSStepIssue, done.Step)
	assert.Equal(t, "acme: unauthorized", done.Message)
	require.NotNil(t, done.Hint)
	assert.Equal(t, "dns_points_elsewhere", done.Hint.Code)
	assert.Equal(t, "hint for issue", done.Hint.Message)

	// Decision 2: the site stays enabled and serves the app over plain HTTP.
	assert.True(t, f.enabled())
	f.assertServesStagedHTTP(t)
}

func TestHTTPSOnboardingKeepsStagedConfigWhenProbeFails(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	o := f.onboarding(http01Request)
	o.Probe = func(ctx context.Context, domains []string) (HTTPSProbeResult, error) {
		return HTTPSProbeResult{}, &HTTPSHintError{
			Hint: HTTPSHint{Code: "challenge_route_unavailable", Message: "404"},
			Err:  errors.New("challenge route returned 404"),
		}
	}

	done := o.Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:running", "probe:error",
		"done:error",
	}, f.trail())
	assert.Equal(t, HTTPSStepProbe, done.Step)
	assert.Equal(t, "challenge route returned 404", done.Message)
	require.NotNil(t, done.Hint)
	assert.Equal(t, "challenge_route_unavailable", done.Hint.Code)
	assert.Zero(t, f.issueCalls.Load())
	assert.True(t, f.enabled())
	f.assertServesStagedHTTP(t)
}

func TestHTTPSOnboardingRollsBackWhenStageFails(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	// The site is disabled, so Save skips the test and Enable hits it.
	appsettings.NginxSettings.TestConfigCmd = "false"

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:error",
		"rollback:running", "rollback:success",
		"done:error",
	}, f.trail())
	assert.Equal(t, HTTPSStepStage, done.Step)
	assert.Zero(t, f.probeCalls.Load())
	assert.Zero(t, f.issueCalls.Load())

	assert.False(t, f.enabled())
	assert.Equal(t, string(f.original), f.content(t))
}

func TestHTTPSOnboardingRollsBackEnabledSiteWhenReloadFails(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	require.NoError(t, os.Symlink(f.availablePath, f.enabledPath))
	appsettings.NginxSettings.ReloadCmd = "false"

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, HTTPSStepStage, done.Step)
	assert.Contains(t, f.trail(), "rollback:running")
	assert.True(t, f.enabled())
	assert.Equal(t, string(f.original), f.content(t))
}

func TestHTTPSOnboardingRevertsToStagedWhenFinalizeFails(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	// Accept the staged configuration, reject the one with a certificate.
	appsettings.NginxSettings.TestConfigCmd = fmt.Sprintf("! grep -q ssl_certificate %q", f.availablePath)

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:running", "probe:success",
		"issue:running", "log", "issue:success",
		"finalize:running", "finalize:error",
		"rollback:running", "rollback:success",
		"done:error",
	}, f.trail())
	assert.Equal(t, HTTPSStepFinalize, done.Step)
	assert.True(t, f.enabled())
	f.assertServesStagedHTTP(t)
}

func TestHTTPSOnboardingDNS01SkipsProbe(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	req := http01Request
	req.ChallengeMethod = HTTPSChallengeDNS01
	req.DNSCredentialID = 3

	done := f.onboarding(req).Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:skipped",
		"issue:running", "log", "issue:success",
		"finalize:running", "finalize:success",
		"done:success",
	}, f.trail())
	assert.Equal(t, HTTPSStatusSuccess, done.Status)
	assert.Zero(t, f.probeCalls.Load())
	assert.Equal(t, HTTPSChallengeDNS01, f.issueReq.ChallengeMethod)
	assert.EqualValues(t, 3, f.issueReq.DNSCredentialID)
	assert.Contains(t, f.content(t), "ssl_certificate "+testCertPath)
}

func TestHTTPSOnboardingSkipsStageWhenAlreadyServing(t *testing.T) {
	// A reissue on an enabled site whose staged form equals the current one.
	f := setupOnboardingTest(t, `server {
    listen 80;
    server_name example.com;
    root /srv/app;
    location ~ /.well-known/acme-challenge {
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:9180;
    }
}
`)
	require.NoError(t, os.Symlink(f.availablePath, f.enabledPath))
	req := http01Request
	req.RedirectHTTPToHTTPS = false

	done := f.onboarding(req).Run(context.Background())

	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	assert.Contains(t, f.trail(), "stage:skipped")
	assert.Contains(t, f.content(t), "listen 443 ssl")
}

func TestHTTPSOnboardingRejectsAdvancedSite(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	require.NoError(t, query.Site.Create(&model.Site{Path: f.availablePath, Advanced: true}))

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, []string{"plan:running", "plan:error", "done:error"}, f.trail())
	assert.Equal(t, HTTPSStepPlan, done.Step)
	require.NotNil(t, done.Hint)
	assert.Equal(t, HTTPSHintAdvancedConfigUnsupported, done.Hint.Code)
	assert.Equal(t, string(f.original), f.content(t))
	assert.False(t, f.enabled())
}

func TestHTTPSOnboardingRejectsUnparseableSite(t *testing.T) {
	f := setupOnboardingTest(t, "server {\n    listen 80;\n")

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, HTTPSStepPlan, done.Step)
	require.NotNil(t, done.Hint)
	assert.Equal(t, HTTPSHintAdvancedConfigUnsupported, done.Hint.Code)
}

func TestHTTPSOnboardingRejectsRemoteDeploy(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	namespace := &model.Namespace{Name: "edge", DeployMode: model.DeployModeRemote}
	require.NoError(t, query.Namespace.Create(namespace))
	require.NoError(t, query.Site.Create(&model.Site{Path: f.availablePath, NamespaceID: namespace.ID}))

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, []string{"plan:running", "plan:error", "done:error"}, f.trail())
	require.NotNil(t, done.Hint)
	assert.Equal(t, HTTPSHintRemoteDeployUnsupported, done.Hint.Code)
	assert.Equal(t, string(f.original), f.content(t))
}

func TestHTTPSOnboardingProbeWarningAndNormalize(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	o := f.onboarding(HTTPSRequest{Domains: []string{"EXAMPLE.com"}, ChallengeMethod: HTTPSChallengeHTTP01})
	o.Normalize = func(domains []string, method string) ([]string, error) {
		assert.Equal(t, HTTPSChallengeHTTP01, method)
		return []string{"example.com"}, nil
	}
	o.Probe = func(ctx context.Context, domains []string) (HTTPSProbeResult, error) {
		return HTTPSProbeResult{Status: HTTPSStatusWarning, Message: "example.com: cannot verify locally"}, nil
	}

	done := o.Run(context.Background())

	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	assert.Contains(t, f.trail(), "probe:warning")
	assert.Equal(t, []string{"example.com"}, f.issueReq.Domains)
}

func TestHTTPSOnboardingNormalizeFailureIsAPlanError(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	o := f.onboarding(http01Request)
	o.Normalize = func([]string, string) ([]string, error) {
		return nil, errors.New("invalid certificate identifier: -bad-")
	}

	done := o.Run(context.Background())

	assert.Equal(t, []string{"plan:running", "plan:error", "done:error"}, f.trail())
	assert.Equal(t, HTTPSStepPlan, done.Step)
	assert.Equal(t, string(f.original), f.content(t))
}

// stubResyncSiteSave records the replications restoreOriginal requests
// instead of contacting sync nodes.
func stubResyncSiteSave(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	var calls []string
	previous := resyncSiteSave
	resyncSiteSave = func(name, content string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, name+"\n"+content)
	}
	t.Cleanup(func() { resyncSiteSave = previous })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), calls...)
	}
}

func TestHTTPSOnboardingStageRollbackResyncsOriginalToNodes(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	resyncs := stubResyncSiteSave(t)
	// The site is disabled: Save skips nginx -t and replicates the staged
	// content, then Enable hits the failing test.
	appsettings.NginxSettings.TestConfigCmd = "false"

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, HTTPSStepStage, done.Step)
	assert.Contains(t, f.trail(), "rollback:success")
	assert.Equal(t, string(f.original), f.content(t))
	assert.Equal(t, []string{onboardingSiteName + "\n" + string(f.original)}, resyncs())
}

func TestHTTPSOnboardingStageRollbackSkipsResyncWhenSaveFailed(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	resyncs := stubResyncSiteSave(t)
	require.NoError(t, os.Symlink(f.availablePath, f.enabledPath))
	// Save itself fails, so nothing was replicated and nothing is re-synced.
	appsettings.NginxSettings.TestConfigCmd = "false"

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, HTTPSStepStage, done.Step)
	assert.Equal(t, string(f.original), f.content(t))
	assert.Empty(t, resyncs())
}

func TestHTTPSOnboardingRejectsConcurrentRunOnSameSite(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)

	first := f.onboarding(http01Request)
	issuing := make(chan struct{})
	proceed := make(chan struct{})
	var proceedOnce sync.Once
	unblock := func() { proceedOnce.Do(func() { close(proceed) }) }
	// Never leave the first run blocked when an assertion below fails.
	defer unblock()
	issue := first.Issue
	first.Issue = func(ctx context.Context, req HTTPSIssueRequest, logf func(string, map[string]any)) (HTTPSIssueResult, error) {
		close(issuing)
		<-proceed
		return issue(ctx, req, logf)
	}
	firstDone := make(chan HTTPSEvent, 1)
	go func() { firstDone <- first.Run(context.Background()) }()
	<-issuing

	// A second run on the same site fails fast at plan and writes nothing.
	stagedContent := f.content(t)
	var secondEvents []HTTPSEvent
	second := f.onboarding(http01Request)
	second.Emit = func(event HTTPSEvent) { secondEvents = append(secondEvents, event) }
	done := second.Run(context.Background())
	assert.Equal(t, HTTPSStatusError, done.Status)
	assert.Equal(t, HTTPSStepPlan, done.Step)
	require.NotNil(t, done.Hint)
	assert.Equal(t, HTTPSHintOnboardingInProgress, done.Hint.Code)
	require.Len(t, secondEvents, 3)
	assert.Equal(t, stagedContent, f.content(t))

	// The read-only pre-flight check is not blocked by the running onboarding.
	preview, err := f.onboarding(http01Request).Preview()
	require.NoError(t, err)
	assert.NotNil(t, preview.Plan)

	unblock()
	firstResult := <-firstDone
	assert.Equal(t, HTTPSStatusSuccess, firstResult.Status, firstResult.Message)

	// The lock is released once the first run is done.
	release, ok := tryAcquireHTTPSOnboarding(f.availablePath)
	require.True(t, ok)
	release()
}

func TestHTTPSOnboardingFinalizeRefusesConfigChangedMidRun(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	edited := "server {\n    listen 80;\n    server_name example.com;\n    root /srv/edited;\n}\n"
	o := f.onboarding(http01Request)
	issue := o.Issue
	o.Issue = func(ctx context.Context, req HTTPSIssueRequest, logf func(string, map[string]any)) (HTTPSIssueResult, error) {
		// Somebody saves the site while the certificate is being issued.
		require.NoError(t, os.WriteFile(f.availablePath, []byte(edited), 0o644))
		return issue(ctx, req, logf)
	}

	done := o.Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:running", "probe:success",
		"issue:running", "log", "issue:success",
		"finalize:running", "finalize:error",
		"done:error",
	}, f.trail())
	assert.Equal(t, HTTPSStepFinalize, done.Step)
	require.NotNil(t, done.Hint)
	assert.Equal(t, HTTPSHintConfigChangedDuringOnboarding, done.Hint.Code)
	assert.Equal(t, edited, f.content(t))
}

func TestHTTPSOnboardingPlanHintReachesDoneEvent(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	req := http01Request
	req.Domains = []string{"other.example.net"}

	done := f.onboarding(req).Run(context.Background())

	assert.Equal(t, []string{"plan:running", "plan:error", "done:error"}, f.trail())
	assert.Equal(t, HTTPSStepPlan, done.Step)
	require.NotNil(t, done.Hint)
	assert.Equal(t, HTTPSHintDomainsNotInServerName, done.Hint.Code)
	assert.Equal(t, string(f.original), f.content(t))
}
