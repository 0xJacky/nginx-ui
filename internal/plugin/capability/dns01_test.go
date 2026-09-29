package capability

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/go-acme/lego/v5/challenge"
	legodns01 "github.com/go-acme/lego/v5/challenge/dns01"
)

// recordedCall is one JSON-RPC request a fake caller received.
type recordedCall struct {
	Method string
	Params json.RawMessage
}

// fakeCaller answers with canned results keyed by method.
type fakeCaller struct {
	mu      sync.Mutex
	calls   []recordedCall
	results map[string]any
	errs    map[string]error
}

func newFakeCaller() *fakeCaller {
	return &fakeCaller{results: map[string]any{}, errs: map[string]error{}}
}

func (f *fakeCaller) Call(_ context.Context, method string, params any, result any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.calls = append(f.calls, recordedCall{Method: method, Params: raw})
	canned, hasResult := f.results[method]
	callErr := f.errs[method]
	f.mu.Unlock()

	if callErr != nil {
		return callErr
	}
	if result == nil || !hasResult {
		return nil
	}
	encoded, err := json.Marshal(canned)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, result)
}

func (f *fakeCaller) Notify(context.Context, string, any) error { return nil }

func (f *fakeCaller) recorded() []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedCall(nil), f.calls...)
}

func (f *fakeCaller) methodCalls(method string) []recordedCall {
	var out []recordedCall
	for _, call := range f.recorded() {
		if call.Method == method {
			out = append(out, call)
		}
	}
	return out
}

// fakeHost is a Host whose answers the test controls.
type fakeHost struct {
	owner      string
	providers  []DNS01ProviderEntry
	manifests  map[string]*protocol.Manifest
	callers    map[string]*fakeCaller
	acquireErr map[string]error

	mu        sync.Mutex
	acquired  []string
	releases  int
	ownerCall int
}

func (h *fakeHost) OwnerOf(capability, code string) (string, bool) {
	h.mu.Lock()
	h.ownerCall++
	h.mu.Unlock()
	if capability != protocol.CapabilityDNS01 || h.owner == "" {
		return "", false
	}
	for _, entry := range h.providers {
		if entry.Provider.Code == code {
			return h.owner, true
		}
	}
	return "", false
}

func (h *fakeHost) Acquire(_ context.Context, pluginID string) (jsonrpc.Caller, func(), error) {
	h.mu.Lock()
	h.acquired = append(h.acquired, pluginID)
	h.mu.Unlock()

	release := func() {
		h.mu.Lock()
		h.releases++
		h.mu.Unlock()
	}
	if err, ok := h.acquireErr[pluginID]; ok && err != nil {
		return nil, release, err
	}
	caller, ok := h.callers[pluginID]
	if !ok {
		return nil, release, errors.New("plugin is not running")
	}
	return caller, release, nil
}

func (h *fakeHost) Manifest(pluginID string) (*protocol.Manifest, bool) {
	m, ok := h.manifests[pluginID]
	return m, ok
}

func (h *fakeHost) DNS01Providers() []DNS01ProviderEntry { return h.providers }

func (h *fakeHost) releaseCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.releases
}

const testPluginID = "com.example.dns"

func testHost(caller *fakeCaller) *fakeHost {
	provider := protocol.DNS01Provider{
		Name:                      "Cloudflare",
		Code:                      "cloudflare",
		Links:                     &protocol.DNS01ProviderLinks{API: "https://api.cloudflare.com/"},
		PropagationTimeoutSeconds: 180,
		PollingIntervalSeconds:    5,
		Form: &protocol.DNS01ProviderForm{
			Fields: []protocol.DNS01ProviderField{
				{Key: "CF_DNS_API_TOKEN", Label: "API token", Group: "credential", Secret: true},
				{Key: "CLOUDFLARE_TTL", Label: "TXT record TTL", Group: "setting", Default: "120", Unit: "seconds"},
			},
			Methods: []protocol.DNS01ProviderMethod{{Name: "API token", Recommended: true, Fields: []string{"CF_DNS_API_TOKEN"}}},
		},
	}
	return &fakeHost{
		owner:     testPluginID,
		providers: []DNS01ProviderEntry{{PluginID: testPluginID, Provider: provider}},
		manifests: map[string]*protocol.Manifest{
			testPluginID: {ID: testPluginID, DNS01: &protocol.ManifestDNS01{Providers: []protocol.DNS01Provider{provider}}},
		},
		callers: map[string]*fakeCaller{testPluginID: caller},
	}
}

