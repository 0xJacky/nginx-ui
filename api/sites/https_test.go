package sites

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/acmehint"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const httpsTestSite = "example.com"

// httpsTestRoutedSite already routes the HTTP-01 challenge on port 80.
const httpsTestRoutedSite = `server {
    listen 80;
    server_name example.com;
    location ~ /.well-known/acme-challenge {
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:9180;
    }
    location / {
        root /var/www/html;
    }
}
`

// httpsTestPlainSite is a plain HTTP site without the challenge route.
const httpsTestPlainSite = `server {
    listen 80;
    server_name example.com;
    location / {
        root /var/www/html;
    }
}
`

var httpsTestChallengeLocation = &nginx.NgxLocation{
	Path:    "~ /.well-known/acme-challenge",
	Content: "proxy_set_header Host $host;\nproxy_pass http://127.0.0.1:9180;\n",
}

// httpsTestFixture records the calls made through the seams.
type httpsTestFixture struct {
	confDir string

	mu            sync.Mutex
	probeDomains  [][]string
	diagnoseCalls int
	diagnoseIPv6  bool
	diagnostics   []site.HTTPSDiagnostic
	probeResults  []cert.HTTP01ProbeResult
	probeErr      error
}

// setupHTTPSTest prepares a temporary nginx tree holding one site, a test
// database and stubbed probe/diagnose seams.
func setupHTTPSTest(t *testing.T, content string, enabled bool) *httpsTestFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	confDir := t.TempDir()
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}
	available := filepath.Join(confDir, "sites-available", httpsTestSite)
	require.NoError(t, os.WriteFile(available, []byte(content), 0o644))
	if enabled {
		require.NoError(t, os.Symlink(available, filepath.Join(confDir, "sites-enabled", httpsTestSite)))
	}

	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Site{}, &model.Namespace{}, &model.Node{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	f := &httpsTestFixture{
		confDir: confDir,
		probeResults: []cert.HTTP01ProbeResult{
			{Domain: httpsTestSite, Status: cert.HTTP01ProbeStatusSuccess, Target: "http://127.0.0.1:80"},
		},
		diagnostics: []site.HTTPSDiagnostic{
			{Level: acmehint.LevelInfo, Code: acmehint.CodeDNSOK, Message: acmehint.Message(acmehint.CodeDNSOK),
				Params: map[string]string{"domain": httpsTestSite, "ip": "192.0.2.10"}},
		},
	}

	previousProbe := probeHTTP01Routes
	previousDiagnose := diagnoseHTTPS
	previousLocation := httpsCheckChallengeLocation
	probeHTTP01Routes = func(ctx context.Context, domains []string, opts ...cert.HTTP01ProbeOption) ([]cert.HTTP01ProbeResult, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.probeDomains = append(f.probeDomains, append([]string(nil), domains...))
		return f.probeResults, f.probeErr
	}
	diagnoseHTTPS = func(ctx context.Context, domains []string, hasIPv6Listen bool, acmeUserID uint64) []site.HTTPSDiagnostic {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.diagnoseCalls++
		f.diagnoseIPv6 = hasIPv6Listen
		return f.diagnostics
	}
	httpsCheckChallengeLocation = httpsTestChallengeLocation

	t.Cleanup(func() {
		probeHTTP01Routes = previousProbe
		diagnoseHTTPS = previousDiagnose
		httpsCheckChallengeLocation = previousLocation
		settings.NginxSettings.ConfigDir = originalConfigDir
	})
	return f
}

func (f *httpsTestFixture) probeCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.probeDomains)
}

func postHTTPSCheck(t *testing.T, name string, body any) (*httptest.ResponseRecorder, HTTPSCheckResponse) {
	t.Helper()

	router := gin.New()
	InitRouter(router.Group("/api"))

	raw, err := json.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/api/sites/"+name+"/https/check", bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response HTTPSCheckResponse
	if recorder.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	}
	return recorder, response
}

func checksByCode(t *testing.T, response HTTPSCheckResponse) map[string]HTTPSCheck {
	t.Helper()
	require.Len(t, response.Checks, 3)
	assert.Equal(t, httpsCheckConfigParse, response.Checks[0].Code)
	assert.Equal(t, httpsCheckChallengeRoute, response.Checks[1].Code)
	assert.Equal(t, httpsCheckDNS, response.Checks[2].Code)
	byCode := make(map[string]HTTPSCheck, len(response.Checks))
	for _, check := range response.Checks {
		byCode[check.Code] = check
	}
	return byCode
}

