package site

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plainHTTPDraftConfig is what the site wizard saves as a disabled draft when
// the site has no TLS server at all.
const plainHTTPDraftConfig = `server {
    listen 80;
    listen [::]:80;
    server_name example.com;
    location / {
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:9000;
    }
}
`

const (
	existingCertPath = "/etc/nginx/ssl/existing/fullchain.cer"
	existingKeyPath  = "/etc/nginx/ssl/existing/private.key"
)

// stubExistingCertificate returns a loader for a certificate covering names.
func stubExistingCertificate(names ...string) func(uint64) (*HTTPSExistingCertificate, error) {
	return func(id uint64) (*HTTPSExistingCertificate, error) {
		return &HTTPSExistingCertificate{
			ID:                id,
			Name:              "Imported example",
			SSLCertificate:    existingCertPath,
			SSLCertificateKey: existingKeyPath,
			KeyType:           "2048",
			NotAfter:          time.Now().Add(30 * 24 * time.Hour),
			Covers: func(identifier string) bool {
				for _, name := range names {
					if name == identifier {
						return true
					}
				}
				return false
			},
		}, nil
	}
}

func existingCertRequest(domains ...string) HTTPSRequest {
	return HTTPSRequest{
		Domains:             domains,
		ChallengeMethod:     HTTPSChallengeHTTP01,
		RedirectHTTPToHTTPS: true,
		KeyType:             "P256",
		Profile:             "shortlived",
		CertificateID:       5,
	}
}

func (f *onboardingFixture) eventsOf(eventType string) []HTTPSEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	var events []HTTPSEvent
	for _, event := range f.events {
		if event.Type == eventType {
			events = append(events, event)
		}
	}
	return events
}

func (f *onboardingFixture) stepEvent(t *testing.T, step string) HTTPSEvent {
	t.Helper()
	var found []HTTPSEvent
	for _, event := range f.eventsOf(HTTPSEventStep) {
		if event.Step == step {
			found = append(found, event)
		}
	}
	require.NotEmpty(t, found, "no %s step event", step)
	return found[len(found)-1]
}

func finalServers(t *testing.T, content string) (tls []*nginx.NgxServer, plain []*nginx.NgxServer) {
	t.Helper()
	cfg, err := nginx.ParseNgxConfigByContent(content)
	require.NoError(t, err)
	for _, server := range cfg.Servers {
		isTLS, _ := serverListenKinds(server)
		if isTLS {
			tls = append(tls, server)
		} else {
			plain = append(plain, server)
		}
	}
	return tls, plain
}

func TestHTTPSOnboardingExistingCertificate(t *testing.T) {
	f := setupOnboardingTest(t, quickSetupRedirectConfig)
	o := f.onboarding(existingCertRequest("example.com"))
	o.LoadCertificate = stubExistingCertificate("example.com")

	done := o.Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:skipped",
		"issue:skipped",
		"finalize:running", "finalize:success",
		"done:success",
	}, f.trail())
	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	assert.Equal(t, existingCertPath, done.SSLCertificate)
	assert.Equal(t, existingKeyPath, done.SSLCertificateKey)
	assert.EqualValues(t, "2048", done.KeyType, "the key type comes from the certificate, not the request")
	assert.EqualValues(t, 5, done.CertID)
	assert.Empty(t, done.Profile)

	assert.Equal(t, "An existing certificate is used", f.stepEvent(t, HTTPSStepProbe).Message)
	issue := f.stepEvent(t, HTTPSStepIssue)
	assert.Equal(t, "Using the existing certificate %{name}", issue.Message)
	assert.Equal(t, map[string]any{"name": "Imported example"}, issue.Args)

	assert.Zero(t, f.probeCalls.Load())
	assert.Zero(t, f.issueCalls.Load())
	assert.Empty(t, f.eventsOf(HTTPSEventLog))

	assert.True(t, f.enabled())
	content := f.content(t)
	assert.Contains(t, content, "ssl_certificate "+existingCertPath)
	assert.Contains(t, content, "ssl_certificate_key "+existingKeyPath)
	assert.Contains(t, content, "return 301 https://$host$request_uri")
	assert.Contains(t, content, "proxy_pass http://127.0.0.1:9000")
}