func testConfiguration() dns.Configuration {
	return dns.Configuration{
		Credentials: map[string]string{"CF_DNS_API_TOKEN": "token"},
		Additional:  map[string]string{"CLOUDFLARE_TTL": "120"},
	}
}

func TestProvidersMapsTheManifest(t *testing.T) {
	host := testHost(newFakeCaller())
	list := NewDNS01Source(host).Providers()

	if len(list) != 1 {
		t.Fatalf("provider count = %d, want 1", len(list))
	}
	p := list[0]
	if p.Code != "cloudflare" || p.Name != "Cloudflare" {
		t.Fatalf("provider = %s/%s, want Cloudflare/cloudflare", p.Name, p.Code)
	}
	if !p.DNS01 || p.RecordManagement {
		t.Fatalf("flags = (dns01 %v, record %v), want (true, false)", p.DNS01, p.RecordManagement)
	}
	if p.PluginID != testPluginID {
		t.Fatalf("plugin id = %q, want %q", p.PluginID, testPluginID)
	}
	if p.Links == nil || p.Links.API != "https://api.cloudflare.com/" {
		t.Fatalf("links = %#v, want the manifest links", p.Links)
	}
	if p.Form == nil || len(p.Form.Fields) != 2 || len(p.Form.Methods) != 1 {
		t.Fatalf("form = %#v, want the manifest form", p.Form)
	}
	want := dns.FormField{Key: "CLOUDFLARE_TTL", Label: "TXT record TTL", Group: "setting", Default: "120", Unit: "seconds"}
	if p.Form.Fields[1] != want || !p.Form.Fields[0].Secret {
		t.Fatalf("form fields = %#v", p.Form.Fields)
	}
	if m := p.Form.Methods[0]; m.Name != "API token" || !m.Recommended || len(m.Fields) != 1 {
		t.Fatalf("form method = %#v", m)
	}
}

func TestNewChallengeProviderUsesTheReportedTimings(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodDNS01Options] = protocol.DNS01OptionsResult{
		PropagationTimeoutSeconds: 300,
		PollingIntervalSeconds:    10,
	}
	host := testHost(caller)

	provider, opts, release, err := NewDNS01Source(host).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	// The recursive requirement is dropped and the pre-check is wrapped.
	if len(opts) != 2 {
		t.Fatalf("challenge option count = %d, want 2", len(opts))
	}

	timeouter, ok := provider.(challenge.ProviderTimeout)
	if !ok {
		t.Fatal("provider does not implement challenge.ProviderTimeout")
	}
	timeout, interval := timeouter.Timeout()
	if timeout != 300*time.Second || interval != 10*time.Second {
		t.Fatalf("timings = (%s, %s), want (5m, 10s)", timeout, interval)
	}

	calls := caller.methodCalls(protocol.MethodDNS01Options)
	if len(calls) != 1 {
		t.Fatalf("dns01.options call count = %d, want 1", len(calls))
	}
	var params protocol.DNS01OptionsParams
	if err := json.Unmarshal(calls[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Provider != "cloudflare" {
		t.Fatalf("options provider = %q, want cloudflare", params.Provider)
	}
	if params.Config["CF_DNS_API_TOKEN"] != "token" || params.Config["CLOUDFLARE_TTL"] != "120" {
		t.Fatalf("options config = %#v, want the merged credential form", params.Config)
	}

	release()
	if host.releaseCount() == 0 {
		t.Fatal("release did not reach the host")
	}
}

func TestNewChallengeProviderFallsBackToTheManifestTimings(t *testing.T) {
	caller := newFakeCaller()
	caller.errs[protocol.MethodDNS01Options] = &protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method"}

	provider, _, release, err := NewDNS01Source(testHost(caller)).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	timeout, interval := provider.(challenge.ProviderTimeout).Timeout()
	if timeout != 180*time.Second || interval != 5*time.Second {
		t.Fatalf("timings = (%s, %s), want the manifest defaults (3m, 5s)", timeout, interval)
	}
}

func TestNewChallengeProviderFallsBackToTheLegoDefaults(t *testing.T) {
	caller := newFakeCaller()
	caller.errs[protocol.MethodDNS01Options] = &protocol.Error{Code: protocol.CodeUnsupported, Message: "not implemented"}
	host := testHost(caller)
	host.providers[0].Provider.PropagationTimeoutSeconds = 0
	host.providers[0].Provider.PollingIntervalSeconds = 0
	host.manifests[testPluginID].DNS01.Providers[0].PropagationTimeoutSeconds = 0
	host.manifests[testPluginID].DNS01.Providers[0].PollingIntervalSeconds = 0

	provider, _, release, err := NewDNS01Source(host).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	timeout, interval := provider.(challenge.ProviderTimeout).Timeout()
	if timeout != legodns01.DefaultPropagationTimeout || interval != legodns01.DefaultPollingInterval {
		t.Fatalf("timings = (%s, %s), want lego's defaults", timeout, interval)
	}
}

func TestNewChallengeProviderWrapsSequentialProviders(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodDNS01Options] = protocol.DNS01OptionsResult{SequentialIntervalSeconds: 60}

	provider, _, release, err := NewDNS01Source(testHost(caller)).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	sequential, ok := provider.(interface{ Sequential() time.Duration })
	if !ok {
		t.Fatal("provider does not implement Sequential")
	}
	if got := sequential.Sequential(); got != time.Minute {
		t.Fatalf("Sequential = %s, want 1m", got)
	}
	if _, ok := provider.(challenge.ProviderTimeout); !ok {
		t.Fatal("the sequential wrapper lost the Timeout method")
	}
}