// --- routes ---

func newHTTPSRouteTestRouter(middlewares ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api", middlewares...)
	InitRouter(group)
	InitWebSocketRouter(group)
	return router
}

func TestHTTPSRoutesAreRegistered(t *testing.T) {
	router := newHTTPSRouteTestRouter()

	want := map[string]bool{
		http.MethodGet + " /api/sites/:name/https":        false,
		http.MethodPost + " /api/sites/:name/https/check": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		assert.True(t, found, "expected %s to be registered", route)
	}
}

// The onboarding websocket authenticates with the token query parameter, so
// it must only live on the WebSocket router group.
func TestHTTPSWebSocketRouteIsNotOnHTTPRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	InitRouter(router.Group("/api"))

	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/sites/:name/https" {
			t.Fatal("sites/:name/https must not be registered on the plain HTTP router group")
		}
	}
}

func TestHTTPSRoutesRejectDemoRequests(t *testing.T) {
	setSitesDemoMode(t, true)
	router := newHTTPSRouteTestRouter()

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/sites/example.com/https"},
		{method: http.MethodPost, path: "/api/sites/example.com/https/check"},
	} {
		t.Run(test.method, func(t *testing.T) {
			response := requestSitesRoute(t, router, test.method, test.path)
			assert.Contains(t, response.Body.String(), "disabled in demo mode",
				"expected demo request to be rejected, got %d", response.Code)
		})
	}
}

func TestHTTPSRoutesRequireSecureSessionForOTPUser(t *testing.T) {
	token := setupSiteSecurityTest(t)
	setSitesDemoMode(t, false)
	router := newHTTPSRouteTestRouter(middleware.AuthRequired())

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/sites/example.com/https"},
		{method: http.MethodPost, path: "/api/sites/example.com/https/check"},
	} {
		t.Run(test.method, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString("{}"))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			assert.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		})
	}
}

// --- check endpoint ---

func TestCheckSiteHTTPSHappyPath(t *testing.T) {
	f := setupHTTPSTest(t, httpsTestRoutedSite, true)

	recorder, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
		Domains:         []string{"Example.COM"},
		ChallengeMethod: "http01",
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	checks := checksByCode(t, response)
	assert.Equal(t, site.HTTPSStatusSuccess, checks[httpsCheckConfigParse].Status)
	assert.Equal(t, httpsCheckConfigOKMessage.Message, checks[httpsCheckConfigParse].Message)

	route := checks[httpsCheckChallengeRoute]
	assert.Equal(t, site.HTTPSStatusSuccess, route.Status)
	assert.Equal(t, httpsCheckRouteReachableMessage.Message, route.Message)
	require.Equal(t, 1, f.probeCalls())
	assert.Equal(t, []string{"example.com"}, f.probeDomains[0], "the probe gets the normalized identifiers")

	dns := checks[httpsCheckDNS]
	assert.Equal(t, site.HTTPSStatusSuccess, dns.Status)
	assert.Equal(t, acmehint.Message(acmehint.CodeDNSOK), dns.Message)
	assert.Contains(t, dns.Detail, "dns_ok")
	assert.Contains(t, dns.Params, "diagnostics")
	assert.Equal(t, 1, f.diagnoseCalls)

	// The check has no side effects on the site.
	content, err := os.ReadFile(filepath.Join(f.confDir, "sites-available", httpsTestSite))
	require.NoError(t, err)
	assert.Equal(t, httpsTestRoutedSite, string(content))
}

func TestCheckSiteHTTPSSkipsProbeUntilStaged(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		enabled bool
	}{
		{name: "disabled site", content: httpsTestRoutedSite, enabled: false},
		{name: "enabled site without challenge route", content: httpsTestPlainSite, enabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := setupHTTPSTest(t, test.content, test.enabled)

			recorder, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
				Domains:         []string{httpsTestSite},
				ChallengeMethod: "http01",
			})
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

			route := checksByCode(t, response)[httpsCheckChallengeRoute]
			assert.Equal(t, site.HTTPSStatusSkipped, route.Status)
			assert.Equal(t, "Verified after the site is staged", route.Message)
			assert.Zero(t, f.probeCalls())
		})
	}
}

