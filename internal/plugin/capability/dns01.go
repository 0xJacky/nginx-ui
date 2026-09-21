// Package capability adapts the plugin capabilities to the core interfaces
// that consume them. The dns01 capability is exposed as a dns.Source, so the
// certificate code never learns that a provider lives in another process.
package capability

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
	legolog "github.com/go-acme/lego/v5/log"
)

const (
	// dns01MetadataTimeout bounds the metadata calls, which never touch the
	// vendor API.
	dns01MetadataTimeout = 30 * time.Second
	// dns01MinCallTimeout is the floor for present and cleanup. A vendor API
	// that is slower than the propagation timeout still gets a fair chance.
	dns01MinCallTimeout = 2 * time.Minute
)

// DNS01ProviderEntry is one vendor a running plugin can solve DNS-01 for.
type DNS01ProviderEntry struct {
	PluginID string
	Provider protocol.DNS01Provider
}

// Host is the part of the plugin manager the capability adapters need.
type Host interface {
	// OwnerOf returns the plugin that claims a capability code.
	OwnerOf(capability, code string) (pluginID string, ok bool)
	// Acquire returns a client for a plugin and a func that releases it. The
	// returned func is safe to call even when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// Manifest returns the manifest of an installed and enabled plugin.
	Manifest(pluginID string) (*protocol.Manifest, bool)
	// DNS01Providers lists what every enabled dns01 plugin offers.
	DNS01Providers() []DNS01ProviderEntry
}

// NewDNS01Source exposes the dns01 capability of every enabled plugin as a
// provider source.
func NewDNS01Source(h Host) dns.Source {
	return &dns01Source{host: h}
}

type dns01Source struct {
	host Host
}

func (s *dns01Source) Providers() []dns.ProviderInfo {
	entries := s.host.DNS01Providers()
	out := make([]dns.ProviderInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Provider.Code == "" {
			continue
		}
		out = append(out, dns.ProviderInfo{
			Config: dns.Config{
				Name:          entry.Provider.Name,
				Code:          entry.Provider.Code,
				Configuration: toConfiguration(entry.Provider.Configuration),
				Links:         toLinks(entry.Provider.Links),
			},
			PluginID: entry.PluginID,
			DNS01:    true,
		})
	}
	return out
}

func (s *dns01Source) NewChallengeProvider(ctx context.Context, code string, cfg dns.Configuration, options map[string]any) (challenge.Provider, []dns01.ChallengeOption, func(), error) {
	noop := func() {}

	pluginID, ok := s.host.OwnerOf(protocol.CapabilityDNS01, code)
	if !ok {
		return nil, nil, noop, dns.ErrProviderNotFound
	}

	caller, release, err := s.host.Acquire(ctx, pluginID)
	if err != nil {
		if release != nil {
			release()
		}
		return nil, nil, noop, plugin.WrapRPCError(err)
	}
	if release == nil {
		release = noop
	}

	config := mergeConfiguration(cfg)
	timings := s.timings(ctx, caller, pluginID, code, config, options)

	p := &challengeProvider{
		host:     s.host,
		caller:   caller,
		pluginID: pluginID,
		code:     code,
		config:   config,
		options:  options,
		timeout:  timings.propagation,
		interval: timings.polling,
	}

	opts := []dns01.ChallengeOption{p.preCheckOption()}

	var provider challenge.Provider = p
	if timings.sequential > 0 {
		provider = &sequentialChallengeProvider{challengeProvider: p, sequential: timings.sequential}
	}

	return provider, opts, release, nil
}

// dns01Timings is what lego needs to know about a provider before it starts.
type dns01Timings struct {
	propagation time.Duration
	polling     time.Duration
	sequential  time.Duration
}