func TestHTTPSOnboardingExistingCertificateOnPlainHTTPDraft(t *testing.T) {
	f := setupOnboardingTest(t, plainHTTPDraftConfig)
	o := f.onboarding(existingCertRequest("example.com"))
	o.LoadCertificate = stubExistingCertificate("example.com")

	done := o.Run(context.Background())

	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	assert.True(t, f.enabled())
	content := f.content(t)
	// Nothing is validated, so no challenge route is installed.
	assert.NotContains(t, content, "acme-challenge")

	tls, plain := finalServers(t, content)
	require.Len(t, tls, 1, content)
	assert.Equal(t, []string{"example.com"}, serverNameList(tls[0]))
	assert.Equal(t, []string{existingCertPath}, directiveValues(tls[0], "ssl_certificate"))
	root, ok := rootLocationContent(tls[0])
	require.True(t, ok)
	assert.Contains(t, root, "proxy_pass http://127.0.0.1:9000", "the TLS server serves the app of the port-80 server")
	require.Len(t, plain, 1)
	root, ok = rootLocationContent(plain[0])
	require.True(t, ok)
	assert.True(t, contentRedirectsOnlyToHTTPS(root), "port 80 redirects: %q", root)
}

func TestHTTPSOnboardingIssuesCertificateForPlainHTTPDraft(t *testing.T) {
	f := setupOnboardingTest(t, plainHTTPDraftConfig)

	done := f.onboarding(http01Request).Run(context.Background())

	assert.Equal(t, []string{
		"plan:running", "plan:success",
		"stage:running", "stage:success",
		"probe:running", "probe:success",
		"issue:running", "log", "issue:success",
		"finalize:running", "finalize:success",
		"done:success",
	}, f.trail())
	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	assert.True(t, f.enabled())

	tls, plain := finalServers(t, f.content(t))
	require.Len(t, tls, 1)
	assert.Equal(t, []string{"example.com"}, serverNameList(tls[0]))
	assert.Equal(t, []string{testCertPath}, directiveValues(tls[0], "ssl_certificate"))
	root, ok := rootLocationContent(tls[0])
	require.True(t, ok)
	assert.Contains(t, root, "proxy_pass http://127.0.0.1:9000")
	assert.Equal(t, 1, challengeCount(tls[0]))
	require.Len(t, plain, 1)
	assertCanonicalRedirect(t, plain[0])
}

func TestHTTPSOnboardingIssueFailureOnPlainHTTPDraftKeepsServingHTTP(t *testing.T) {
	f := setupOnboardingTest(t, plainHTTPDraftConfig)
	o := f.onboarding(http01Request)
	o.Issue = func(context.Context, HTTPSIssueRequest, func(string, map[string]any)) (HTTPSIssueResult, error) {
		return HTTPSIssueResult{}, assert.AnError
	}

	done := o.Run(context.Background())

	assert.Equal(t, HTTPSStepIssue, done.Step)
	assert.True(t, f.enabled())
	f.assertServesStagedHTTP(t)
}

func TestHTTPSOnboardingExistingCertificatePartialCoverage(t *testing.T) {
	f := setupOnboardingTest(t, `server {
    listen 80;
    server_name example.com www.example.com;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`)
	o := f.onboarding(existingCertRequest("example.com", "www.example.com"))
	o.LoadCertificate = stubExistingCertificate("example.com")

	done := o.Run(context.Background())

	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	var coverage *HTTPSEvent
	for _, event := range f.eventsOf(HTTPSEventDiagnostic) {
		if event.Code == HTTPSDiagnosticNamesNotInCertificate && event.Params["uncovered"] != "" {
			coverage = &event
			break
		}
	}
	require.NotNil(t, coverage, "no names_not_in_certificate diagnostic for the uncovered domain")
	assert.Equal(t, "warning", coverage.Level)
	assert.Equal(t, "www.example.com", coverage.Params["uncovered"])
	assert.Equal(t, "Imported example", coverage.Params["name"])
	assert.NotEmpty(t, coverage.Params["not_after"])

	// The diagnostic comes before the plan step succeeds.
	trail := f.trail()
	assert.Less(t, indexOf(trail, "diagnostic"), indexOf(trail, "plan:success"))

	tls, plain := finalServers(t, f.content(t))
	require.Len(t, tls, 1)
	assert.Equal(t, []string{"example.com"}, serverNameList(tls[0]), "the uncovered name stays out of the TLS server")
	require.Len(t, plain, 1)
	root, ok := rootLocationContent(plain[0])
	require.True(t, ok)
	assert.Contains(t, root, "proxy_pass", "the port-80 server keeps serving the uncovered name")
}

