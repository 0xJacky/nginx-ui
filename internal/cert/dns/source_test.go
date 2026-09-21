package dns

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
)

// fakeSource is a Source whose answers the test controls.
type fakeSource struct {
	providers []ProviderInfo
	provider  challenge.Provider
	opts      []dns01.ChallengeOption
	err       error
	calls     int
	released  int
}

func (f *fakeSource) Providers() []ProviderInfo { return f.providers }

func (f *fakeSource) NewChallengeProvider(context.Context, string, Configuration, map[string]any) (challenge.Provider, []dns01.ChallengeOption, func(), error) {
	f.calls++
	release := func() { f.released++ }
	if f.err != nil {
		return nil, nil, release, f.err
	}
	return f.provider, f.opts, release, nil
}

type stubProvider struct{}

func (stubProvider) Present(context.Context, string, string, string) error { return nil }
func (stubProvider) CleanUp(context.Context, string, string, string) error { return nil }

// withSources swaps the registry for the duration of one test.
func withSources(t *testing.T, list ...Source) {
	t.Helper()
	sourcesMu.Lock()
	previous := sources
	sources = list
	sourcesMu.Unlock()
	t.Cleanup(func() {
		sourcesMu.Lock()
		sources = previous
		sourcesMu.Unlock()
	})
}

func TestBuiltinProvidersAreRegistered(t *testing.T) {
	list := GetProvidersList()
	if len(list) != len(builtinProviders) {
		t.Fatalf("builtin provider count = %d, want %d", len(list), len(builtinProviders))
	}
	for _, p := range list {
		if !p.RecordManagement || p.DNS01 {
			t.Fatalf("builtin provider %s flags = (record %v, dns01 %v), want (true, false)", p.Code, p.RecordManagement, p.DNS01)
		}
		if p.Configuration != nil || p.Links != nil {
			t.Fatalf("builtin provider %s still carries its schema in the list", p.Code)
		}
	}
	if HasDNS01Providers() {
		t.Fatal("HasDNS01Providers with only the builtin source = true, want false")
	}
}

func TestBuiltinProviderSchemaIsNotShared(t *testing.T) {
	first, ok := GetProvider("cloudflare")
	if !ok {
		t.Fatal("cloudflare provider is missing")
	}
	first.Configuration.Credentials["CF_API_KEY"] = "mutated"

	second, _ := GetProvider("cloudflare")
	if second.Configuration.Credentials["CF_API_KEY"] == "mutated" {
		t.Fatal("the builtin schema is shared between lookups")
	}
}

func TestGetProvidersListMergesByCode(t *testing.T) {
	builtin := &fakeSource{providers: []ProviderInfo{{
		Config:           Config{Name: "Builtin Cloudflare", Code: "cloudflare", Configuration: &Configuration{Credentials: map[string]string{"CF_API_KEY": "from builtin"}}},
		RecordManagement: true,
	}}}
	pluginSource := &fakeSource{providers: []ProviderInfo{
		{
			Config:   Config{Name: "Cloudflare", Code: "cloudflare", Configuration: &Configuration{Credentials: map[string]string{"CF_DNS_API_TOKEN": "from plugin"}}},
			PluginID: "com.nginxui.dns01",
			DNS01:    true,
		},
		{
			Config:   Config{Name: "ACME DNS", Code: "acmedns"},
			PluginID: "com.nginxui.dns01",
			DNS01:    true,
		},
	}}
	withSources(t, builtin, pluginSource)

	list := GetProvidersList()
	if len(list) != 2 {
		t.Fatalf("merged provider count = %d, want 2", len(list))
	}
	if list[0].Code != "acmedns" || list[1].Code != "cloudflare" {
		t.Fatalf("providers are not sorted by name: %s, %s", list[0].Name, list[1].Name)
	}

	merged := list[1]
	if merged.Name != "Cloudflare" {
		t.Fatalf("merged name = %q, want the plugin name", merged.Name)
	}
	if merged.PluginID != "com.nginxui.dns01" {
		t.Fatalf("merged plugin id = %q, want com.nginxui.dns01", merged.PluginID)
	}
	if !merged.DNS01 || !merged.RecordManagement {
		t.Fatalf("merged flags = (dns01 %v, record %v), want both true", merged.DNS01, merged.RecordManagement)
	}

	full, ok := GetProvider("cloudflare")
	if !ok {
		t.Fatal("GetProvider(cloudflare) = false")
	}
	if full.Configuration == nil || full.Configuration.Credentials["CF_DNS_API_TOKEN"] != "from plugin" {
		t.Fatalf("the plugin schema did not win: %#v", full.Configuration)
	}
	if !HasDNS01Providers() {
		t.Fatal("HasDNS01Providers with a plugin source = false, want true")
	}
}

func TestGetProvidersListSortedByName(t *testing.T) {
	list := GetProvidersList()
	for i := 1; i < len(list); i++ {
		left := strings.ToLower(list[i-1].Name)
		right := strings.ToLower(list[i].Name)
		if left > right {
			t.Fatalf("providers are not sorted by name: %q before %q", left, right)
		}
	}
}

func TestNewChallengeProviderDispatchesToTheOwner(t *testing.T) {
	builtin := &fakeSource{providers: []ProviderInfo{{
		Config:           Config{Name: "Cloudflare", Code: "cloudflare"},
		RecordManagement: true,
	}}}
	owner := &fakeSource{
		providers: []ProviderInfo{{Config: Config{Name: "Cloudflare", Code: "cloudflare"}, PluginID: "com.nginxui.dns01", DNS01: true}},
		provider:  stubProvider{},
		opts:      []dns01.ChallengeOption{func(*dns01.Challenge) error { return nil }},
	}
	withSources(t, builtin, owner)

	provider, opts, release, err := NewChallengeProvider(t.Context(), "cloudflare", Configuration{}, nil)
	if err != nil {
		t.Fatalf("NewChallengeProvider error = %v", err)
	}
	if provider == nil || len(opts) != 1 {
		t.Fatalf("provider = %v, options = %d, want a provider and 1 option", provider, len(opts))
	}
	if builtin.calls != 0 {
		t.Fatalf("builtin source calls = %d, want 0", builtin.calls)
	}
	if owner.calls != 1 {
		t.Fatalf("owner source calls = %d, want 1", owner.calls)
	}
	release()
	if owner.released != 1 {
		t.Fatalf("release calls = %d, want 1", owner.released)
	}
}

func TestNewChallengeProviderWithoutOwner(t *testing.T) {
	withSources(t, &fakeSource{providers: []ProviderInfo{{
		Config:           Config{Name: "Cloudflare", Code: "cloudflare"},
		RecordManagement: true,
	}}})

	_, _, release, err := NewChallengeProvider(t.Context(), "cloudflare", Configuration{}, nil)
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("error = %v, want ErrProviderNotFound", err)
	}
	if release == nil {
		t.Fatal("release is nil, want a no-op")
	}
	release()
}

func TestNewChallengeProviderReleasesOnError(t *testing.T) {
	failing := &fakeSource{
		providers: []ProviderInfo{{Config: Config{Name: "Cloudflare", Code: "cloudflare"}, PluginID: "com.nginxui.dns01", DNS01: true}},
		err:       errors.New("plugin is not running"),
	}
	withSources(t, failing)

	_, _, _, err := NewChallengeProvider(t.Context(), "cloudflare", Configuration{}, nil)
	if err == nil || errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("error = %v, want the source error", err)
	}
	if failing.released != 1 {
		t.Fatalf("release calls = %d, want 1", failing.released)
	}
}