// timings asks the plugin how long the vendor needs, falling back to the
// manifest defaults and then to lego's own defaults.
func (s *dns01Source) timings(ctx context.Context, caller jsonrpc.Caller, pluginID, code string, config map[string]string, options map[string]any) dns01Timings {
	timings := dns01Timings{
		propagation: dns01.DefaultPropagationTimeout,
		polling:     dns01.DefaultPollingInterval,
	}

	if declared, ok := s.manifestProvider(pluginID, code); ok {
		if declared.PropagationTimeoutSeconds > 0 {
			timings.propagation = time.Duration(declared.PropagationTimeoutSeconds) * time.Second
		}
		if declared.PollingIntervalSeconds > 0 {
			timings.polling = time.Duration(declared.PollingIntervalSeconds) * time.Second
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, dns01MetadataTimeout)
	defer cancel()

	var result protocol.DNS01OptionsResult
	err := caller.Call(callCtx, protocol.MethodDNS01Options, protocol.DNS01OptionsParams{
		Provider: code,
		Config:   config,
		Options:  options,
	}, &result)
	if err != nil {
		if !jsonrpc.IsMethodNotFound(err) && !jsonrpc.IsUnsupported(err) {
			legolog.Warn("dns01: the plugin did not report its propagation timings",
				slog.String("plugin", pluginID),
				slog.String("provider", code),
				slog.String("error", err.Error()))
		}
		return timings
	}

	if result.PropagationTimeoutSeconds > 0 {
		timings.propagation = time.Duration(result.PropagationTimeoutSeconds) * time.Second
	}
	if result.PollingIntervalSeconds > 0 {
		timings.polling = time.Duration(result.PollingIntervalSeconds) * time.Second
	}
	if result.SequentialIntervalSeconds > 0 {
		timings.sequential = time.Duration(result.SequentialIntervalSeconds) * time.Second
	}
	return timings
}

func (s *dns01Source) manifestProvider(pluginID, code string) (protocol.DNS01Provider, bool) {
	manifest, ok := s.host.Manifest(pluginID)
	if !ok || manifest == nil || manifest.DNS01 == nil {
		return protocol.DNS01Provider{}, false
	}
	for _, p := range manifest.DNS01.Providers {
		if p.Code == code {
			return p, true
		}
	}
	return protocol.DNS01Provider{}, false
}

// challengeProvider is the lego provider that forwards every challenge
// operation to the owning plugin process.
type challengeProvider struct {
	host     Host
	caller   jsonrpc.Caller
	pluginID string
	code     string
	config   map[string]string
	options  map[string]any
	timeout  time.Duration
	interval time.Duration

	// ownerCannotCheck and officialCannotCheck remember that a plugin does not
	// implement dns01.check, so the polling loop stops asking.
	ownerCannotCheck    atomic.Bool
	officialCannotCheck atomic.Bool
	warnCoreCheck       sync.Once
}

func (p *challengeProvider) Present(ctx context.Context, domain, token, keyAuth string) error {
	return p.solve(ctx, protocol.MethodDNS01Present, domain, token, keyAuth)
}

func (p *challengeProvider) CleanUp(ctx context.Context, domain, token, keyAuth string) error {
	return p.solve(ctx, protocol.MethodDNS01Cleanup, domain, token, keyAuth)
}

// Timeout is what lego waits for the record to propagate.
func (p *challengeProvider) Timeout() (timeout, interval time.Duration) {
	return p.timeout, p.interval
}

func (p *challengeProvider) solve(ctx context.Context, method, domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(ctx, domain, keyAuth)

	callCtx, cancel := context.WithTimeout(ctx, p.callTimeout())
	defer cancel()

	err := p.caller.Call(callCtx, method, protocol.DNS01ChallengeParams{
		Provider:      p.code,
		Config:        p.config,
		Options:       p.options,
		Domain:        domain,
		FQDN:          info.FQDN,
		EffectiveFQDN: info.EffectiveFQDN,
		Value:         info.Value,
		Token:         token,
		KeyAuth:       keyAuth,
	}, nil)
	if err != nil {
		return plugin.WrapRPCError(err)
	}
	return nil
}

func (p *challengeProvider) callTimeout() time.Duration {
	if p.timeout > dns01MinCallTimeout {
		return p.timeout
	}
	return dns01MinCallTimeout
}

func (p *challengeProvider) preCheckOption() dns01.ChallengeOption {
	return dns01.WrapPreCheck(p.preCheckFunc())
}

// preCheckFunc routes the propagation check to a plugin. When no plugin can
// answer it falls back to lego's own check, which queries DNS from the core.
func (p *challengeProvider) preCheckFunc() dns01.WrapPreCheckFunc {
	return func(ctx context.Context, domain, fqdn, value string, check dns01.PreCheckFunc) (bool, error) {
		ready, handled, err := p.remoteCheck(ctx, domain, fqdn, value)
		if handled {
			return ready, err
		}

		p.warnCoreCheck.Do(func() {
			legolog.Warn("dns01: no plugin can check the record propagation, nginx-ui is querying DNS itself",
				slog.String("provider", p.code))
		})
		return check(ctx, fqdn, value)
	}
}

// remoteCheck asks the owning plugin and then the official one. handled is
// false when neither implements dns01.check.
func (p *challengeProvider) remoteCheck(ctx context.Context, domain, fqdn, value string) (ready, handled bool, err error) {
	params := protocol.DNS01CheckParams{
		Provider: p.code,
		Config:   p.config,
		Options:  p.options,
		Domain:   domain,
		FQDN:     fqdn,
		Value:    value,
	}

	if !p.ownerCannotCheck.Load() {
		result, err := callCheck(ctx, p.caller, params)
		switch {
		case err == nil:
			return reportCheck(result), true, nil
		case isUnimplemented(err):
			p.ownerCannotCheck.Store(true)
		default:
			return false, true, plugin.WrapRPCError(err)
		}
	}

	if p.pluginID == dns.OfficialDNS01PluginID || p.officialCannotCheck.Load() {
		return false, false, nil
	}

	if _, ok := p.host.Manifest(dns.OfficialDNS01PluginID); !ok {
		p.officialCannotCheck.Store(true)
		return false, false, nil
	}

	caller, release, err := p.host.Acquire(ctx, dns.OfficialDNS01PluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		p.officialCannotCheck.Store(true)
		return false, false, nil
	}

	result, err := callCheck(ctx, caller, params)
	switch {
	case err == nil:
		return reportCheck(result), true, nil
	case isUnimplemented(err):
		p.officialCannotCheck.Store(true)
		return false, false, nil
	default:
		return false, true, plugin.WrapRPCError(err)
	}
}

func callCheck(ctx context.Context, caller jsonrpc.Caller, params protocol.DNS01CheckParams) (protocol.DNS01CheckResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, dns01MetadataTimeout)
	defer cancel()

	var result protocol.DNS01CheckResult
	if err := caller.Call(callCtx, protocol.MethodDNS01Check, params, &result); err != nil {
		return protocol.DNS01CheckResult{}, err
	}
	return result, nil
}

