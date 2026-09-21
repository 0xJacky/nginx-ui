package dns

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
)

// OfficialDNS01PluginID is the plugin the core points at when DNS-01 is
// requested and no source can solve it.
const OfficialDNS01PluginID = "com.nginxui.dns01"

// ErrProviderNotFound is returned when no registered source owns a code.
var ErrProviderNotFound = errors.New("dns: provider not found")

// ProviderInfo is what the credential page and the challenge form see.
type ProviderInfo struct {
	Config
	PluginID         string `json:"plugin_id,omitempty"`
	DNS01            bool   `json:"dns01"`
	RecordManagement bool   `json:"record_management"`
}

// Source contributes provider schemas and, when it can solve DNS-01, the lego
// challenge provider behind them.
type Source interface {
	// Providers lists what the source offers right now. It is called on every
	// lookup, so a plugin source reflects enable and disable immediately.
	Providers() []ProviderInfo
	// NewChallengeProvider builds the lego provider for code. The returned
	// func is always safe to call and releases whatever the source holds for
	// the duration of the issuance. Sources that do not own code return
	// ErrProviderNotFound.
	NewChallengeProvider(ctx context.Context, code string, cfg Configuration, options map[string]any) (challenge.Provider, []dns01.ChallengeOption, func(), error)
}

var (
	sourcesMu sync.RWMutex
	sources   []Source
)

// RegisterSource adds a source. Registration is additive and goroutine safe:
// the builtin source registers at init, plugin sources register when the
// plugin manager starts.
func RegisterSource(s Source) {
	if s == nil {
		return
	}
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	sources = append(sources, s)
}

func registeredSources() []Source {
	sourcesMu.RLock()
	defer sourcesMu.RUnlock()
	return slices.Clone(sources)
}

// GetProvidersList returns every provider, merged by code and sorted by name.
// Schemas are stripped: the list endpoint only carries what the picker needs,
// the detail endpoint returns the full entry.
func GetProvidersList() []ProviderInfo {
	list := mergedProviders()
	for i := range list {
		list[i].Configuration = nil
		list[i].Links = nil
	}
	return list
}

// GetProvider returns one merged provider with its schema.
func GetProvider(code string) (ProviderInfo, bool) {
	if code == "" {
		return ProviderInfo{}, false
	}
	for _, p := range mergedProviders() {
		if p.Code == code {
			return p, true
		}
	}
	return ProviderInfo{}, false
}

// HasDNS01Providers reports whether anything can solve the DNS-01 challenge.
func HasDNS01Providers() bool {
	for _, s := range registeredSources() {
		for _, p := range s.Providers() {
			if p.DNS01 {
				return true
			}
		}
	}
	return false
}

// NewChallengeProvider dispatches to the source that owns code.
func NewChallengeProvider(ctx context.Context, code string, cfg Configuration, options map[string]any) (challenge.Provider, []dns01.ChallengeOption, func(), error) {
	noop := func() {}
	for _, s := range registeredSources() {
		if !ownsDNS01(s, code) {
			continue
		}
		provider, opts, release, err := s.NewChallengeProvider(ctx, code, cfg, options)
		if release == nil {
			release = noop
		}
		if err != nil {
			release()
			// Another source may still own the code, for instance while a
			// plugin is being replaced.
			if errors.Is(err, ErrProviderNotFound) {
				continue
			}
			return nil, nil, noop, err
		}
		return provider, opts, release, nil
	}
	return nil, nil, noop, ErrProviderNotFound
}

func ownsDNS01(s Source, code string) bool {
	if code == "" {
		return false
	}
	for _, p := range s.Providers() {
		if p.Code == code && p.DNS01 {
			return true
		}
	}
	return false
}

// mergedProviders folds every source into one list keyed by code. A plugin
// entry wins the schema, the capability flags are the union of all entries.
func mergedProviders() []ProviderInfo {
	byCode := make(map[string]int)
	var merged []ProviderInfo

	for _, s := range registeredSources() {
		for _, p := range s.Providers() {
			if p.Code == "" {
				continue
			}
			index, ok := byCode[p.Code]
			if !ok {
				byCode[p.Code] = len(merged)
				merged = append(merged, p)
				continue
			}

			current := merged[index]
			dns01Flag := current.DNS01 || p.DNS01
			recordFlag := current.RecordManagement || p.RecordManagement
			if current.PluginID == "" && p.PluginID != "" {
				current = p
			} else if current.Name == "" {
				current.Name = p.Name
			}
			current.DNS01 = dns01Flag
			current.RecordManagement = recordFlag
			merged[index] = current
		}
	}

	slices.SortStableFunc(merged, func(a, b ProviderInfo) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Code), strings.ToLower(b.Code))
	})

	return merged
}
