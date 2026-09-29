package protocol

// HostLogParams is the payload of host.log.
type HostLogParams struct {
	Level   string         `json:"level"` // debug | info | warn | error
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// HostKVGetParams is the payload of host.kv.get and host.kv.delete.
type HostKVGetParams struct {
	Key string `json:"key"`
}

// HostKVGetResult is the reply to host.kv.get.
type HostKVGetResult struct {
	Value any  `json:"value"`
	Found bool `json:"found"`
}

// HostKVSetParams is the payload of host.kv.set. Value is any JSON, up to 64 KiB encoded.
type HostKVSetParams struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// HostKVListParams is the payload of host.kv.list.
type HostKVListParams struct {
	Prefix string `json:"prefix,omitempty"`
}

// HostKVListResult is the reply to host.kv.list.
type HostKVListResult struct {
	Keys []string `json:"keys"`
}

// HostSettingsGetResult is the reply to host.settings.get.
type HostSettingsGetResult struct {
	Settings map[string]any `json:"settings"`
}

// HostI18nLocaleResult is the reply to host.i18n.locale.
type HostI18nLocaleResult struct {
	Locale string `json:"locale"`
}

// HostCredentialsGetParams is the payload of host.credentials.get.
type HostCredentialsGetParams struct {
	Kind string `json:"kind"` // "dns"
	ID   string `json:"id"`
}

// HostCredentialsGetResult is the reply to host.credentials.get.
type HostCredentialsGetResult struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ProviderCode string            `json:"provider_code"`
	Config       map[string]string `json:"config"`
}

// HostCronRegisterParams is the payload of host.cron.register.
type HostCronRegisterParams struct {
	ID       string `json:"id"`
	Schedule string `json:"schedule"` // five-field cron or "@every 10m"
	Method   string `json:"method"`
}

// HostCronUnregisterParams is the payload of host.cron.unregister.
type HostCronUnregisterParams struct {
	ID string `json:"id"`
}

// HostNotifyParams is the payload of host.notify.
type HostNotifyParams struct {
	Level   string `json:"level"` // info | success | warning | error
	Title   string `json:"title"`
	Content string `json:"content"`
	Details any    `json:"details,omitempty"`
}

// HostMetricsSnapshotResult is the reply to host.metrics.snapshot. The shape
// mirrors internal/analytic's snapshot and is opaque to the protocol.
type HostMetricsSnapshotResult struct {
	Snapshot any `json:"snapshot"`
}

// HostLogsListResult is the reply to host.logs.list.
type HostLogsListResult struct {
	Logs []HostLogFile `json:"logs"`
}

// HostLogFile is one entry of HostLogsListResult.
type HostLogFile struct {
	Path       string `json:"path"`
	Type       string `json:"type"`   // access | error
	Source     string `json:"source"` // config | default
	ConfigFile string `json:"config_file,omitempty"`
}

// HostActivitySetParams is the payload of host.activity.set.
type HostActivitySetParams struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Active bool   `json:"active"`
}