func TestNewChallengeProviderWithoutOwner(t *testing.T) {
	host := testHost(newFakeCaller())
	host.owner = ""

	_, _, release, err := NewDNS01Source(host).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if !errors.Is(err, dns.ErrProviderNotFound) {
		t.Fatalf("error = %v, want ErrProviderNotFound", err)
	}
	release()
}

func TestPresentAndCleanUpSendTheChallengeRecord(t *testing.T) {
	caller := newFakeCaller()
	host := testHost(caller)

	provider, _, release, err := NewDNS01Source(host).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), map[string]any{"disable_cname": true})
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	const domain = "example.com"
	const keyAuth = "token.thumbprint"
	if err := provider.Present(t.Context(), domain, "token", keyAuth); err != nil {
		t.Fatalf("Present error = %v", err)
	}
	if err := provider.CleanUp(t.Context(), domain, "token", keyAuth); err != nil {
		t.Fatalf("CleanUp error = %v", err)
	}

	info := legodns01.GetChallengeInfo(t.Context(), domain, keyAuth)
	for _, method := range []string{protocol.MethodDNS01Present, protocol.MethodDNS01Cleanup} {
		calls := caller.methodCalls(method)
		if len(calls) != 1 {
			t.Fatalf("%s call count = %d, want 1", method, len(calls))
		}
		var params protocol.DNS01ChallengeParams
		if err := json.Unmarshal(calls[0].Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.Provider != "cloudflare" || params.Domain != domain {
			t.Fatalf("%s params = %#v, want the cloudflare challenge for %s", method, params, domain)
		}
		if params.FQDN != info.FQDN || params.EffectiveFQDN != info.EffectiveFQDN || params.Value != info.Value {
			t.Fatalf("%s record = (%s, %s, %s), want (%s, %s, %s)", method,
				params.FQDN, params.EffectiveFQDN, params.Value, info.FQDN, info.EffectiveFQDN, info.Value)
		}
		if params.Token != "token" || params.KeyAuth != keyAuth {
			t.Fatalf("%s carried token %q and key auth %q", method, params.Token, params.KeyAuth)
		}
		if params.Options["disable_cname"] != true {
			t.Fatalf("%s options = %#v, want the per-certificate options", method, params.Options)
		}
	}
}

func TestPresentWrapsThePluginError(t *testing.T) {
	caller := newFakeCaller()
	caller.errs[protocol.MethodDNS01Present] = &protocol.Error{Code: protocol.CodeInternalError, Message: "vendor rejected the record"}

	provider, _, release, err := NewDNS01Source(testHost(caller)).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	err = provider.Present(t.Context(), "example.com", "token", "token.thumbprint")
	if err == nil {
		t.Fatal("Present error = nil, want the plugin error")
	}
	var rpcErr *protocol.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("Present error = %v, want a wrapped JSON-RPC error", err)
	}
}

