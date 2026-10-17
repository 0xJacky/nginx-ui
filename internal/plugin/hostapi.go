package plugin

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// MaxKVValueSize bounds one key value entry so a plugin cannot fill the
// database with a single call.
const MaxKVValueSize = 64 << 10

// HostBackend is everything a plugin can reach through the host API. The
// supervisor knows nothing about the database, the manager supplies the
// implementation.
type HostBackend interface {
	Log(pluginID string, p protocol.HostLogParams)
	KVGet(pluginID, key string) (json.RawMessage, bool, error)
	KVSet(pluginID, key string, value json.RawMessage) error
	KVDelete(pluginID, key string) error
	KVList(pluginID, prefix string) ([]string, error)
	SettingsGet(pluginID string) (map[string]any, error)
	Locale() string
	CredentialsGet(pluginID, kind, id string) (*protocol.HostCredentialsGetResult, error)
	CronRegister(pluginID string, p protocol.HostCronRegisterParams) error
	CronUnregister(pluginID, id string) error
	Notify(pluginID string, p protocol.HostNotifyParams) error
	MetricsSnapshot() (any, error)
	// LogsList returns the log files the plugin may read, never nil.
	LogsList(pluginID string) []protocol.HostLogFile
	// ActivitySet shows or clears one entry of the processing indicator. The
	// params are already validated.
	ActivitySet(pluginID string, p protocol.HostActivitySetParams) error
	NginxSnippetPut(pluginID, name, content string) (changed bool, err error)
	NginxSnippetDelete(pluginID, name string) (removed bool, err error)
	// NginxSnippetList returns the snippets of the plugin, never nil.
	NginxSnippetList(pluginID string) ([]protocol.HostNginxSnippet, error)
	NginxConfigList() ([]string, error)
	NginxConfigGet(path string) (string, error)
	// SitesList and CertsList return lists that are never nil.
	SitesList() ([]protocol.HostSite, error)
	CertsList() ([]protocol.HostCert, error)
}

