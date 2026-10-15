package ingress

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// brokenMarker in any generated file makes the fake `nginx -t` fail.
const brokenMarker = "BROKEN"

// testEnv is an nginx configuration tree with fake test and reload commands
// that count their calls, and an in-memory database.
type testEnv struct {
	confDir string
	logDir  string
	certs   *fakeCerts
}

func setupEnv(t *testing.T) *testEnv {
	t.Helper()

	env := &testEnv{confDir: t.TempDir(), logDir: t.TempDir(), certs: newFakeCerts()}
	for _, dir := range []string{"conf.d", "sites-available", "sites-enabled"} {
		require.NoError(t, os.MkdirAll(filepath.Join(env.confDir, dir), 0o755))
	}

	original := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = env.confDir
	settings.NginxSettings.PIDPath = filepath.Join(env.confDir, "nginx.pid")
	settings.NginxSettings.TestConfigCmd = "echo t >> " + env.logPath("tests") +
		"; if grep -rqs " + brokenMarker + " " + env.confDir + "; then echo 'nginx: [emerg] broken config' >&2; exit 1; fi"
	settings.NginxSettings.ReloadCmd = "echo r >> " + env.logPath("reloads")
	settings.NginxSettings.RestartCmd = "true"
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))

	originalChallengePort := settings.CertSettings.HTTPChallengePort
	settings.CertSettings.HTTPChallengePort = "9180"
	originalAllowed := settings.IngressSettings.AllowedBackends
	settings.IngressSettings.AllowedBackends = []string{"127.0.0.1/32", "::1/128"}

	originalHTTP2 := supportsHTTP2Directive
	supportsHTTP2Directive = func() bool { return true }
	originalSync, originalRemoveSync := syncToNodes, syncRemoveToNodes
	syncToNodes = func(context.Context, *preparedItem) {}
	syncRemoveToNodes = func(context.Context, *preparedItem) {}
	originalQuiet, originalMax := coalesceQuiet, coalesceMax
	coalesceQuiet, coalesceMax = 30*time.Millisecond, 300*time.Millisecond
	SetCertManager(env.certs)

	t.Cleanup(func() {
		*settings.NginxSettings = original
		settings.CertSettings.HTTPChallengePort = originalChallengePort
		settings.IngressSettings.AllowedBackends = originalAllowed
		supportsHTTP2Directive = originalHTTP2
		syncToNodes, syncRemoveToNodes = originalSync, originalRemoveSync
		coalesceQuiet, coalesceMax = originalQuiet, originalMax
		SetCertManager(noCerts{})
		queue.take()
	})

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	// The worker and the test write concurrently; a shared cache database
	// reports "table is locked" instead of waiting, so use one connection.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Ingress{}, &model.Site{}, &model.Namespace{}, &model.Node{},
		&model.Config{}, &model.ConfigBackup{}, &model.Cert{}, &model.Stream{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	upstream.GetUpstreamService().ClearTargets()
	t.Cleanup(upstream.GetUpstreamService().ClearTargets)
	queue.take()
	return env
}

func (env *testEnv) logPath(name string) string {
	return filepath.Join(env.logDir, name+".log")
}

func (env *testEnv) count(name string) int {
	content, err := os.ReadFile(env.logPath(name))
	if err != nil {
		return 0
	}
	return strings.Count(string(content), "\n")
}

func (env *testEnv) tests() int   { return env.count("tests") }
func (env *testEnv) reloads() int { return env.count("reloads") }

func (env *testEnv) path(parts ...string) string {
	return filepath.Join(append([]string{env.confDir}, parts...)...)
}

func (env *testEnv) read(t *testing.T, parts ...string) string {
	t.Helper()
	content, err := os.ReadFile(env.path(parts...))
	require.NoError(t, err)
	return string(content)
}

func (env *testEnv) exists(parts ...string) bool {
	_, err := os.Lstat(env.path(parts...))
	return err == nil
}

// startWorker runs the worker until the test ends.
func (env *testEnv) startWorker(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startWorker(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
}

// runWorker runs the worker until the returned function is called.
func (env *testEnv) runWorker() (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startWorker(ctx)
	}()
	return func() {
		cancel()
		wg.Wait()
	}
}

// fakeCerts is a certificate manager with fixed states.
type fakeCerts struct {
	mu        sync.Mutex
	states    map[string]CertState
	ensured   []string
	forgotten []string
}

func newFakeCerts() *fakeCerts {
	return &fakeCerts{states: make(map[string]CertState)}
}

func (f *fakeCerts) set(name string, state CertState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[name] = state
}

func (f *fakeCerts) Resolve(_ context.Context, ing *model.Ingress) (CertState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if state, ok := f.states[ing.Name]; ok {
		return state, nil
	}
	return CertState{Reason: "Issuing"}, nil
}

func (f *fakeCerts) Ensure(_ context.Context, ing *model.Ingress) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensured = append(f.ensured, ing.Name)
}

func (f *fakeCerts) Forget(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, name)
}

func (f *fakeCerts) snapshot() (ensured, forgotten []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ensured...), append([]string(nil), f.forgotten...)
}

// readyCert is a certificate state the renderer accepts.
func readyCert() CertState {
	return CertState{Ready: true, CertID: 7, CertPath: "/etc/nginx/ssl/example/fullchain.cer", KeyPath: "/etc/nginx/ssl/example/private.key"}
}

// basicSpec is a valid single host, single route spec.
func basicSpec(host string) model.IngressSpec {
	return model.IngressSpec{
		Hosts: []string{host},
		Backends: []model.IngressBackend{{
			Name:    "app",
			Servers: []model.IngressServer{{Address: "127.0.0.1:24001"}},
		}},
		Routes: []model.IngressRoute{{PathPrefix: "/", Backend: "app"}},
		TLS:    model.IngressTLS{Mode: model.IngressTLSModeNone},
	}
}

func newIngress(name, host string) *model.Ingress {
	return &model.Ingress{Name: name, Owner: "tester", Spec: basicSpec(host), Generation: 1}
}

// requireCode asserts err is a cosy error with the code of want.
func requireCode(t *testing.T, err error, want error) {
	t.Helper()
	require.Error(t, err)
	var got, expected *cosy.Error
	require.True(t, errors.As(err, &got), "not a cosy error: %v", err)
	require.True(t, errors.As(want, &expected))
	require.Equal(t, expected.Code, got.Code, "error: %v", err)
}

// eventually polls cond until it holds or the timeout expires.
func eventually(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

func setReloadCmd(t *testing.T, cmd string) {
	t.Helper()
	original := settings.NginxSettings.ReloadCmd
	settings.NginxSettings.ReloadCmd = cmd
	t.Cleanup(func() { settings.NginxSettings.ReloadCmd = original })
}
