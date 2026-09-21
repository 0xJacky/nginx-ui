package protocol

// DNS01ChallengeParams is the payload of dns01.present and dns01.cleanup.
type DNS01ChallengeParams struct {
	// Provider is the vendor code declared in the manifest, without the plugin id prefix.
	Provider string `json:"provider"`
	// Config merges the credential and additional fields the user filled in.
	Config map[string]string `json:"config"`
	// Options is the per-certificate DNS-01 configuration produced by the
	// challenge form slot. Opaque to the host.
	Options map[string]any `json:"options,omitempty"`
	// Domain is the certificate identifier with any leading wildcard removed.
	Domain string `json:"domain"`
	// FQDN is the _acme-challenge name derived by lego, without CNAME following.
	FQDN string `json:"fqdn"`
	// EffectiveFQDN is the CNAME target when known, otherwise equal to FQDN.
	EffectiveFQDN string `json:"effective_fqdn"`
	// Value is the TXT record value derived from KeyAuth.
	Value string `json:"value"`
	// Token is the raw ACME challenge token.
	Token string `json:"token"`
	// KeyAuth is the ACME key authorization. lego providers derive FQDN and
	// Value from it themselves.
	KeyAuth string `json:"key_auth"`
	// DryRun asks the plugin to validate shapes without touching the vendor API.
	DryRun bool `json:"dry_run,omitempty"`
}

// DNS01OptionsParams is the payload of dns01.options.
type DNS01OptionsParams struct {
	Provider string            `json:"provider"`
	Config   map[string]string `json:"config"`
	Options  map[string]any    `json:"options,omitempty"`
}

// DNS01OptionsResult is the reply to dns01.options.
type DNS01OptionsResult struct {
	PropagationTimeoutSeconds int `json:"propagation_timeout_seconds"`
	PollingIntervalSeconds    int `json:"polling_interval_seconds"`
	// SequentialIntervalSeconds is non-zero when the provider requires
	// challenges to be solved one at a time.
	SequentialIntervalSeconds int `json:"sequential_interval_seconds,omitempty"`
}

// DNS01CheckParams is the payload of dns01.check.
type DNS01CheckParams struct {
	Provider string            `json:"provider"`
	Config   map[string]string `json:"config"`
	Options  map[string]any    `json:"options,omitempty"`
	Domain   string            `json:"domain"`
	FQDN     string            `json:"fqdn"`
	Value    string            `json:"value"`
	KeyAuth  string            `json:"key_auth"`
}

// DNS01CheckResult is the reply to dns01.check.
type DNS01CheckResult struct {
	Ready         bool   `json:"ready"`
	EffectiveFQDN string `json:"effective_fqdn,omitempty"`
	Detail        string `json:"detail,omitempty"`
}

// DNS01ValidateParams is the payload of dns01.validate.
type DNS01ValidateParams struct {
	Provider string            `json:"provider"`
	Config   map[string]string `json:"config"`
}

// Keys of the per-certificate options map understood by the official dns01 plugin.
const (
	DNS01OptionCredentialID                      = "credential_id"
	DNS01OptionDisableCNAME                      = "disable_cname"
	DNS01OptionDisableAuthoritativeNSPropagation = "disable_authoritative_ns_propagation"
	DNS01OptionDisableRecursiveNSPropagation     = "disable_recursive_ns_propagation"
)