// hostKVSetParams mirrors protocol.HostKVSetParams but keeps the value raw so
// what gets stored is exactly what the plugin sent.
type hostKVSetParams struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// RegisterHostHandlers wires every host.* method onto conn for one plugin.
// Permissions are the ones the user approved, methods outside that set fail
// with protocol.CodePermissionDenied.
func RegisterHostHandlers(conn *jsonrpc.Conn, pluginID string, permissions []string, backend HostBackend) {
	if conn == nil || backend == nil {
		return
	}
	granted := slices.Clone(permissions)

	conn.Handle(protocol.MethodHostLog, func(ctx context.Context, params json.RawMessage) (any, error) {
		var p protocol.HostLogParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		backend.Log(pluginID, p)
		return protocol.EmptyResult{}, nil
	})

	conn.Handle(protocol.MethodHostKVGet, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionKV); err != nil {
			return nil, err
		}
		var p protocol.HostKVGetParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := requireKey(p.Key); err != nil {
			return nil, err
		}
		value, found, err := backend.KVGet(pluginID, p.Key)
		if err != nil {
			return nil, err
		}
		result := protocol.HostKVGetResult{Found: found}
		if found {
			result.Value = value
		}
		return result, nil
	})

	conn.Handle(protocol.MethodHostKVSet, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionKV); err != nil {
			return nil, err
		}
		var p hostKVSetParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := requireKey(p.Key); err != nil {
			return nil, err
		}
		if len(p.Value) == 0 {
			return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "value is required")
		}
		if len(p.Value) > MaxKVValueSize {
			return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "value exceeds the 64 KiB limit")
		}
		if err := backend.KVSet(pluginID, p.Key, p.Value); err != nil {
			return nil, err
		}
		return protocol.EmptyResult{}, nil
	})

	conn.Handle(protocol.MethodHostKVDelete, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionKV); err != nil {
			return nil, err
		}
		var p protocol.HostKVGetParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := requireKey(p.Key); err != nil {
			return nil, err
		}
		if err := backend.KVDelete(pluginID, p.Key); err != nil {
			return nil, err
		}
		return protocol.EmptyResult{}, nil
	})

	conn.Handle(protocol.MethodHostKVList, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionKV); err != nil {
			return nil, err
		}
		var p protocol.HostKVListParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		keys, err := backend.KVList(pluginID, p.Prefix)
		if err != nil {
			return nil, err
		}
		if keys == nil {
			keys = []string{}
		}
		return protocol.HostKVListResult{Keys: keys}, nil
	})

	conn.Handle(protocol.MethodHostSettingsGet, func(ctx context.Context, params json.RawMessage) (any, error) {
		settings, err := backend.SettingsGet(pluginID)
		if err != nil {
			return nil, err
		}
		if settings == nil {
			settings = map[string]any{}
		}
		return protocol.HostSettingsGetResult{Settings: settings}, nil
	})

	conn.Handle(protocol.MethodHostI18nLocale, func(ctx context.Context, params json.RawMessage) (any, error) {
		return protocol.HostI18nLocaleResult{Locale: backend.Locale()}, nil
	})

	conn.Handle(protocol.MethodHostCredentialsGet, func(ctx context.Context, params json.RawMessage) (any, error) {
		var p protocol.HostCredentialsGetParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if p.Kind == "" {
			return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "kind is required")
		}
		if p.ID == "" {
			return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "id is required")
		}
		if err := requirePermission(granted, protocol.PermissionCredentialsReadPrefix+p.Kind); err != nil {
			return nil, err
		}
		credential, err := backend.CredentialsGet(pluginID, p.Kind, p.ID)
		if err != nil {
			return nil, err
		}
		return credential, nil
	})

	conn.Handle(protocol.MethodHostCronRegister, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionCron); err != nil {
			return nil, err
		}
		var p protocol.HostCronRegisterParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if p.ID == "" || p.Schedule == "" || p.Method == "" {
			return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "id, schedule and method are required")
		}
		if err := backend.CronRegister(pluginID, p); err != nil {
			return nil, err
		}
		return protocol.EmptyResult{}, nil
	})

	conn.Handle(protocol.MethodHostCronUnregister, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionCron); err != nil {
			return nil, err
		}
		var p protocol.HostCronUnregisterParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if p.ID == "" {
			return nil, jsonrpc.Errorf(protocol.CodeInvalidParams, "id is required")
		}
		if err := backend.CronUnregister(pluginID, p.ID); err != nil {
			return nil, err
		}
		return protocol.EmptyResult{}, nil
	})

	conn.Handle(protocol.MethodHostNotify, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionNotify); err != nil {
			return nil, err
		}
		var p protocol.HostNotifyParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := backend.Notify(pluginID, p); err != nil {
			return nil, err
		}
		return protocol.EmptyResult{}, nil
	})

	conn.Handle(protocol.MethodHostMetricsSnapshot, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionMetricsRead); err != nil {
			return nil, err
		}
		snapshot, err := backend.MetricsSnapshot()
		if err != nil {
			return nil, err
		}
		return protocol.HostMetricsSnapshotResult{Snapshot: snapshot}, nil
	})

	conn.Handle(protocol.MethodHostLogsList, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionLogFiles); err != nil {
			return nil, err
		}
		logs := backend.LogsList(pluginID)
		if logs == nil {
			logs = []protocol.HostLogFile{}
		}
		return protocol.HostLogsListResult{Logs: logs}, nil
	})

	conn.Handle(protocol.MethodHostActivitySet, func(ctx context.Context, params json.RawMessage) (any, error) {
		var p protocol.HostActivitySetParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		if err := validateActivity(p); err != nil {
			return nil, err
		}
		if err := backend.ActivitySet(pluginID, p); err != nil {
			return nil, err
		}
		return protocol.EmptyResult{}, nil
	})

	registerNginxHandlers(conn, pluginID, granted, backend)
}

