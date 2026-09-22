// Package discovery keeps nginx upstream blocks in step with the servers
// upstream.discovery plugins resolve from service registries and
// orchestrators. It keeps the provider registry, renders the files and
// refreshes every binding on its interval.
package discovery

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/uozi-tech/cosy"
)

// ProviderField is one field of the configuration form of a provider. Values
// travel as strings whatever the type.
type ProviderField struct {
	Key string `json:"key"`
	// Type is text (default when empty), textarea, number or bool.
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name"`
	HelpText    string `json:"help_text,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// Provider is a place services are resolved from.
type Provider struct {
	// Kind is the value stored in model.UpstreamDiscovery.Kind.
	Kind string `json:"kind"`
	Name string `json:"name"`
	// PluginID names the plugin behind the provider, when there is one.
	PluginID string          `json:"plugin_id,omitempty"`
	Fields   []ProviderField `json:"fields"`
}

// Target is one server a provider answered.
type Target struct {
	Address string
	Port    int
	Weight  int
	Tags    []string
}

// Result is what a provider answered.
type Result struct {
	// Targets lists every server of the service.
	Targets []Target
	// TTLSeconds asks for an earlier refresh, 0 when the provider has no
	// opinion.
	TTLSeconds int
}

// Source offers providers that come and go at runtime, such as the
// providers of upstream.discovery plugins.
type Source interface {
	// Providers lists the providers the source serves right now.
	Providers() []Provider
	// Resolve returns the servers of a service.
	Resolve(ctx context.Context, kind string, config map[string]string, service string) (Result, error)
}

var (
	sources      []Source
	sourcesMutex sync.RWMutex
)

// RegisterSource adds a source of providers.
func RegisterSource(source Source) {
	sourcesMutex.Lock()
	defer sourcesMutex.Unlock()
	sources = append(sources, source)
}

func registeredSources() []Source {
	sourcesMutex.RLock()
	defer sourcesMutex.RUnlock()
	return append([]Source(nil), sources...)
}

// kindPattern bounds the kinds a binding may store, e.g. "plugin:registry".
var kindPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`)

// IsValidKind reports whether kind is well formed. It does not require a
// source to serve it right now.
func IsValidKind(kind string) bool {
	return kindPattern.MatchString(kind)
}

// Providers lists the providers every source offers, ordered by kind.
func Providers() []Provider {
	providers := []Provider{}
	seen := map[string]struct{}{}
	for _, source := range registeredSources() {
		for _, provider := range source.Providers() {
			if _, dup := seen[provider.Kind]; dup {
				continue
			}
			seen[provider.Kind] = struct{}{}
			if provider.Fields == nil {
				provider.Fields = []ProviderField{}
			}
			providers = append(providers, provider)
		}
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Kind < providers[j].Kind })
	return providers
}

// sourceFor returns the source that serves a provider right now.
func sourceFor(kind string) (Source, Provider, error) {
	for _, source := range registeredSources() {
		for _, offered := range source.Providers() {
			if offered.Kind == kind {
				return source, offered, nil
			}
		}
	}
	return nil, Provider{}, cosy.WrapErrorWithParams(plugin.ErrDiscoveryKindUnavailable, kind)
}

// ValidateConfig checks a binding configuration before it is stored: the
// provider is served and every required field is filled in.
func ValidateConfig(kind string, config map[string]string) error {
	if !IsValidKind(kind) {
		return cosy.WrapErrorWithParams(plugin.ErrDiscoveryKindUnavailable, kind)
	}
	_, offered, err := sourceFor(kind)
	if err != nil {
		return err
	}
	for _, field := range offered.Fields {
		if field.Required && strings.TrimSpace(config[field.Key]) == "" {
			return cosy.WrapErrorWithParams(plugin.ErrDiscoveryConfigInvalid, field.Key, "it is required")
		}
	}
	return nil
}

// Resolve asks the source that serves a provider right now for the servers
// of a service.
func Resolve(ctx context.Context, kind string, config map[string]string, service string) (Result, error) {
	source, _, err := sourceFor(kind)
	if err != nil {
		return Result{}, err
	}
	return source.Resolve(ctx, kind, config, service)
}