func indexOf(values []string, value string) int {
	for i, candidate := range values {
		if candidate == value {
			return i
		}
	}
	return -1
}

func TestHTTPSOnboardingExistingCertificateRefusals(t *testing.T) {
	for _, test := range []struct {
		name string
		load func(uint64) (*HTTPSExistingCertificate, error)
		code string
	}{
		{
			name: "expired",
			load: func(id uint64) (*HTTPSExistingCertificate, error) {
				existing, err := stubExistingCertificate("example.com")(id)
				existing.NotAfter = time.Now().Add(-time.Hour)
				return existing, err
			},
			code: HTTPSHintCertificateExpired,
		},
		{
			name: "no requested domain covered",
			load: stubExistingCertificate("other.example.net"),
			code: HTTPSHintCertificateDoesNotCoverDomains,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := setupOnboardingTest(t, plainHTTPDraftConfig)
			o := f.onboarding(existingCertRequest("example.com"))
			o.LoadCertificate = test.load

			done := o.Run(context.Background())

			assert.Equal(t, []string{"plan:running", "plan:error", "done:error"}, f.trail())
			assert.Equal(t, HTTPSStepPlan, done.Step)
			require.NotNil(t, done.Hint)
			assert.Equal(t, test.code, done.Hint.Code)
			assert.Equal(t, "Imported example", done.Hint.Params["name"])
			assert.Contains(t, done.Hint.Message, "%{name}")
			assert.Equal(t, string(f.original), f.content(t))
			assert.False(t, f.enabled())
		})
	}
}

func TestHTTPSOnboardingExistingCertificateNormalizesIdentifiersOneByOne(t *testing.T) {
	f := setupOnboardingTest(t, `server {
    listen 80;
    server_name example.com 192.0.2.10;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
`)
	o := f.onboarding(existingCertRequest("*.example.com", "192.0.2.10", "example.com"))
	o.Request.ChallengeMethod = HTTPSChallengeDNS01
	var calls [][]string
	o.Normalize = func(domains []string, method string) ([]string, error) {
		// ACME refuses an IP identifier next to a wildcard or with DNS-01.
		assert.Equal(t, HTTPSChallengeHTTP01, method)
		calls = append(calls, domains)
		return domains, nil
	}
	o.LoadCertificate = stubExistingCertificate("*.example.com", "192.0.2.10", "example.com")

	done := o.Run(context.Background())

	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	assert.Equal(t, [][]string{{"*.example.com"}, {"192.0.2.10"}, {"example.com"}}, calls)
}

// migrateCertTables adds the certificate tables to the onboarding test DB.
func migrateCertTables(t *testing.T) {
	t.Helper()
	require.NoError(t, model.UseDB().AutoMigrate(&model.AcmeUser{}, &model.DnsCredential{}, &model.Cert{}))
}

func TestHTTPSOnboardingExistingCertificateFromCertificateManager(t *testing.T) {
	f := setupOnboardingTest(t, plainHTTPDraftConfig)
	migrateCertTables(t)

	certPEM, keyPEM, err := cert.GenerateSelfSigned(cert.SelfSignedOptions{
		CommonName:  "example.com",
		DNSNames:    []string{"*.example.com"},
		IPAddresses: []string{"192.0.2.10"},
		KeyType:     "P256",
	})
	require.NoError(t, err)
	sslDir := filepath.Join(f.confDir, "ssl", "wildcard")
	require.NoError(t, os.MkdirAll(sslDir, 0o755))
	certPath := filepath.Join(sslDir, "fullchain.cer")
	keyPath := filepath.Join(sslDir, "private.key")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	record := &model.Cert{
		Name:                  "Wildcard",
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		AutoCert:              model.AutoCertDisabled,
	}
	require.NoError(t, query.Cert.Create(record))
	before, err := query.Cert.Where(query.Cert.ID.Eq(record.ID)).First()
	require.NoError(t, err)

	req := existingCertRequest("www.example.com", "example.com")
	req.CertificateID = record.ID
	f.original = []byte(strings.ReplaceAll(plainHTTPDraftConfig,
		"server_name example.com;", "server_name www.example.com example.com;"))
	require.NoError(t, os.WriteFile(f.availablePath, f.original, 0o644))
	o := f.onboarding(req)

	done := o.Run(context.Background())

	assert.Equal(t, HTTPSStatusSuccess, done.Status, done.Message)
	assert.Equal(t, certPath, done.SSLCertificate)
	assert.Equal(t, keyPath, done.SSLCertificateKey)
	assert.EqualValues(t, "P256", done.KeyType, "a record without key type falls back to the certificate's")
	assert.Equal(t, record.ID, done.CertID)
	assert.Contains(t, f.stepEvent(t, HTTPSStepIssue).Args, "name")

	// The wildcard does not cover the apex; it stays on plain HTTP.
	tls, _ := finalServers(t, f.content(t))
	require.Len(t, tls, 1)
	assert.Equal(t, []string{"www.example.com"}, serverNameList(tls[0]))

	// The cert record is read, never written.
	after, err := query.Cert.Where(query.Cert.ID.Eq(record.ID)).First()
	require.NoError(t, err)
	assert.Equal(t, model.AutoCertDisabled, after.AutoCert)
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt)
	assert.Equal(t, before.KeyType, after.KeyType)
}

