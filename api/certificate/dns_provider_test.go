package certificate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v5/challenge"
	legodns01 "github.com/go-acme/lego/v5/challenge/dns01"
)

// stubSource feeds the registry a known provider list for one test.
type stubSource struct {
	providers []dns.ProviderInfo
}

func (s stubSource) Providers() []dns.ProviderInfo { return s.providers }

func (stubSource) NewChallengeProvider(context.Context, string, dns.Configuration, map[string]any) (challenge.Provider, []legodns01.ChallengeOption, func(), error) {
	return nil, nil, func() {}, dns.ErrProviderNotFound
}

func registerSource(t *testing.T, providers ...dns.ProviderInfo) {
	t.Helper()
	// Registration is additive by design, so a test may only add entries.
	// The codes below exist nowhere else, which keeps the tests independent.
	dns.RegisterSource(stubSource{providers: providers})
}

func performRequest(t *testing.T, handler gin.HandlerFunc, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(path, handler)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

func TestGetDNSProvidersListStripsTheSchema(t *testing.T) {
	registerSource(t, dns.ProviderInfo{
		Config: dns.Config{
			Name:          "Test Vendor",
			Code:          "api-test-vendor",
			Configuration: &dns.Configuration{Credentials: map[string]string{"TEST_TOKEN": "API token"}},
			Links:         &dns.Links{API: "https://example.com/api"},
		},
		PluginID: "com.example.dns",
		DNS01:    true,
	})

	recorder := performRequest(t, GetDNSProvidersList, http.MethodGet, "/certificate/dns_providers")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var list []dns.ProviderInfo
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, provider := range list {
		if provider.Code != "api-test-vendor" {
			continue
		}
		found = true
		if provider.Configuration != nil || provider.Links != nil {
			t.Fatal("the list endpoint returned the provider schema")
		}
		if !provider.DNS01 || provider.PluginID != "com.example.dns" {
			t.Fatalf("provider = %#v, want a dns01 entry owned by the plugin", provider)
		}
	}
	if !found {
		t.Fatal("the registered provider is missing from the list")
	}
}

func TestGetDNSProviderReturnsTheSchema(t *testing.T) {
	registerSource(t, dns.ProviderInfo{
		Config: dns.Config{
			Name:          "Detail Vendor",
			Code:          "api-detail-vendor",
			Configuration: &dns.Configuration{Credentials: map[string]string{"DETAIL_TOKEN": "API token"}},
		},
		PluginID: "com.example.dns",
		DNS01:    true,
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/certificate/dns_provider/:code", GetDNSProvider)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/certificate/dns_provider/api-detail-vendor", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var provider dns.ProviderInfo
	if err := json.Unmarshal(recorder.Body.Bytes(), &provider); err != nil {
		t.Fatal(err)
	}
	if provider.Configuration == nil || provider.Configuration.Credentials["DETAIL_TOKEN"] != "API token" {
		t.Fatalf("configuration = %#v, want the full schema", provider.Configuration)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/certificate/dns_provider/does-not-exist", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status for an unknown code = %d, want 404", recorder.Code)
	}
}

func TestGetChallengeMethodsOffersDNS01WhenAPluginProvidesIt(t *testing.T) {
	recorder := performRequest(t, GetChallengeMethods, http.MethodGet, "/certificate/challenge_methods")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var methods []ChallengeMethod
	if err := json.Unmarshal(recorder.Body.Bytes(), &methods); err != nil {
		t.Fatal(err)
	}
	if len(methods) == 0 || methods[0].Code != "http01" || !methods[0].Builtin {
		t.Fatalf("methods = %#v, want http01 first and builtin", methods)
	}

	if !dns.HasDNS01Providers() {
		if len(methods) != 1 {
			t.Fatalf("methods = %#v, want only http01 while nothing provides dns01", methods)
		}
		return
	}

	var dns01Method *ChallengeMethod
	for i := range methods {
		if methods[i].Code == "dns01" {
			dns01Method = &methods[i]
		}
	}
	if dns01Method == nil {
		t.Fatalf("methods = %#v, want a dns01 entry", methods)
	}
	if dns01Method.Builtin {
		t.Fatal("dns01 is reported as builtin")
	}
	if dns01Method.PluginID == "" {
		t.Fatal("the dns01 entry does not name a plugin")
	}
}

func TestDNS01PluginIDPrefersTheOfficialPlugin(t *testing.T) {
	registerSource(t,
		dns.ProviderInfo{Config: dns.Config{Name: "Third Party A", Code: "api-third-party-a"}, PluginID: "com.example.dns", DNS01: true},
		dns.ProviderInfo{Config: dns.Config{Name: "Third Party B", Code: "api-third-party-b"}, PluginID: "com.example.dns", DNS01: true},
		dns.ProviderInfo{Config: dns.Config{Name: "Official", Code: "api-official"}, PluginID: dns.OfficialDNS01PluginID, DNS01: true},
	)

	if got := dns01PluginID(); got != dns.OfficialDNS01PluginID {
		t.Fatalf("dns01PluginID = %q, want %q", got, dns.OfficialDNS01PluginID)
	}
}