func TestCheckSiteHTTPSDNS01SkipsChallengeRoute(t *testing.T) {
	f := setupHTTPSTest(t, httpsTestRoutedSite, true)
	f.diagnostics = []site.HTTPSDiagnostic{
		{Level: acmehint.LevelWarning, Code: acmehint.CodeDNSPointsElsewhere, Message: "points elsewhere",
			Params: map[string]string{"domain": httpsTestSite}},
		{Level: acmehint.LevelWarning, Code: acmehint.CodeCAABlocksCA, Message: "caa blocks",
			Params: map[string]string{"domain": httpsTestSite, "ca": "letsencrypt.org"}},
	}

	recorder, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
		Domains:         []string{httpsTestSite},
		ChallengeMethod: "dns01",
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	checks := checksByCode(t, response)
	assert.Equal(t, site.HTTPSStatusSuccess, checks[httpsCheckConfigParse].Status)

	route := checks[httpsCheckChallengeRoute]
	assert.Equal(t, site.HTTPSStatusSkipped, route.Status)
	assert.Equal(t, httpsCheckRouteDNSMessage.Message, route.Message)
	assert.Zero(t, f.probeCalls())

	// Only the CAA diagnostic matters for DNS-01.
	dns := checks[httpsCheckDNS]
	assert.Equal(t, site.HTTPSStatusWarning, dns.Status)
	assert.Equal(t, "caa blocks", dns.Message)
	assert.NotContains(t, dns.Detail, acmehint.CodeDNSPointsElsewhere)
}

func TestCheckSiteHTTPSInvalidDomain(t *testing.T) {
	f := setupHTTPSTest(t, httpsTestRoutedSite, true)

	recorder, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
		Domains:         []string{"bad domain;{}"},
		ChallengeMethod: "http01",
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	checks := checksByCode(t, response)
	parse := checks[httpsCheckConfigParse]
	assert.Equal(t, site.HTTPSStatusError, parse.Status)
	assert.NotEmpty(t, parse.Message)
	assert.Equal(t, site.HTTPSStatusSkipped, checks[httpsCheckChallengeRoute].Status)
	assert.Equal(t, site.HTTPSStatusSkipped, checks[httpsCheckDNS].Status)
	assert.Zero(t, f.probeCalls())
	assert.Zero(t, f.diagnoseCalls)
}

func TestCheckSiteHTTPSReportsProbeFailureWithHint(t *testing.T) {
	f := setupHTTPSTest(t, httpsTestRoutedSite, true)
	f.probeResults = []cert.HTTP01ProbeResult{
		{Domain: httpsTestSite, Status: cert.HTTP01ProbeStatusFailure, Target: "http://127.0.0.1:80", Error: "unexpected status 404"},
	}

	recorder, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
		Domains:         []string{httpsTestSite},
		ChallengeMethod: "http01",
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	route := checksByCode(t, response)[httpsCheckChallengeRoute]
	assert.Equal(t, site.HTTPSStatusError, route.Status)
	require.NotNil(t, route.Hint)
	assert.Equal(t, httpsHintChallengeRouteUnavailable, route.Hint.Code)
	assert.Equal(t, httpsTestSite, route.Params["domain"])
}

func TestCheckSiteHTTPSRejectsUnknownAndInvalidSites(t *testing.T) {
	setupHTTPSTest(t, httpsTestRoutedSite, true)

	recorder, _ := postHTTPSCheck(t, "missing.example.com", HTTPSCheckRequest{Domains: []string{"missing.example.com"}})
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	gin.SetMode(gin.TestMode)
	invalid := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(invalid)
	c.Params = gin.Params{{Key: "name", Value: "..%2F..%2Fnginx.conf"}}
	CheckSiteHTTPS(c)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
}

func TestCheckSiteHTTPSPassesPlanHintToConfigParse(t *testing.T) {
	setupHTTPSTest(t, httpsTestPlainSite, true)

	recorder, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
		Domains:         []string{"other.example.net"},
		ChallengeMethod: "http01",
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	parse := checksByCode(t, response)[httpsCheckConfigParse]
	assert.Equal(t, site.HTTPSStatusError, parse.Status)
	require.NotNil(t, parse.Hint)
	assert.Equal(t, site.HTTPSHintDomainsNotInServerName, parse.Hint.Code)
}

// --- existing certificate ---

// stubHTTPSCertificate replaces the certificate loader for the test.
func stubHTTPSCertificate(t *testing.T, load func(id uint64) (*site.HTTPSExistingCertificate, error)) {
	t.Helper()
	previous := loadHTTPSCertificate
	loadHTTPSCertificate = load
	t.Cleanup(func() { loadHTTPSCertificate = previous })
}