// reportCheck surfaces why a record is not visible yet in the issuance log.
func reportCheck(result protocol.DNS01CheckResult) bool {
	if !result.Ready && result.Detail != "" {
		legolog.Info("dns01: the record has not propagated yet", slog.String("detail", result.Detail))
	}
	return result.Ready
}

func isUnimplemented(err error) bool {
	return jsonrpc.IsMethodNotFound(err) || jsonrpc.IsUnsupported(err)
}

// sequentialChallengeProvider is what lego looks for when a vendor cannot
// handle two challenges of one certificate at the same time.
type sequentialChallengeProvider struct {
	*challengeProvider
	sequential time.Duration
}

func (p *sequentialChallengeProvider) Sequential() time.Duration { return p.sequential }

// mergeConfiguration flattens the credential form into the single map the
// wire contract carries.
func mergeConfiguration(cfg dns.Configuration) map[string]string {
	config := make(map[string]string, len(cfg.Credentials)+len(cfg.Additional))
	maps.Copy(config, cfg.Credentials)
	maps.Copy(config, cfg.Additional)
	return config
}

func toConfiguration(c *protocol.DNS01ProviderConfig) *dns.Configuration {
	if c == nil {
		return nil
	}
	return &dns.Configuration{
		Credentials: maps.Clone(c.Credentials),
		Additional:  maps.Clone(c.Additional),
	}
}

func toLinks(l *protocol.DNS01ProviderLinks) *dns.Links {
	if l == nil {
		return nil
	}
	return &dns.Links{API: l.API, GoClient: l.GoClient}
}