// registerNginxHandlers wires the methods that reach nginx, its sites and
// its certificates.
func registerNginxHandlers(conn *jsonrpc.Conn, pluginID string, granted []string, backend HostBackend) {
	conn.Handle(protocol.MethodHostNginxSnippetPut, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionNginxSnippet); err != nil {
			return nil, err
		}
		var p protocol.HostNginxSnippetPutParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		changed, err := backend.NginxSnippetPut(pluginID, p.Name, p.Content)
		if err != nil {
			return nil, err
		}
		return protocol.HostNginxSnippetPutResult{Changed: changed, Include: SnippetInclude(pluginID, p.Name)}, nil
	})

	conn.Handle(protocol.MethodHostNginxSnippetDelete, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionNginxSnippet); err != nil {
			return nil, err
		}
		var p protocol.HostNginxSnippetDeleteParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		removed, err := backend.NginxSnippetDelete(pluginID, p.Name)
		if err != nil {
			return nil, err
		}
		return protocol.HostNginxSnippetDeleteResult{Removed: removed}, nil
	})

	conn.Handle(protocol.MethodHostNginxSnippetList, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionNginxSnippet); err != nil {
			return nil, err
		}
		snippets, err := backend.NginxSnippetList(pluginID)
		if err != nil {
			return nil, err
		}
		if snippets == nil {
			snippets = []protocol.HostNginxSnippet{}
		}
		return protocol.HostNginxSnippetListResult{Snippets: snippets}, nil
	})

	conn.Handle(protocol.MethodHostNginxConfigList, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionNginxConfigRead); err != nil {
			return nil, err
		}
		files, err := backend.NginxConfigList()
		if err != nil {
			return nil, err
		}
		if files == nil {
			files = []string{}
		}
		return protocol.HostNginxConfigListResult{Files: files}, nil
	})

	conn.Handle(protocol.MethodHostNginxConfigGet, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionNginxConfigRead); err != nil {
			return nil, err
		}
		var p protocol.HostNginxConfigGetParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		content, err := backend.NginxConfigGet(p.Path)
		if err != nil {
			return nil, err
		}
		return protocol.HostNginxConfigGetResult{Content: content}, nil
	})

	conn.Handle(protocol.MethodHostSitesList, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionSitesRead); err != nil {
			return nil, err
		}
		sites, err := backend.SitesList()
		if err != nil {
			return nil, err
		}
		if sites == nil {
			sites = []protocol.HostSite{}
		}
		return protocol.HostSitesListResult{Sites: sites}, nil
	})

	conn.Handle(protocol.MethodHostCertsList, func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := requirePermission(granted, protocol.PermissionCertsRead); err != nil {
			return nil, err
		}
		certs, err := backend.CertsList()
		if err != nil {
			return nil, err
		}
		if certs == nil {
			certs = []protocol.HostCert{}
		}
		return protocol.HostCertsListResult{Certs: certs}, nil
	})
}

const (
	// maxActivityKeyLen and maxActivityLabelLen bound host.activity.set.
	maxActivityKeyLen   = 64
	maxActivityLabelLen = 128
)

// validateActivity checks the params of host.activity.set. The
// label is ignored when the entry is removed.
func validateActivity(p protocol.HostActivitySetParams) error {
	if p.Key == "" || len(p.Key) > maxActivityKeyLen {
		return jsonrpc.Errorf(protocol.CodeInvalidParams, "key must be 1 to 64 characters")
	}
	for _, r := range p.Key {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return jsonrpc.Errorf(protocol.CodeInvalidParams, "key may only contain a-z, 0-9, dot, underscore and dash")
		}
	}
	if !p.Active {
		return nil
	}
	if n := utf8.RuneCountInString(p.Label); n < 1 || n > maxActivityLabelLen {
		return jsonrpc.Errorf(protocol.CodeInvalidParams, "label must be 1 to 128 characters")
	}
	return nil
}

// decodeParams unmarshals the params member, treating an absent one as empty.
func decodeParams(params json.RawMessage, target any) error {
	trimmed := strings.TrimSpace(string(params))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if err := json.Unmarshal(params, target); err != nil {
		return jsonrpc.Errorf(protocol.CodeInvalidParams, err.Error())
	}
	return nil
}

// requirePermission turns a missing permission into the wire error the plugin
// SDK knows how to report.
func requirePermission(granted []string, permission string) error {
	if slices.Contains(granted, permission) {
		return nil
	}
	return jsonrpc.Errorf(protocol.CodePermissionDenied, "permission "+permission+" is not granted")
}

func requireKey(key string) error {
	if key == "" {
		return jsonrpc.Errorf(protocol.CodeInvalidParams, "key is required")
	}
	return nil
}
