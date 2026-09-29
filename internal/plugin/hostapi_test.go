package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHostBackend is an in-memory HostBackend for the tests.
type fakeHostBackend struct {
	mu       sync.Mutex
	kv       map[string]json.RawMessage
	settings map[string]any
	locale   string
	cron     []protocol.HostCronRegisterParams
	notified []protocol.HostNotifyParams
	logged   []protocol.HostLogParams
	onLog    func(protocol.HostLogParams)
	failKV   error

	logFiles    []protocol.HostLogFile
	activity    []protocol.HostActivitySetParams
	activityErr error
}

func (f *fakeHostBackend) Log(pluginID string, p protocol.HostLogParams) {
	f.mu.Lock()
	f.logged = append(f.logged, p)
	f.mu.Unlock()
	if f.onLog != nil {
		f.onLog(p)
	}
}

func (f *fakeHostBackend) KVGet(pluginID, key string) (json.RawMessage, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failKV != nil {
		return nil, false, f.failKV
	}
	value, ok := f.kv[key]
	return value, ok, nil
}

func (f *fakeHostBackend) KVSet(pluginID, key string, value json.RawMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failKV != nil {
		return f.failKV
	}
	if f.kv == nil {
		f.kv = map[string]json.RawMessage{}
	}
	f.kv[key] = value
	return nil
}

func (f *fakeHostBackend) KVDelete(pluginID, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.kv, key)
	return nil
}

func (f *fakeHostBackend) KVList(pluginID, prefix string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.kv))
	for key := range f.kv {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (f *fakeHostBackend) SettingsGet(pluginID string) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settings, nil
}

func (f *fakeHostBackend) Locale() string { return f.locale }

func (f *fakeHostBackend) CredentialsGet(pluginID, kind, id string) (*protocol.HostCredentialsGetResult, error) {
	return &protocol.HostCredentialsGetResult{
		ID:           id,
		Name:         "credential " + id,
		ProviderCode: kind,
		Config:       map[string]string{"token": "secret"},
	}, nil
}

func (f *fakeHostBackend) CronRegister(pluginID string, p protocol.HostCronRegisterParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cron = append(f.cron, p)
	return nil
}

func (f *fakeHostBackend) CronUnregister(pluginID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cron = nil
	return nil
}

func (f *fakeHostBackend) Notify(pluginID string, p protocol.HostNotifyParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notified = append(f.notified, p)
	return nil
}

func (f *fakeHostBackend) MetricsSnapshot() (any, error) {
	return map[string]any{"cpu": 12.5}, nil
}

func (f *fakeHostBackend) LogsList(pluginID string) []protocol.HostLogFile {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logFiles
}

func (f *fakeHostBackend) ActivitySet(pluginID string, p protocol.HostActivitySetParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activity = append(f.activity, p)
	return f.activityErr
}

// hostPipe puts the host API on one end of an in-memory pipe pair and returns
// the caller a plugin would use.
func hostPipe(t *testing.T, permissions []string, backend HostBackend) *jsonrpc.Conn {
	t.Helper()

	hostReader, pluginWriter := io.Pipe()
	pluginReader, hostWriter := io.Pipe()

	host := jsonrpc.NewConn(hostReader, hostWriter)
	plugin := jsonrpc.NewConn(pluginReader, pluginWriter)
	RegisterHostHandlers(host, "official.test", permissions, backend)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = host.Serve(ctx) }()
	go func() { defer wg.Done(); _ = plugin.Serve(ctx) }()

	t.Cleanup(func() {
		cancel()
		_ = host.Close()
		_ = plugin.Close()
		wg.Wait()
	})
	return plugin
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestHostAPIWithoutPermissions(t *testing.T) {
	backend := &fakeHostBackend{locale: "zh_CN", settings: map[string]any{"token": "value"}}
	client := hostPipe(t, nil, backend)
	ctx := testContext(t)

	// host.log, host.settings.get and host.i18n.locale need no permission.
	require.NoError(t, client.Call(ctx, protocol.MethodHostLog,
		protocol.HostLogParams{Level: "warn", Message: "careful"}, nil))
	backend.mu.Lock()
	require.Len(t, backend.logged, 1)
	assert.Equal(t, "careful", backend.logged[0].Message)
	backend.mu.Unlock()

	var settings protocol.HostSettingsGetResult
	require.NoError(t, client.Call(ctx, protocol.MethodHostSettingsGet, nil, &settings))
	assert.Equal(t, map[string]any{"token": "value"}, settings.Settings)

	var locale protocol.HostI18nLocaleResult
	require.NoError(t, client.Call(ctx, protocol.MethodHostI18nLocale, nil, &locale))
	assert.Equal(t, "zh_CN", locale.Locale)

	// Everything else is denied.
	denied := []struct {
		method string
		params any
	}{
		{protocol.MethodHostKVGet, protocol.HostKVGetParams{Key: "a"}},
		{protocol.MethodHostKVSet, protocol.HostKVSetParams{Key: "a", Value: 1}},
		{protocol.MethodHostKVDelete, protocol.HostKVGetParams{Key: "a"}},
		{protocol.MethodHostKVList, protocol.HostKVListParams{}},
		{protocol.MethodHostCredentialsGet, protocol.HostCredentialsGetParams{Kind: "dns", ID: "1"}},
		{protocol.MethodHostCronRegister, protocol.HostCronRegisterParams{ID: "a", Schedule: "@every 1m", Method: "job"}},
		{protocol.MethodHostCronUnregister, protocol.HostCronUnregisterParams{ID: "a"}},
		{protocol.MethodHostNotify, protocol.HostNotifyParams{Level: "info", Title: "t"}},
		{protocol.MethodHostMetricsSnapshot, nil},
		{protocol.MethodHostLogsList, nil},
	}
	for _, call := range denied {
		err := client.Call(ctx, call.method, call.params, nil)
		perr, ok := jsonrpc.AsProtocolError(err)
		require.True(t, ok, "%s should fail with a protocol error", call.method)
		assert.Equal(t, protocol.CodePermissionDenied, perr.Code, "%s", call.method)
	}
}