func TestLoadHTTPSExistingCertificateErrors(t *testing.T) {
	f := setupOnboardingTest(t, plainHTTPDraftConfig)
	migrateCertTables(t)

	hintCode := func(err error) string {
		var hintErr *HTTPSHintError
		require.ErrorAs(t, err, &hintErr)
		return hintErr.Hint.Code
	}

	_, err := LoadHTTPSExistingCertificate(404)
	assert.Equal(t, HTTPSHintCertificateNotFound, hintCode(err))

	noPaths := &model.Cert{Name: "No paths"}
	require.NoError(t, query.Cert.Create(noPaths))
	_, err = LoadHTTPSExistingCertificate(noPaths.ID)
	assert.Equal(t, HTTPSHintCertificateFilesMissing, hintCode(err))

	missing := &model.Cert{
		Domains:               []string{"example.com"},
		SSLCertificatePath:    filepath.Join(f.confDir, "ssl", "gone", "fullchain.cer"),
		SSLCertificateKeyPath: filepath.Join(f.confDir, "ssl", "gone", "private.key"),
	}
	require.NoError(t, query.Cert.Create(missing))
	_, err = LoadHTTPSExistingCertificate(missing.ID)
	assert.Equal(t, HTTPSHintCertificateFilesMissing, hintCode(err))
	var hintErr *HTTPSHintError
	require.ErrorAs(t, err, &hintErr)
	assert.Equal(t, "example.com", hintErr.Hint.Params["name"], "a record without name is named after its first domain")

	// The run reports the loader's hint at the plan step.
	o := f.onboarding(existingCertRequest("example.com"))
	o.Request.CertificateID = 404
	done := o.Run(context.Background())
	assert.Equal(t, HTTPSStepPlan, done.Step)
	require.NotNil(t, done.Hint)
	assert.Equal(t, HTTPSHintCertificateNotFound, done.Hint.Code)
	assert.False(t, f.enabled())
}

func TestHTTPSPreviewReportsCertificateWithoutFailing(t *testing.T) {
	f := setupOnboardingTest(t, plainHTTPDraftConfig)

	t.Run("usable", func(t *testing.T) {
		o := f.onboarding(existingCertRequest("example.com"))
		o.LoadCertificate = stubExistingCertificate("example.com")
		preview, err := o.Preview()
		require.NoError(t, err)
		require.NotNil(t, preview.Certificate)
		assert.NoError(t, preview.Certificate.Err)
		assert.Equal(t, []string{"example.com"}, preview.Certificate.Covered)
		assert.Equal(t, HTTPSChallengeDNS01, preview.Plan.Request.ChallengeMethod)
	})

	t.Run("unusable", func(t *testing.T) {
		o := f.onboarding(existingCertRequest("example.com"))
		o.LoadCertificate = stubExistingCertificate("other.example.net")
		preview, err := o.Preview()
		require.NoError(t, err)
		require.NotNil(t, preview.Certificate)
		require.Error(t, preview.Certificate.Err)
		assert.Equal(t, []string{"example.com"}, preview.Certificate.Uncovered)
		// The configuration is still planned for every requested domain.
		assert.Equal(t, []string{"example.com"}, preview.Plan.Request.Domains)
	})

	t.Run("issuance", func(t *testing.T) {
		preview, err := f.onboarding(http01Request).Preview()
		require.NoError(t, err)
		assert.Nil(t, preview.Certificate)
	})

	assert.Equal(t, string(f.original), f.content(t))
	assert.False(t, f.enabled())
}
