// Package deploy pushes issued certificates to external targets, such as a
// CDN or a load balancer, through the target kinds cert.deploy plugins offer.
// It keeps the target kind registry, reads the certificate for a push and
// runs the pushes with retries.
package deploy

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/uozi-tech/cosy"
)

// KindField is one field of the configuration form of a target kind. Values
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

// TargetKind is a kind of external target a source can push certificates to.
type TargetKind struct {
	// Kind is the value stored in model.CertDeployTarget.Kind.
	Kind string `json:"kind"`
	Name string `json:"name"`
	// PluginID names the plugin behind the kind, when there is one.
	PluginID string      `json:"plugin_id,omitempty"`
	Fields   []KindField `json:"fields"`
}

// Certificate is what a push carries.
type Certificate struct {
	Name    string
	Domains []string
	// CertificatePEM is the leaf certificate.
	CertificatePEM string
	// PrivateKeyPEM is the private key of the leaf.
	PrivateKeyPEM string
	// ChainPEM holds the intermediates, starting with the issuer of the leaf.
	ChainPEM string
	NotAfter time.Time
}

// Source offers target kinds that come and go at runtime, such as the kinds
// of cert.deploy plugins.
type Source interface {
	// Kinds lists the target kinds the source serves right now.
	Kinds() []TargetKind
	// Validate checks a configuration without contacting the target. It
	// returns nil when it has no opinion.
	Validate(ctx context.Context, kind string, config map[string]string) error
	// Push pushes a certificate to a target and returns what the target
	// reported. A dry run changes nothing.
	Push(ctx context.Context, kind string, config map[string]string, cert Certificate, dryRun bool) (string, error)
}

var (
	sources      []Source
	sourcesMutex sync.RWMutex
)

// RegisterSource adds a source of target kinds.
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

// kindPattern bounds the kinds a target may store, e.g. "plugin:mycdn".
var kindPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`)

// IsValidKind reports whether kind is well formed. It does not require a
// source to serve it right now: a target keeps its kind while the plugin
// behind it is disabled.
func IsValidKind(kind string) bool {
	return kindPattern.MatchString(kind)
}

// Kinds lists the target kinds every source offers, ordered by kind.
func Kinds() []TargetKind {
	kinds := []TargetKind{}
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
			kinds = append(kinds, kind)
		}
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Kind < kinds[j].Kind })
	return kinds
}

// sourceFor returns the source that serves a kind right now and the kind it
// describes.
func sourceFor(kind string) (Source, TargetKind, error) {
	for _, source := range registeredSources() {
		for _, offered := range source.Kinds() {
			if offered.Kind == kind {
				return source, offered, nil
			}
		}
	}
	return nil, TargetKind{}, cosy.WrapErrorWithParams(plugin.ErrDeployKindUnavailable, kind)
}

// ValidateConfig checks a target configuration before it is stored: the kind
// is served, every required field is filled in, and the source accepts the
// values.
func ValidateConfig(ctx context.Context, kind string, config map[string]string) error {
	if !IsValidKind(kind) {
		return cosy.WrapErrorWithParams(plugin.ErrDeployKindUnavailable, kind)
	}
	source, offered, err := sourceFor(kind)
	if err != nil {
		return err
	}
	for _, field := range offered.Fields {
		if field.Required && strings.TrimSpace(config[field.Key]) == "" {
			return cosy.WrapErrorWithParams(plugin.ErrDeployConfigInvalid, field.Key, "it is required")
		}
	}
	return source.Validate(ctx, kind, config)
}

// Push pushes a certificate to a target of a kind through the source that
// serves the kind right now.
func Push(ctx context.Context, kind string, config map[string]string, cert Certificate, dryRun bool) (string, error) {
	source, _, err := sourceFor(kind)
	if err != nil {
		return "", err
	}
	return source.Push(ctx, kind, config, cert, dryRun)
}
