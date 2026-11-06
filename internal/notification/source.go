package notification

import (
	"context"
	"sort"
	"sync"
)

// ExternalNotifierField is one field of the configuration form of a notifier
// type a source offers. Values travel as strings whatever the type.
type ExternalNotifierField struct {
	Key string `json:"key"`
	// Type is text (default when empty), textarea, number or bool.
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name"`
	HelpText    string `json:"help_text,omitempty"`
	Required    bool   `json:"required,omitempty"`
	// Secret marks a credential the form masks.
	Secret bool `json:"secret,omitempty"`
}

// ExternalNotifierChannel is a notifier type a source offers next to the
// built-in ones, together with the schema of its configuration form.
type ExternalNotifierChannel struct {
	// Type is the value stored in model.ExternalNotify.Type.
	Type string `json:"type"`
	Name string `json:"name"`
	// PluginID names the plugin behind the channel, when there is one.
	PluginID string                  `json:"plugin_id,omitempty"`
	Fields   []ExternalNotifierField `json:"fields"`
}

// ExternalNotifierSource offers notifier types that come and go at runtime,
// such as the channels of notify plugins. The built-in notifiers registered
// with RegisterExternalNotifier always take precedence.
type ExternalNotifierSource interface {
	// Channels lists the notifier types the source serves right now.
	Channels() []ExternalNotifierChannel
	// Handler returns the handler of a notifier type, false when the source
	// does not serve it right now.
	Handler(notifierType string) (ExternalNotifierHandlerFunc, bool)
	// Validate checks a configuration without sending anything. It returns
	// nil for a type the source does not serve and when it has no opinion.
	Validate(ctx context.Context, notifierType string, config map[string]string) error
}

var (
	externalNotifierSources      []ExternalNotifierSource
	externalNotifierSourcesMutex sync.RWMutex
)

// RegisterExternalNotifierSource adds a source of notifier types.
func RegisterExternalNotifierSource(source ExternalNotifierSource) {
	externalNotifierSourcesMutex.Lock()
	defer externalNotifierSourcesMutex.Unlock()
	externalNotifierSources = append(externalNotifierSources, source)
}

func notifierSources() []ExternalNotifierSource {
	externalNotifierSourcesMutex.RLock()
	defer externalNotifierSourcesMutex.RUnlock()
	return append([]ExternalNotifierSource(nil), externalNotifierSources...)
}

// sourceHandler asks the registered sources for a notifier type the built-in
// registry does not know.
func sourceHandler(notifierType string) (ExternalNotifierHandlerFunc, bool) {
	for _, source := range notifierSources() {
		if handler, ok := source.Handler(notifierType); ok {
			return handler, true
		}
	}
	return nil, false
}

// ExternalNotifierChannels lists the notifier types every source offers,
// ordered by type. A type that shadows a built-in notifier is left out, since
// it could never be used.
func ExternalNotifierChannels() []ExternalNotifierChannel {
	channels := []ExternalNotifierChannel{}
	seen := map[string]struct{}{}
	for _, source := range notifierSources() {
		for _, channel := range source.Channels() {
			if _, builtin := builtinHandler(channel.Type); builtin {
				continue
			}
			if _, dup := seen[channel.Type]; dup {
				continue
			}
			seen[channel.Type] = struct{}{}
			if channel.Fields == nil {
				channel.Fields = []ExternalNotifierField{}
			}
			channels = append(channels, channel)
		}
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].Type < channels[j].Type })
	return channels
}

// ValidateExternalNotifierConfig lets the source of a notifier type reject a
// configuration before it is stored. Built-in types are not checked here.
func ValidateExternalNotifierConfig(ctx context.Context, notifierType string, config map[string]string) error {
	if _, builtin := builtinHandler(notifierType); builtin {
		return nil
	}
	for _, source := range notifierSources() {
		if err := source.Validate(ctx, notifierType, config); err != nil {
			return err
		}
	}
	return nil
}