// preCheck runs the pre-check the source installed. The option itself only
// sets an unexported lego field, so the test drives the func behind it.
func preCheck(t *testing.T, provider challenge.Provider, opts []legodns01.ChallengeOption, fallback legodns01.PreCheckFunc) (bool, error) {
	t.Helper()
	// The recursive requirement is dropped and the pre-check is wrapped.
	if len(opts) != 2 {
		t.Fatalf("challenge option count = %d, want 2", len(opts))
	}
	return challengeProviderOf(t, provider).preCheckFunc()(t.Context(), "example.com", "_acme-challenge.example.com.", "value", fallback)
}

func challengeProviderOf(t *testing.T, provider challenge.Provider) *challengeProvider {
	t.Helper()
	switch p := provider.(type) {
	case *challengeProvider:
		return p
	case *sequentialChallengeProvider:
		return p.challengeProvider
	default:
		t.Fatalf("provider type = %T, want a plugin backed provider", provider)
		return nil
	}
}

func TestPreCheckUsesTheOwningPlugin(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodDNS01Check] = protocol.DNS01CheckResult{Ready: true}

	provider, opts, release, err := NewDNS01Source(testHost(caller)).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	ready, err := preCheck(t, provider, opts, func(context.Context, string, string) (bool, error) {
		t.Fatal("the core check ran although the plugin answered")
		return false, nil
	})
	if err != nil || !ready {
		t.Fatalf("pre-check = (%v, %v), want (true, nil)", ready, err)
	}
	if len(caller.methodCalls(protocol.MethodDNS01Check)) != 1 {
		t.Fatalf("dns01.check call count = %d, want 1", len(caller.methodCalls(protocol.MethodDNS01Check)))
	}
}

func TestPreCheckFallsBackToTheOfficialPlugin(t *testing.T) {
	owner := newFakeCaller()
	owner.errs[protocol.MethodDNS01Check] = &protocol.Error{Code: protocol.CodeUnsupported, Message: "no check"}
	official := newFakeCaller()
	official.results[protocol.MethodDNS01Check] = protocol.DNS01CheckResult{Ready: false, Detail: "2 of 3 nameservers answered"}

	host := testHost(owner)
	host.callers[dns.OfficialDNS01PluginID] = official
	host.manifests[dns.OfficialDNS01PluginID] = &protocol.Manifest{ID: dns.OfficialDNS01PluginID}

	provider, opts, release, err := NewDNS01Source(host).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	ready, err := preCheck(t, provider, opts, func(context.Context, string, string) (bool, error) {
		t.Fatal("the core check ran although the official plugin answered")
		return false, nil
	})
	if err != nil || ready {
		t.Fatalf("pre-check = (%v, %v), want (false, nil)", ready, err)
	}
	if len(official.methodCalls(protocol.MethodDNS01Check)) != 1 {
		t.Fatal("the official plugin was not asked to check")
	}
}

func TestPreCheckWaitsWhenTheAuthoritativeCheckIsOff(t *testing.T) {
	previous := disabledAuthoritativeNSPropagationWait
	disabledAuthoritativeNSPropagationWait = 10 * time.Millisecond
	t.Cleanup(func() { disabledAuthoritativeNSPropagationWait = previous })

	owner := newFakeCaller()
	owner.errs[protocol.MethodDNS01Check] = &protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method"}
	options := map[string]any{protocol.DNS01OptionDisableAuthoritativeNSPropagation: true}

	provider, opts, release, err := NewDNS01Source(testHost(owner)).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), options)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	ready, err := preCheck(t, provider, opts, func(context.Context, string, string) (bool, error) {
		t.Fatal("the core check ran although the certificate turned it off")
		return false, nil
	})
	if err != nil || !ready {
		t.Fatalf("pre-check = (%v, %v), want (true, nil)", ready, err)
	}
}

func TestPreCheckFallsBackToTheCoreCheck(t *testing.T) {
	owner := newFakeCaller()
	owner.errs[protocol.MethodDNS01Check] = &protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method"}

	provider, opts, release, err := NewDNS01Source(testHost(owner)).NewChallengeProvider(t.Context(), "cloudflare", testConfiguration(), nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	defer release()

	called := false
	ready, err := preCheck(t, provider, opts, func(context.Context, string, string) (bool, error) {
		called = true
		return true, nil
	})
	if err != nil || !ready {
		t.Fatalf("pre-check = (%v, %v), want (true, nil)", ready, err)
	}
	if !called {
		t.Fatal("the core check did not run although no plugin can check")
	}
}
