package capability

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
)

const (
	// probeDefaultTimeout applies when the site sets no timeout.
	probeDefaultTimeout = 10 * time.Second
	// probeCallSlack is what the host waits beyond timeout_seconds, so a
	// plugin that reports a slow target as down still gets to answer.
	probeCallSlack = 5 * time.Second
)

// ProbeHost is the part of the plugin manager the probe capability needs.
type ProbeHost interface {
	// OwnerOf returns the plugin that serves a kind code.
	OwnerOf(capability, code string) (pluginID string, ok bool)
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// ProbeKinds lists what every enabled probe plugin offers.
	ProbeKinds() []plugin.ProbeKindEntry
}

// RegisterProbe offers the kinds of every enabled probe plugin as site health
// check kinds, next to the built-in HTTP check.
func RegisterProbe(h ProbeHost) {
	sitecheck.RegisterProbeSource(NewProbeSource(h))
}

// NewProbeSource exposes the probe capability of the plugins of h as a probe
// source. A kind code is published as PluginType(code).
func NewProbeSource(h ProbeHost) sitecheck.ProbeSource {
	return &probeSource{host: h}
}

type probeSource struct {
	host ProbeHost
}

// ProbeKinds lists every kind once, served by the plugin that owns its code.
func (s *probeSource) ProbeKinds() []sitecheck.ProbeKind {
	entries := s.host.ProbeKinds()
	kinds := make([]sitecheck.ProbeKind, 0, len(entries))
	for _, entry := range entries {
		if owner, ok := s.host.OwnerOf(protocol.CapabilityProbe, entry.Kind.Code); !ok || owner != entry.PluginID {
			continue
		}
		fields := configurationFields(entry.Kind.Configuration)
		kind := sitecheck.ProbeKind{
			Kind:     PluginType(entry.Kind.Code),
			Name:     entry.Kind.Name,
			PluginID: entry.PluginID,
			Fields:   make([]sitecheck.ProbeField, 0, len(fields)),
		}
		for _, field := range fields {
			kind.Fields = append(kind.Fields, sitecheck.ProbeField{
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

// Probe runs probe.check on the plugin that owns the kind.
func (s *probeSource) Probe(ctx context.Context, kind string, req sitecheck.ProbeRequest) (sitecheck.ProbeOutcome, error) {
	code, ok := pluginCode(kind)
	if !ok {
		return sitecheck.ProbeOutcome{}, fmt.Errorf("%w: %s", sitecheck.ErrProbeKindUnavailable, kind)
	}
	pluginID, ok := s.host.OwnerOf(protocol.CapabilityProbe, code)
	if !ok {
		return sitecheck.ProbeOutcome{}, fmt.Errorf("%w: %s", sitecheck.ErrProbeKindUnavailable, kind)
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = probeDefaultTimeout
	}
	timeoutSeconds := int(math.Ceil(timeout.Seconds()))

	caller, release, err := s.host.Acquire(ctx, pluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		return sitecheck.ProbeOutcome{}, plugin.WrapRPCError(err)
	}

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second+probeCallSlack)
	defer cancel()

	var result protocol.ProbeCheckResult
	err = caller.Call(callCtx, protocol.MethodProbeCheck, protocol.ProbeCheckParams{
		Kind:           code,
		Target:         req.Target,
		Config:         req.Config,
		TimeoutSeconds: timeoutSeconds,
	}, &result)
	if err != nil {
		if field, message, invalid := invalidConfigField(err); invalid {
			return sitecheck.ProbeOutcome{}, fmt.Errorf("probe field %s is invalid: %s", field, message)
		}
		return sitecheck.ProbeOutcome{}, fmt.Errorf("probe %s failed: %s", kind, rpcMessage(err))
	}

	latency := time.Duration(max(result.LatencyMS, 0)) * time.Millisecond
	return sitecheck.ProbeOutcome{Status: result.Status, Latency: latency, Message: result.Message}, nil
}
