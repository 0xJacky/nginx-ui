package capability

import (
	"context"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/security/blocklist"
	"github.com/uozi-tech/cosy"
)

// blocklistFetchTimeout bounds one fetch, including starting an on_demand
// plugin.
const blocklistFetchTimeout = 60 * time.Second

// BlocklistHost is the part of the plugin manager the security.blocklist
// capability needs.
type BlocklistHost interface {
	// OwnerOf returns the plugin that serves a source kind code.
	OwnerOf(capability, code string) (pluginID string, ok bool)
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// BlocklistSources lists what every enabled security.blocklist plugin
	// offers.
	BlocklistSources() []plugin.BlocklistSourceEntry
}

// RegisterBlocklist offers the source kinds of every enabled
// security.blocklist plugin to the blocklist sources.
func RegisterBlocklist(h BlocklistHost) {
	blocklist.RegisterSource(NewBlocklistSource(h))
}

// NewBlocklistSource exposes the security.blocklist capability of the
// plugins of h as a source kind registry. A kind code is published as
// PluginType(code).
func NewBlocklistSource(h BlocklistHost) blocklist.Source {
	return &blocklistSource{host: h}
}

type blocklistSource struct {
	host BlocklistHost
}

// Kinds lists every source kind once, served by the plugin that owns its
// code.
func (s *blocklistSource) Kinds() []blocklist.Kind {
	entries := s.host.BlocklistSources()
	kinds := make([]blocklist.Kind, 0, len(entries))
	for _, entry := range entries {
		if owner, ok := s.host.OwnerOf(protocol.CapabilitySecurityBlocklist, entry.Source.Code); !ok || owner != entry.PluginID {
			continue
		}
		fields := configurationFields(entry.Source.Configuration)
		kind := blocklist.Kind{
			Kind:           PluginType(entry.Source.Code),
			Name:           entry.Source.Name,
			PluginID:       entry.PluginID,
			Fields:         make([]blocklist.KindField, 0, len(fields)),
			RefreshSeconds: entry.Source.RefreshSeconds,
		}
		if kind.RefreshSeconds == 0 {
			kind.RefreshSeconds = protocol.DefaultBlocklistRefreshSeconds
		}
		for _, field := range fields {
			kind.Fields = append(kind.Fields, blocklist.KindField{
				Key:         field.Key,
				Type:        field.Type,
				DisplayName: field.DisplayName,
				HelpText:    field.HelpText,
				Required:    field.Required,
				Secret:      field.Secret,
			})
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

// Fetch runs blocklist.fetch on the plugin that owns the kind right now.
func (s *blocklistSource) Fetch(ctx context.Context, kind string, config map[string]string) (blocklist.Result, error) {
	code, ok := pluginCode(kind)
	if !ok {
		return blocklist.Result{}, cosy.WrapErrorWithParams(plugin.ErrBlocklistKindUnavailable, kind)
	}
	pluginID, ok := s.host.OwnerOf(protocol.CapabilitySecurityBlocklist, code)
	if !ok {
		return blocklist.Result{}, cosy.WrapErrorWithParams(plugin.ErrBlocklistKindUnavailable, kind)
	}

	callCtx, cancel := context.WithTimeout(ctx, blocklistFetchTimeout)
	defer cancel()

	caller, release, err := s.host.Acquire(callCtx, pluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		return blocklist.Result{}, plugin.WrapRPCError(err)
	}

	var reply protocol.BlocklistFetchResult
	err = caller.Call(callCtx, protocol.MethodBlocklistFetch, protocol.BlocklistFetchParams{
		Source: code,
		Config: config,
	}, &reply)
	if err != nil {
		if field, message, invalid := invalidConfigField(err); invalid {
			return blocklist.Result{}, cosy.WrapErrorWithParams(plugin.ErrBlocklistConfigInvalid, field, message)
		}
		return blocklist.Result{}, plugin.WrapRPCError(err)
	}

	result := blocklist.Result{Entries: make([]blocklist.Entry, 0, len(reply.Entries)), TTLSeconds: reply.TTLSeconds}
	for _, entry := range reply.Entries {
		result.Entries = append(result.Entries, blocklist.Entry{CIDR: entry.CIDR, Reason: entry.Reason})
	}
	return result, nil
}