func TestHostAPIKV(t *testing.T) {
	backend := &fakeHostBackend{}
	client := hostPipe(t, []string{protocol.PermissionKV}, backend)
	ctx := testContext(t)

	require.NoError(t, client.Call(ctx, protocol.MethodHostKVSet,
		protocol.HostKVSetParams{Key: "state", Value: map[string]any{"count": 2}}, nil))

	var got protocol.HostKVGetResult
	require.NoError(t, client.Call(ctx, protocol.MethodHostKVGet, protocol.HostKVGetParams{Key: "state"}, &got))
	assert.True(t, got.Found)
	assert.Equal(t, map[string]any{"count": float64(2)}, got.Value)

	var missing protocol.HostKVGetResult
	require.NoError(t, client.Call(ctx, protocol.MethodHostKVGet, protocol.HostKVGetParams{Key: "nope"}, &missing))
	assert.False(t, missing.Found)
	assert.Nil(t, missing.Value)

	var list protocol.HostKVListResult
	require.NoError(t, client.Call(ctx, protocol.MethodHostKVList, protocol.HostKVListParams{Prefix: "st"}, &list))
	assert.Equal(t, []string{"state"}, list.Keys)

	require.NoError(t, client.Call(ctx, protocol.MethodHostKVDelete, protocol.HostKVGetParams{Key: "state"}, nil))
	require.NoError(t, client.Call(ctx, protocol.MethodHostKVList, protocol.HostKVListParams{}, &list))
	assert.Empty(t, list.Keys)

	// An empty key is rejected before the backend is reached.
	err := client.Call(ctx, protocol.MethodHostKVGet, protocol.HostKVGetParams{}, nil)
	perr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)

	// Values above 64 KiB are refused.
	err = client.Call(ctx, protocol.MethodHostKVSet,
		protocol.HostKVSetParams{Key: "big", Value: strings.Repeat("x", MaxKVValueSize+1)}, nil)
	perr, ok = jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)
	assert.Contains(t, perr.Message, "64 KiB")

	// Backend failures become internal errors.
	backend.mu.Lock()
	backend.failKV = errors.New("database is down")
	backend.mu.Unlock()
	err = client.Call(ctx, protocol.MethodHostKVGet, protocol.HostKVGetParams{Key: "state"}, nil)
	perr, ok = jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInternalError, perr.Code)
	assert.Equal(t, "database is down", perr.Message)
}

func TestHostAPICredentialsPermissionIsScopedToKind(t *testing.T) {
	backend := &fakeHostBackend{}
	client := hostPipe(t, []string{protocol.PermissionCredentialsReadPrefix + "dns"}, backend)
	ctx := testContext(t)

	var credential protocol.HostCredentialsGetResult
	require.NoError(t, client.Call(ctx, protocol.MethodHostCredentialsGet,
		protocol.HostCredentialsGetParams{Kind: "dns", ID: "7"}, &credential))
	assert.Equal(t, "7", credential.ID)
	assert.Equal(t, "secret", credential.Config["token"])

	err := client.Call(ctx, protocol.MethodHostCredentialsGet,
		protocol.HostCredentialsGetParams{Kind: "ssh", ID: "7"}, nil)
	perr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodePermissionDenied, perr.Code)

	err = client.Call(ctx, protocol.MethodHostCredentialsGet, protocol.HostCredentialsGetParams{ID: "7"}, nil)
	perr, ok = jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)
}

