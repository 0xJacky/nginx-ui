// Package blocklist turns lists of addresses to deny, which
// security.blocklist plugins fetch from threat feeds and ban lists, into
// nginx deny rules. It keeps the source kind registry, renders the files and
// refreshes every source on its interval.
package blocklist

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/uozi-tech/cosy"
)

// KindField is one field of the configuration form of a source kind. Values
// travel as strings whatever the type.
type KindField struct {
	Key string `json:"key"`
	// Type is text (default when empty), textarea, number or bool.
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name"`
	HelpText    string `json:"help_text,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// Kind is a kind of source a plugin fetches lists from.
type Kind struct {
	// Kind is the value stored in model.BlocklistSource.Kind.
	Kind string `json:"kind"`
	Name string `json:"name"`
	// PluginID names the plugin behind the kind, when there is one.
	PluginID string      `json:"plugin_id,omitempty"`
	Fields   []KindField `json:"fields"`
	// RefreshSeconds is the default interval of a new source of the kind.
	RefreshSeconds int `json:"refresh_seconds"`
}

// Entry is one address or network a source lists.
type Entry struct {
	CIDR   string
	Reason string
}

// Result is what a source answered.
type Result struct {
	// Entries is the complete list of the source.
	Entries []Entry
	// TTLSeconds asks for an earlier refresh, 0 when the source has no
	// opinion.
	TTLSeconds int
}

// Source offers source kinds that come and go at runtime, such as the kinds
// of security.blocklist plugins.
type Source interface {
	// Kinds lists the source kinds the source serves right now.
	Kinds() []Kind
	// Fetch returns the current list of a source of a kind.
	Fetch(ctx context.Context, kind string, config map[string]string) (Result, error)
}

var (
	sources      []Source
	sourcesMutex sync.RWMutex
)

// RegisterSource adds a source of source kinds.
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

// kindPattern bounds the kinds a source may store, e.g. "plugin:threatfeed".
var kindPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`)

// IsValidKind reports whether kind is well formed. It does not require a
// source to serve it right now: a stored source keeps its kind while the
// plugin behind it is disabled.
func IsValidKind(kind string) bool {
	return kindPattern.MatchString(kind)
}

// Kinds lists the source kinds every source offers, ordered by kind.
func Kinds() []Kind {
	kinds := []Kind{}
	seen := map[string]struct{}{}
	for _, source := range registeredSources() {
		for _, kind := range source.Kinds() {
			if _, dup := seen[kind.Kind]; dup {
				continue
			}
			seen[kind.Kind] = struct{}{}
			if kind.Fields == nil {
				kind.Fields = []KindField{}
			}
			if kind.RefreshSeconds == 0 {
				kind.RefreshSeconds = protocol.DefaultBlocklistRefreshSeconds
			}
			kinds = append(kinds, kind)
		}
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Kind < kinds[j].Kind })
	return kinds
}

// KindOf returns the kind a source serves right now.
func KindOf(kind string) (Kind, bool) {
	for _, offered := range Kinds() {
		if offered.Kind == kind {
			return offered, true
		}
	}
	return Kind{}, false
}

// sourceFor returns the source that serves a kind right now.
func sourceFor(kind string) (Source, Kind, error) {
	for _, source := range registeredSources() {
		for _, offered := range source.Kinds() {
			if offered.Kind == kind {
				return source, offered, nil
			}
		}
	}
	return nil, Kind{}, cosy.WrapErrorWithParams(plugin.ErrBlocklistKindUnavailable, kind)
}

// ValidateConfig checks a source configuration before it is stored: the
// kind is served and every required field is filled in.
func ValidateConfig(kind string, config map[string]string) error {
	if !IsValidKind(kind) {
		return cosy.WrapErrorWithParams(plugin.ErrBlocklistKindUnavailable, kind)
	}
	_, offered, err := sourceFor(kind)
	if err != nil {
		return err
	}
	for _, field := range offered.Fields {
		if field.Required && strings.TrimSpace(config[field.Key]) == "" {
			return cosy.WrapErrorWithParams(plugin.ErrBlocklistConfigInvalid, field.Key, "it is required")
		}
	}
	return nil
}

// Fetch asks the source that serves a kind right now for its list.
func Fetch(ctx context.Context, kind string, config map[string]string) (Result, error) {
	source, _, err := sourceFor(kind)
	if err != nil {
		return Result{}, err
	}
	return source.Fetch(ctx, kind, config)
}