func httpsTestExistingCertificate(notAfter time.Time, covered ...string) func(uint64) (*site.HTTPSExistingCertificate, error) {
	return func(id uint64) (*site.HTTPSExistingCertificate, error) {
		return &site.HTTPSExistingCertificate{
			ID:                id,
			Name:              "Imported",
			SSLCertificate:    "/etc/nginx/ssl/imported/fullchain.cer",
			SSLCertificateKey: "/etc/nginx/ssl/imported/private.key",
			KeyType:           "P256",
			NotAfter:          notAfter,
			Covers: func(identifier string) bool {
				for _, name := range covered {
					if name == identifier {
						return true
					}
				}
				return false
			},
		}, nil
	}
}

// existingCertChecks asserts the check order of an existing-certificate
// request and returns the checks by code.
func existingCertChecks(t *testing.T, response HTTPSCheckResponse) map[string]HTTPSCheck {
	t.Helper()
	require.Len(t, response.Checks, 4)
	codes := make([]string, 0, len(response.Checks))
	byCode := make(map[string]HTTPSCheck, len(response.Checks))
	for _, check := range response.Checks {
		codes = append(codes, check.Code)
		byCode[check.Code] = check
	}
	assert.Equal(t, []string{httpsCheckConfigParse, httpsCheckCertificate, httpsCheckChallengeRoute, httpsCheckDNS}, codes)
	return byCode
}

func TestCheckSiteHTTPSExistingCertificate(t *testing.T) {
	notAfter := time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, test := range []struct {
		name      string
		domains   []string
		status    string
		message   string
		uncovered string
	}{
		{name: "covers every domain", domains: []string{"Example.COM"}, status: site.HTTPSStatusSuccess,
			message: httpsCheckCertificateOKMessage.Message},
		{name: "covers some domains", domains: []string{"example.com", "www.example.com"}, status: site.HTTPSStatusWarning,
			message: httpsCheckCertificatePartialMsg.Message, uncovered: "www.example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := setupHTTPSTest(t, httpsTestRoutedSite, true)
			var loadedID uint64
			load := httpsTestExistingCertificate(notAfter, "example.com")
			stubHTTPSCertificate(t, func(id uint64) (*site.HTTPSExistingCertificate, error) {
				loadedID = id
				return load(id)
			})

			recorder, response := postHTTPSCheck(t, httpsTestSite, map[string]any{
				"domains":          test.domains,
				"challenge_method": "http01",
				"certificate_id":   9,
			})
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

			checks := existingCertChecks(t, response)
			assert.EqualValues(t, 9, loadedID)
			certificate := checks[httpsCheckCertificate]
			assert.Equal(t, test.status, certificate.Status)
			assert.Equal(t, test.message, certificate.Message)
			assert.Equal(t, "Imported", certificate.Params["name"])
			assert.Equal(t, "2031-01-02T03:04:05Z", certificate.Params["not_after"])
			assert.Equal(t, test.uncovered, certificate.Params["uncovered"])

			for _, code := range []string{httpsCheckChallengeRoute, httpsCheckDNS} {
				assert.Equal(t, site.HTTPSStatusSkipped, checks[code].Status, code)
				assert.Equal(t, httpsCheckExistingCertMessage.Message, checks[code].Message, code)
			}
			assert.Zero(t, f.probeCalls())
			assert.Zero(t, f.diagnoseCalls)
		})
	}
}