func TestHostAPICronNotifyAndMetrics(t *testing.T) {
	backend := &fakeHostBackend{}
	client := hostPipe(t, []string{
		protocol.PermissionCron,
		protocol.PermissionNotify,
		protocol.PermissionMetricsRead,
	}, backend)
	ctx := testContext(t)

	require.NoError(t, client.Call(ctx, protocol.MethodHostCronRegister,
		protocol.HostCronRegisterParams{ID: "sync", Schedule: "@every 10m", Method: "cron.sync"}, nil))
	backend.mu.Lock()
	require.Len(t, backend.cron, 1)
	assert.Equal(t, "cron.sync", backend.cron[0].Method)
	backend.mu.Unlock()

	err := client.Call(ctx, protocol.MethodHostCronRegister, protocol.HostCronRegisterParams{ID: "sync"}, nil)
	perr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)

	require.NoError(t, client.Call(ctx, protocol.MethodHostCronUnregister,
		protocol.HostCronUnregisterParams{ID: "sync"}, nil))

	require.NoError(t, client.Call(ctx, protocol.MethodHostNotify,
		protocol.HostNotifyParams{Level: "success", Title: "done", Content: "all good"}, nil))
	backend.mu.Lock()
	require.Len(t, backend.notified, 1)
	assert.Equal(t, "done", backend.notified[0].Title)
	backend.mu.Unlock()

	var snapshot protocol.HostMetricsSnapshotResult
	require.NoError(t, client.Call(ctx, protocol.MethodHostMetricsSnapshot, nil, &snapshot))
	assert.Equal(t, map[string]any{"cpu": 12.5}, snapshot.Snapshot)
}

func TestHostAPIRejectsMalformedParams(t *testing.T) {
	client := hostPipe(t, []string{protocol.PermissionKV}, &fakeHostBackend{})
	ctx := testContext(t)

	err := client.Call(ctx, protocol.MethodHostKVGet, []string{"not an object"}, nil)
	perr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)
}

func TestHostAPILogsList(t *testing.T) {
	backend := &fakeHostBackend{logFiles: []protocol.HostLogFile{
		{Path: "/var/log/nginx/access.log", Type: "access", Source: "default"},
		{Path: "/var/log/nginx/a.error.log", Type: "error", Source: "config", ConfigFile: "/etc/nginx/sites-enabled/a.conf"},
	}}

	// Without log.files the call is refused and nothing is listed.
	denied := hostPipe(t, []string{protocol.PermissionKV}, backend)
	err := denied.Call(testContext(t), protocol.MethodHostLogsList, nil, nil)
	perr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodePermissionDenied, perr.Code)

	client := hostPipe(t, []string{protocol.PermissionLogFiles}, backend)
	var result protocol.HostLogsListResult
	require.NoError(t, client.Call(testContext(t), protocol.MethodHostLogsList, nil, &result))
	assert.Equal(t, backend.logFiles, result.Logs)

	// An empty list is an array, never null.
	empty := hostPipe(t, []string{protocol.PermissionLogFiles}, &fakeHostBackend{})
	var raw json.RawMessage
	require.NoError(t, empty.Call(testContext(t), protocol.MethodHostLogsList, nil, &raw))
	assert.JSONEq(t, `{"logs":[]}`, string(raw))
}

func TestHostAPIActivitySet(t *testing.T) {
	backend := &fakeHostBackend{}
	// No permission is needed.
	client := hostPipe(t, nil, backend)
	ctx := testContext(t)

	var raw json.RawMessage
	require.NoError(t, client.Call(ctx, protocol.MethodHostActivitySet,
		protocol.HostActivitySetParams{Key: "indexing", Label: "Nginx Log Indexing...", Active: true}, &raw))
	assert.JSONEq(t, `{}`, string(raw))
	// A removal ignores the label.
	require.NoError(t, client.Call(ctx, protocol.MethodHostActivitySet,
		protocol.HostActivitySetParams{Key: "indexing", Active: false}, nil))
	backend.mu.Lock()
	assert.Len(t, backend.activity, 2)
	backend.mu.Unlock()

	invalid := []protocol.HostActivitySetParams{
		{Key: "", Label: "x", Active: true},
		{Key: strings.Repeat("a", 65), Label: "x", Active: true},
		{Key: "Upper", Label: "x", Active: true},
		{Key: "has space", Label: "x", Active: true},
		{Key: "slash/key", Label: "x", Active: true},
		{Key: "ok", Label: "", Active: true},
		{Key: "ok", Label: strings.Repeat("l", 129), Active: true},
		{Key: "", Active: false},
	}
	for _, p := range invalid {
		err := client.Call(ctx, protocol.MethodHostActivitySet, p, nil)
		perr, ok := jsonrpc.AsProtocolError(err)
		require.True(t, ok, "%+v", p)
		assert.Equal(t, protocol.CodeInvalidParams, perr.Code, "%+v", p)
	}
	backend.mu.Lock()
	assert.Len(t, backend.activity, 2, "invalid calls never reach the backend")
	backend.mu.Unlock()

	// 128 characters count as characters, not bytes.
	require.NoError(t, client.Call(ctx, protocol.MethodHostActivitySet,
		protocol.HostActivitySetParams{Key: "a.b_c-1", Label: strings.Repeat("é", 128), Active: true}, nil))

	// A backend refusal such as the entry limit is passed on.
	backend.mu.Lock()
	backend.activityErr = jsonrpc.Errorf(protocol.CodeInvalidParams, "too many")
	backend.mu.Unlock()
	err := client.Call(ctx, protocol.MethodHostActivitySet,
		protocol.HostActivitySetParams{Key: "more", Label: "x", Active: true}, nil)
	perr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)
}
