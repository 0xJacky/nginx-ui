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

// HostNginxSnippetPutParams is the payload of host.nginx.snippet.put.
type HostNginxSnippetPutParams struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// HostNginxSnippetPutResult is the reply to host.nginx.snippet.put.
type HostNginxSnippetPutResult struct {
	Changed bool   `json:"changed"`
	Include string `json:"include"`
}

// HostNginxSnippetDeleteParams is the payload of host.nginx.snippet.delete.
type HostNginxSnippetDeleteParams struct {
	Name string `json:"name"`
}

// HostNginxSnippetDeleteResult is the reply to host.nginx.snippet.delete.
type HostNginxSnippetDeleteResult struct {
	Removed bool `json:"removed"`
}

// HostNginxSnippetListResult is the reply to host.nginx.snippet.list.
type HostNginxSnippetListResult struct {
	Snippets []HostNginxSnippet `json:"snippets"`
}

// HostNginxSnippet is one entry of HostNginxSnippetListResult.
type HostNginxSnippet struct {
	Name    string `json:"name"`
	Include string `json:"include"`
}

// HostNginxConfigListResult is the reply to host.nginx.config.list.
type HostNginxConfigListResult struct {
	Files []string `json:"files"`
}

// HostNginxConfigGetParams is the payload of host.nginx.config.get.
type HostNginxConfigGetParams struct {
	Path string `json:"path"`
}

// HostNginxConfigGetResult is the reply to host.nginx.config.get.
type HostNginxConfigGetResult struct {
	Content string `json:"content"`
}

// HostSitesListResult is the reply to host.sites.list.
type HostSitesListResult struct {
	Sites []HostSite `json:"sites"`
}

// HostSite is one entry of HostSitesListResult.
type HostSite struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	URLs       []string `json:"urls"`
	ConfigFile string   `json:"config_file"`
}

// HostCertsListResult is the reply to host.certs.list.
type HostCertsListResult struct {
	Certs []HostCert `json:"certs"`
}

// HostCert is one entry of HostCertsListResult. It never carries a private
// key.
type HostCert struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Domains         []string `json:"domains"`
	AutoRenew       bool     `json:"auto_renew"`
	ChallengeMethod string   `json:"challenge_method"`
	KeyType         string   `json:"key_type"`
	NotBefore       string   `json:"not_before"`
	NotAfter        string   `json:"not_after"`
	Issuer          string   `json:"issuer"`
}