func TestCheckSiteHTTPSExistingCertificateErrors(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		setupHTTPSTest(t, httpsTestRoutedSite, true)
		stubHTTPSCertificate(t, httpsTestExistingCertificate(time.Now().Add(-time.Hour), "example.com"))

		recorder, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
			Domains:       []string{httpsTestSite},
			CertificateID: 9,
		})
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

		checks := existingCertChecks(t, response)
		assert.Equal(t, site.HTTPSStatusSuccess, checks[httpsCheckConfigParse].Status)
		certificate := checks[httpsCheckCertificate]
		assert.Equal(t, site.HTTPSStatusError, certificate.Status)
		require.NotNil(t, certificate.Hint)
		assert.Equal(t, site.HTTPSHintCertificateExpired, certificate.Hint.Code)
		assert.Equal(t, "Imported", certificate.Params["name"])
		assert.NotEmpty(t, certificate.Params["not_after"])
	})

	t.Run("no domain covered", func(t *testing.T) {
		setupHTTPSTest(t, httpsTestRoutedSite, true)
		stubHTTPSCertificate(t, httpsTestExistingCertificate(time.Now().Add(time.Hour), "other.example.net"))

		_, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
			Domains:       []string{httpsTestSite},
			CertificateID: 9,
		})

		certificate := existingCertChecks(t, response)[httpsCheckCertificate]
		assert.Equal(t, site.HTTPSStatusError, certificate.Status)
		require.NotNil(t, certificate.Hint)
		assert.Equal(t, site.HTTPSHintCertificateDoesNotCoverDomains, certificate.Hint.Code)
		assert.Equal(t, httpsTestSite, certificate.Params["uncovered"])
	})

	t.Run("unknown certificate", func(t *testing.T) {
		setupHTTPSTest(t, httpsTestRoutedSite, true)
		require.NoError(t, model.UseDB().AutoMigrate(&model.AcmeUser{}, &model.DnsCredential{}, &model.Cert{}))

		_, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
			Domains:       []string{httpsTestSite},
			CertificateID: 404,
		})

		certificate := existingCertChecks(t, response)[httpsCheckCertificate]
		assert.Equal(t, site.HTTPSStatusError, certificate.Status)
		require.NotNil(t, certificate.Hint)
		assert.Equal(t, site.HTTPSHintCertificateNotFound, certificate.Hint.Code)
	})

	t.Run("configuration problem", func(t *testing.T) {
		setupHTTPSTest(t, httpsTestRoutedSite, true)
		stubHTTPSCertificate(t, httpsTestExistingCertificate(time.Now().Add(time.Hour), "example.com"))

		_, response := postHTTPSCheck(t, httpsTestSite, HTTPSCheckRequest{
			Domains:       []string{"bad domain;{}"},
			CertificateID: 9,
		})

		checks := existingCertChecks(t, response)
		assert.Equal(t, site.HTTPSStatusError, checks[httpsCheckConfigParse].Status)
		assert.Equal(t, site.HTTPSStatusSkipped, checks[httpsCheckCertificate].Status)
	})
}

func TestHTTPSRequestDecodesCertificateID(t *testing.T) {
	var req site.HTTPSRequest
	require.NoError(t, json.Unmarshal([]byte(`{"domains":["example.com"],"certificate_id":12}`), &req))
	assert.EqualValues(t, 12, req.CertificateID)
	assert.True(t, req.UsesExistingCertificate())

	stubHTTPSCertificate(t, httpsTestExistingCertificate(time.Now().Add(time.Hour), "example.com"))
	onboarding := newHTTPSOnboarding(httpsTestSite, req, func(site.HTTPSEvent) {})
	require.NotNil(t, onboarding.LoadCertificate)
	existing, err := onboarding.LoadCertificate(12)
	require.NoError(t, err)
	assert.Equal(t, "Imported", existing.Name)
}

// --- websocket ---

func TestEnableSiteHTTPSStreamsDoneAndCloses(t *testing.T) {
	setupHTTPSTest(t, httpsTestRoutedSite, true)
	setSitesDemoMode(t, false)

	router := gin.New()
	InitWebSocketRouter(router.Group("/api"))
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	header := http.Header{"Origin": []string{server.URL}}
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(server.URL, "http")+"/api/sites/"+httpsTestSite+"/https", header)
	require.NoError(t, err)
	defer conn.Close()

	// An invalid identifier fails the plan step before anything is written.
	require.NoError(t, conn.WriteJSON(site.HTTPSRequest{
		Domains:         []string{"bad domain;{}"},
		ChallengeMethod: "http01",
	}))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var events []site.HTTPSEvent
	for {
		var event site.HTTPSEvent
		if err := conn.ReadJSON(&event); err != nil {
			assert.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure), "unexpected read error: %v", err)
			break
		}
		events = append(events, event)
	}

	require.NotEmpty(t, events)
	done := events[len(events)-1]
	assert.Equal(t, site.HTTPSEventDone, done.Type)
	assert.Equal(t, site.HTTPSStatusError, done.Status)
	assert.Equal(t, site.HTTPSStepPlan, done.Step)
	doneCount := 0
	for _, event := range events {
		if event.Type == site.HTTPSEventDone {
			doneCount++
		}
	}
	assert.Equal(t, 1, doneCount)
}

func TestEnableSiteHTTPSClosesSocketWithoutRequest(t *testing.T) {
	setupHTTPSTest(t, httpsTestRoutedSite, true)
	setSitesDemoMode(t, false)

	router := gin.New()
	router.GET("/api/sites/:name/https", func(c *gin.Context) {
		enableSiteHTTPS(c, 100*time.Millisecond)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	header := http.Header{"Origin": []string{server.URL}}
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(server.URL, "http")+"/api/sites/"+httpsTestSite+"/https", header)
	require.NoError(t, err)
	defer conn.Close()

	// The client never sends the request: the server gives up and closes the
	// socket well before the client-side deadline.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	started := time.Now()
	_, _, err = conn.ReadMessage()
	require.Error(t, err)
	var netErr net.Error
	assert.False(t, errors.As(err, &netErr) && netErr.Timeout(), "client deadline hit instead of server close: %v", err)
	assert.Less(t, time.Since(started), 4*time.Second)
}

// --- mapping functions ---

func TestHTTPSProbeOutcome(t *testing.T) {
	t.Run("challenge port unavailable", func(t *testing.T) {
		_, err := httpsProbeOutcome("", nil, cert.NewHTTP01ChallengePortUnavailableError("9180", "address already in use"))

		var hintErr *site.HTTPSHintError
		require.ErrorAs(t, err, &hintErr)
		assert.Equal(t, httpsHintChallengePortUnavailable, hintErr.Hint.Code)
		assert.Equal(t, httpsChallengePortUnavailableMessage.Message, hintErr.Hint.Message)
		assert.Equal(t, map[string]string{"port": "9180", "reason": "address already in use"}, hintErr.Hint.Params)
	})

	t.Run("other probe error passes through", func(t *testing.T) {
		probeErr := context.DeadlineExceeded
		_, err := httpsProbeOutcome("", nil, probeErr)

		assert.ErrorIs(t, err, probeErr)
		var hintErr *site.HTTPSHintError
		assert.False(t, errors.As(err, &hintErr))
	})

	t.Run("route failure", func(t *testing.T) {
		_, err := httpsProbeOutcome("", []cert.HTTP01ProbeResult{
			{Domain: "a.example.com", Status: cert.HTTP01ProbeStatusSuccess, Target: "http://127.0.0.1:80"},
			{Domain: "b.example.com", Status: cert.HTTP01ProbeStatusFailure, Target: "http://127.0.0.1:80", Error: "unexpected status 404"},
		}, nil)

		var hintErr *site.HTTPSHintError
		require.ErrorAs(t, err, &hintErr)
		assert.Equal(t, httpsHintChallengeRouteUnavailable, hintErr.Hint.Code)
		assert.Equal(t, "b.example.com", hintErr.Hint.Params["domain"])
		assert.Contains(t, hintErr.Hint.Params["reason"], "unexpected status 404")
		assert.Equal(t, hintErr.Hint.Params["reason"], hintErr.Hint.Message)
		assert.Contains(t, err.Error(), "b.example.com")
	})

	t.Run("skipped", func(t *testing.T) {
		result, err := httpsProbeOutcome("", []cert.HTTP01ProbeResult{
			{Domain: "a.example.com", Status: cert.HTTP01ProbeStatusSkipped, SkipReason: "Nginx is not local"},
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, site.HTTPSProbeResult{Status: site.HTTPSStatusSkipped, Message: "Nginx is not local"}, result)
	})

	t.Run("warning", func(t *testing.T) {
		results := []cert.HTTP01ProbeResult{
			{Domain: "a.example.com", Status: cert.HTTP01ProbeStatusSuccess, Target: "http://127.0.0.1:80"},
			{Domain: "b.example.com", Status: cert.HTTP01ProbeStatusWarning, Target: "https://127.0.0.1:443", Error: "redirected to another host"},
		}
		result, err := httpsProbeOutcome("", results, nil)

		require.NoError(t, err)
		assert.Equal(t, site.HTTPSStatusWarning, result.Status)
		assert.Equal(t, cert.SummarizeHTTP01ProbeResults(results), result.Message)
	})

	t.Run("connection-level problems are a warning, not a failure", func(t *testing.T) {
		results := []cert.HTTP01ProbeResult{
			{Domain: "a.example.com", Status: cert.HTTP01ProbeStatusWarning, Target: "127.0.0.1:80",
				Error: "the local Nginx did not answer (connect: connection refused), so the route cannot be verified locally"},
			{Domain: "b.example.com", Status: cert.HTTP01ProbeStatusWarning,
				Error: "no server block listens on port 80 for b.example.com; the certificate authority may reach it through a load balancer or port mapping"},
		}
		result, err := httpsProbeOutcome("site", results, nil)

		require.NoError(t, err)
		assert.Equal(t, site.HTTPSStatusWarning, result.Status)
		assert.Contains(t, result.Message, "a.example.com: cannot verify locally")
		assert.Contains(t, result.Message, "no server block listens on port 80 for b.example.com")
	})

	t.Run("success", func(t *testing.T) {
		result, err := httpsProbeOutcome("", []cert.HTTP01ProbeResult{
			{Domain: "a.example.com", Status: cert.HTTP01ProbeStatusSuccess, Target: "http://127.0.0.1:80"},
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, site.HTTPSStatusSuccess, result.Status)
	})
}

func TestHTTPSFailureHint(t *testing.T) {
	rateLimited := errors.New(`obtain cert error: acme: error: 429 :: POST :: https://acme-v02.api.letsencrypt.org/acme/new-order :: urn:ietf:params:acme:error:rateLimited :: too many certificates (5) already issued for this exact set of identifiers in the last 168h0m0s`)

	assert.Nil(t, httpsFailureHint(site.HTTPSStepPlan, rateLimited, nil))
	assert.Nil(t, httpsFailureHint(site.HTTPSStepIssue, nil, nil))

	hint := httpsFailureHint(site.HTTPSStepIssue, rateLimited, nil)
	require.NotNil(t, hint)
	assert.Equal(t, acmehint.CodeRateLimited, hint.Code)
}

func TestDNSCheckFromDiagnostics(t *testing.T) {
	t.Run("no diagnostics", func(t *testing.T) {
		check := dnsCheckFromDiagnostics(nil)
		assert.Equal(t, site.HTTPSStatusSuccess, check.Status)
		assert.Equal(t, httpsCheckDNSOKMessage.Message, check.Message)
	})

	t.Run("worst level wins", func(t *testing.T) {
		diagnostics := []site.HTTPSDiagnostic{
			{Level: acmehint.LevelInfo, Code: acmehint.CodeDNSOK, Message: "ok", Params: map[string]string{"domain": "a.example.com"}},
			{Level: acmehint.LevelWarning, Code: acmehint.CodeAAAAWithoutIPv6Listen, Message: "aaaa", Params: map[string]string{"domain": "b.example.com", "ip": "2001:db8::1"}},
			{Level: acmehint.LevelWarning, Code: acmehint.CodeAAAAWithoutIPv6Listen, Message: "aaaa", Params: map[string]string{"domain": "c.example.com"}},
			{Level: acmehint.LevelWarning, Code: acmehint.CodeCAABlocksCA, Message: "caa", Params: map[string]string{"domain": "b.example.com"}},
		}
		check := dnsCheckFromDiagnostics(diagnostics)

		assert.Equal(t, httpsCheckDNS, check.Code)
		assert.Equal(t, site.HTTPSStatusWarning, check.Status)
		assert.Equal(t, "aaaa caa", check.Message, "only the distinct messages of the worst level are joined")
		assert.Equal(t, diagnostics, check.Params["diagnostics"])
		assert.Equal(t,
			"dns_ok (domain=a.example.com); aaaa_without_ipv6_listen (domain=b.example.com, ip=2001:db8::1); aaaa_without_ipv6_listen (domain=c.example.com); caa_blocks_ca (domain=b.example.com)",
			check.Detail)
	})

	t.Run("info only", func(t *testing.T) {
		check := dnsCheckFromDiagnostics([]site.HTTPSDiagnostic{
			{Level: acmehint.LevelInfo, Code: acmehint.CodeDNSResolved, Message: "resolved"},
		})
		assert.Equal(t, site.HTTPSStatusSuccess, check.Status)
		assert.Equal(t, "resolved", check.Message)
	})
}

func TestConfigParseCheckReportsPlanDiagnostics(t *testing.T) {
	check := configParseCheck(&site.HTTPSPlan{Diagnostics: []site.HTTPSDiagnostic{
		{Level: acmehint.LevelWarning, Code: site.HTTPSDiagnosticServerNamesExtended, Message: "extended",
			Params: map[string]string{"domains": "www.example.com"}},
	}})

	assert.Equal(t, site.HTTPSStatusWarning, check.Status)
	assert.Equal(t, "extended", check.Message)
	assert.Contains(t, check.Detail, site.HTTPSDiagnosticServerNamesExtended)
}

func TestErrorCheckCarriesHint(t *testing.T) {
	check := errorCheck(httpsCheckConfigParse, &site.HTTPSHintError{
		Hint: site.HTTPSHint{Code: site.HTTPSHintAdvancedConfigUnsupported, Message: "use the advanced editor",
			Params: map[string]string{"reason": "x"}},
		Err: errors.New("site uses the advanced editor"),
	})

	assert.Equal(t, site.HTTPSStatusError, check.Status)
	assert.Equal(t, "site uses the advanced editor", check.Message)
	require.NotNil(t, check.Hint)
	assert.Equal(t, site.HTTPSHintAdvancedConfigUnsupported, check.Hint.Code)
	assert.Equal(t, "use the advanced editor", check.Detail)
	assert.Equal(t, "x", check.Params["reason"])
}

// --- issuance ---

func TestHTTPSIssuePayloadLeavesConfigNameEmpty(t *testing.T) {
	payload := httpsIssuePayload(site.HTTPSIssueRequest{
		SiteName: httpsTestSite,
		HTTPSRequest: site.HTTPSRequest{
			Domains:          []string{"example.com", "www.example.com"},
			ChallengeMethod:  "dns01",
			DNSCredentialID:  3,
			ACMEUserID:       4,
			KeyType:          "",
			Profile:          "shortlived",
			MustStaple:       true,
			ChallengeConfig:  map[string]any{"disable_cname": true},
			EnableCommonName: true,
			RevokeOld:        true,
		},
	})

	assert.Empty(t, payload.ConfigName, "IssueCert must not probe the staged configuration again")
	assert.Equal(t, []string{"example.com", "www.example.com"}, payload.ServerName)
	assert.Equal(t, "dns01", payload.ChallengeMethod)
	assert.Equal(t, uint64(3), payload.DNSCredentialID)
	assert.Equal(t, uint64(4), payload.ACMEUserID)
	assert.NotEmpty(t, payload.KeyType, "the key type is normalized to the default")
	assert.Equal(t, "shortlived", payload.Profile)
	assert.True(t, payload.MustStaple)
	assert.Equal(t, map[string]any{"disable_cname": true}, payload.ChallengeConfig)
	assert.True(t, payload.EnableCommonName)
	assert.True(t, payload.RevokeOld)
}

func TestIssueHTTPSCertificateStreamsLogsBeforeReturning(t *testing.T) {
	previous := issueWithRecord
	t.Cleanup(func() { issueWithRecord = previous })

	var gotName string
	var gotPayload *cert.ConfigPayload
	issueWithRecord = func(name string, payload *cert.ConfigPayload, log *cert.Logger) (*model.Cert, error) {
		gotName = name
		gotPayload = payload
		for i := 0; i < 20; i++ {
			log.Info(translation.C("[Nginx UI] Preparing lego configurations"))
		}
		payload.SSLCertificatePath = "/etc/nginx/ssl/example.com_P256/fullchain.cer"
		payload.SSLCertificateKeyPath = "/etc/nginx/ssl/example.com_P256/private.key"
		return &model.Cert{Model: model.Model{ID: 9}}, nil
	}

	var logs []string
	result, err := issueHTTPSCertificate(context.Background(), site.HTTPSIssueRequest{
		SiteName: httpsTestSite,
		HTTPSRequest: site.HTTPSRequest{
			Domains:         []string{httpsTestSite},
			ChallengeMethod: "http01",
			KeyType:         "P256",
		},
	}, func(message string, args map[string]any) {
		logs = append(logs, message)
	})
	require.NoError(t, err)

	assert.Equal(t, httpsTestSite, gotName)
	assert.Empty(t, gotPayload.ConfigName)
	assert.Len(t, logs, 20, "every log line is delivered before the result")
	assert.Equal(t, "[Nginx UI] Preparing lego configurations", logs[0])
	assert.Equal(t, uint64(9), result.CertID)
	assert.Equal(t, gotPayload.GetCertificatePath(), result.SSLCertificate)
	assert.Equal(t, gotPayload.GetCertificateKeyPath(), result.SSLCertificateKey)
	assert.Equal(t, gotPayload.GetKeyType(), result.KeyType)
}

func TestIssueHTTPSCertificateReturnsError(t *testing.T) {
	previous := issueWithRecord
	t.Cleanup(func() { issueWithRecord = previous })

	issueErr := errors.New("acme: error: 403")
	issueWithRecord = func(name string, payload *cert.ConfigPayload, log *cert.Logger) (*model.Cert, error) {
		return &model.Cert{Model: model.Model{ID: 1}}, issueErr
	}

	_, err := issueHTTPSCertificate(context.Background(), site.HTTPSIssueRequest{
		SiteName:     httpsTestSite,
		HTTPSRequest: site.HTTPSRequest{Domains: []string{httpsTestSite}},
	}, nil)
	assert.ErrorIs(t, err, issueErr)
}
