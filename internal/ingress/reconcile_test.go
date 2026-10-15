package ingress

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// reconcileTestEnv replaces the reconcile seams with fakes and points nginx
// at a temp conf dir.
type reconcileTestEnv struct {
	mu        sync.Mutex
	enqueued  []string
	ensured   []string
	certs     map[string]CertState
	certErrs  map[string]error
	renderErr map[string]error
	avail     map[string]*upstream.Status
	checkerOn bool
	now       time.Time
}

func setupReconcileTest(t *testing.T) *reconcileTestEnv {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"sites-available", "sites-enabled", "conf.d"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = originalConfigDir })

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Ingress{}, &model.Site{}, &model.Namespace{}))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })

	env := &reconcileTestEnv{
		certs:     map[string]CertState{},
		certErrs:  map[string]error{},
		renderErr: map[string]error{},
		avail:     map[string]*upstream.Status{},
		checkerOn: true,
		now:       time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC),
	}

	originalRender, originalEnqueue := reconcileRender, reconcileEnqueue
	originalResolve, originalAvail, originalNow := reconcileResolveCert, reconcileAvailability, reconcileNow
	originalEnsure := reconcileEnsureCert
	t.Cleanup(func() {
		reconcileRender, reconcileEnqueue = originalRender, originalEnqueue
		reconcileResolveCert, reconcileAvailability, reconcileNow = originalResolve, originalAvail, originalNow
		reconcileEnsureCert = originalEnsure
	})
	reconcileEnsureCert = func(_ context.Context, ing *model.Ingress) {
		env.mu.Lock()
		defer env.mu.Unlock()
		env.ensured = append(env.ensured, ing.Name)
	}

	reconcileRender = func(ing *model.Ingress, opts RenderOptions) (*Rendered, error) {
		if err := env.renderErr[ing.Name]; err != nil {
			return nil, err
		}
		return reconcileFakeRender(ing, opts), nil
	}
	reconcileEnqueue = func(names ...string) {
		env.mu.Lock()
		defer env.mu.Unlock()
		env.enqueued = append(env.enqueued, names...)
	}
	reconcileResolveCert = func(_ context.Context, ing *model.Ingress) (CertState, error) {
		if err := env.certErrs[ing.Name]; err != nil {
			return CertState{}, err
		}
		return env.certs[ing.Name], nil
	}
	reconcileAvailability = func() (map[string]*upstream.Status, bool) {
		return env.avail, env.checkerOn
	}
	reconcileNow = func() time.Time { return env.now }

	return env
}

func (env *reconcileTestEnv) ensuredNames() []string {
	env.mu.Lock()
	defer env.mu.Unlock()
	return append([]string(nil), env.ensured...)
}

func (env *reconcileTestEnv) enqueuedNames() []string {
	env.mu.Lock()
	defer env.mu.Unlock()
	return append([]string(nil), env.enqueued...)
}

// reconcileFakeRender is deterministic like Render: the output depends on the spec and
// the certificate only.
func reconcileFakeRender(ing *model.Ingress, opts RenderOptions) *Rendered {
	r := &Rendered{SiteName: SiteName(ing.Name), Upstreams: map[string]string{}}
	var sb strings.Builder
	fmt.Fprintf(&sb, "server { listen 80; server_name %s; }\n", strings.Join(ing.Spec.Hosts, " "))
	if opts.Cert.Ready {
		fmt.Fprintf(&sb, "server { listen 443 ssl; ssl_certificate %s; }\n", opts.Cert.CertPath)
	}
	r.SiteContent = sb.String()
	for _, b := range ing.Spec.Backends {
		var ub strings.Builder
		fmt.Fprintf(&ub, "upstream %s {\n", UpstreamName(ing.Name, b.Name))
		for _, s := range b.Servers {
			fmt.Fprintf(&ub, "    server %s%s;\n", s.Address, map[bool]string{true: " down"}[s.Down])
		}
		ub.WriteString("}\n")
		r.Upstreams[UpstreamName(ing.Name, b.Name)] = ub.String()
	}
	return r
}

var reconcileTestNotAfter = time.Date(2027, 1, 13, 8, 0, 0, 0, time.UTC)

// newIngress stores an applied ACME ingress with a ready
// certificate and writes its files as the worker would have.
func (env *reconcileTestEnv) newIngress(t *testing.T, name string) *model.Ingress {
	t.Helper()

	notAfter := reconcileTestNotAfter
	applied := env.now.Add(-time.Hour)
	ing := &model.Ingress{
		Name:       name,
		Owner:      "telemetry-prod",
		Generation: 1,
		SitePath:   SiteName(name),
		Spec: model.IngressSpec{
			Hosts: []string{name + ".example.com"},
			Backends: []model.IngressBackend{{
				Name: "app",
				Servers: []model.IngressServer{
					{Address: "127.0.0.1:24001"},
					{Address: "127.0.0.1:24002"},
				},
			}},
			Routes: []model.IngressRoute{{PathPrefix: "/", Backend: "app"}},
			TLS:    model.IngressTLS{Mode: model.IngressTLSModeACME},
		},
		Status: model.IngressStatus{
			ObservedGeneration: 1,
			CertID:             31,
			CertNotAfter:       &notAfter,
			LastAppliedAt:      &applied,
			Conditions: []model.IngressCondition{
				{Type: model.IngressConditionConfigApplied, Status: true, Reason: "Applied", LastTransitionTime: applied},
				{Type: model.IngressConditionCertReady, Status: true, Reason: reconcileReasonIssued, LastTransitionTime: applied},
				{Type: model.IngressConditionBackendsHealthy, Status: true, Reason: reconcileReasonAllHealthy, LastTransitionTime: applied},
			},
		},
	}
	ing.Status.Servers = []model.IngressServerStatus{
		{Backend: "app", Address: "127.0.0.1:24001", Healthy: true},
		{Backend: "app", Address: "127.0.0.1:24002", Healthy: true},
	}
	ing.Status.Phase = ComputePhase(ing)
	require.NoError(t, model.UseDB().Create(ing).Error)

	env.certs[name] = CertState{
		Ready:    true,
		CertID:   31,
		CertPath: "/etc/nginx/ssl/" + name + "/fullchain.cer",
		KeyPath:  "/etc/nginx/ssl/" + name + "/private.key",
		NotAfter: &notAfter,
	}
	env.avail["127.0.0.1:24001"] = &upstream.Status{Online: true}
	env.avail["127.0.0.1:24002"] = &upstream.Status{Online: true}

	env.writeFiles(t, ing)
	return ing
}

// writeFiles writes what the worker would have applied for the current cert.
func (env *reconcileTestEnv) writeFiles(t *testing.T, ing *model.Ingress) {
	t.Helper()

	rendered := reconcileFakeRender(ing, RenderOptions{Cert: env.certs[ing.Name]})
	sitePath, err := site.ResolveAvailablePath(rendered.SiteName)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(sitePath, []byte(rendered.SiteContent), 0o644))
	enabledPath, err := site.ResolveEnabledPath(rendered.SiteName)
	require.NoError(t, err)
	_ = os.Remove(enabledPath)
	require.NoError(t, os.Symlink(sitePath, enabledPath))
	for name, content := range rendered.Upstreams {
		path, err := managed.Path(name)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func loadReconcileTestIngress(t *testing.T, name string) *model.Ingress {
	t.Helper()
	var ing model.Ingress
	require.NoError(t, model.UseDB().Where("name = ?", name).First(&ing).Error)
	return &ing
}

func requireReconcileCondition(t *testing.T, ing *model.Ingress, conditionType string) model.IngressCondition {
	t.Helper()
	c, ok := FindCondition(&ing.Status, conditionType)
	require.True(t, ok, "condition %s missing", conditionType)
	return c
}

func TestReconcileAllInSyncChangesNothing(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "web")
	before := loadReconcileTestIngress(t, "web")

	require.NoError(t, ReconcileAll(context.Background()))

	assert.Empty(t, env.enqueuedNames())
	after := loadReconcileTestIngress(t, "web")
	assert.Equal(t, before.Status, after.Status)
	assert.Equal(t, model.IngressPhaseReady, after.Status.Phase)
}

func TestReconcileAllMarksDriftAndEnqueues(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(t *testing.T)
		wantMsg string
	}{
		{
			name: "modified site file",
			mutate: func(t *testing.T) {
				path, err := site.ResolveAvailablePath("ingress-web")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, []byte("server { listen 80; } # edited\n"), 0o644))
			},
			wantMsg: "sites-available/ingress-web modified",
		},
		{
			name: "missing site file",
			mutate: func(t *testing.T) {
				path, err := site.ResolveAvailablePath("ingress-web")
				require.NoError(t, err)
				require.NoError(t, os.Remove(path))
			},
			wantMsg: "sites-available/ingress-web missing",
		},
		{
			name: "missing enabled link",
			mutate: func(t *testing.T) {
				path, err := site.ResolveEnabledPath("ingress-web")
				require.NoError(t, err)
				require.NoError(t, os.Remove(path))
			},
			wantMsg: "sites-enabled/ingress-web missing",
		},
		{
			name: "modified upstream file",
			mutate: func(t *testing.T) {
				path, err := managed.Path(UpstreamName("web", "app"))
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, []byte("upstream ig-web-app { server 127.0.0.1:9; }\n"), 0o644))
			},
			wantMsg: "conf.d/upstream-ig-web-app.conf modified",
		},
		{
			name: "missing upstream file",
			mutate: func(t *testing.T) {
				path, err := managed.Path(UpstreamName("web", "app"))
				require.NoError(t, err)
				require.NoError(t, os.Remove(path))
			},
			wantMsg: "conf.d/upstream-ig-web-app.conf missing",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupReconcileTest(t)
			env.newIngress(t, "web")
			tc.mutate(t)

			require.NoError(t, ReconcileAll(context.Background()))

			assert.Equal(t, []string{"web"}, env.enqueuedNames())
			ing := loadReconcileTestIngress(t, "web")
			applied := requireReconcileCondition(t, ing, model.IngressConditionConfigApplied)
			assert.True(t, applied.Status, "the previous config is still live")
			assert.Equal(t, reconcileReasonDrifted, applied.Reason)
			assert.Contains(t, applied.Message, tc.wantMsg)
			assert.Equal(t, model.IngressPhaseReady, ing.Status.Phase)
		})
	}
}

func TestReconcileAllSkipsPendingGeneration(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "web")
	require.NoError(t, model.UseDB().Model(&model.Ingress{}).Where("name = ?", "web").
		Update("generation", 2).Error)
	path, err := site.ResolveAvailablePath("ingress-web")
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	before := loadReconcileTestIngress(t, "web")

	require.NoError(t, ReconcileAll(context.Background()))

	assert.Empty(t, env.enqueuedNames())
	assert.Equal(t, before.Status, loadReconcileTestIngress(t, "web").Status)
}

func TestReconcileAllKeepsFailedApplyReason(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "web")
	_, err := UpdateStatus("web", func(ing *model.Ingress) {
		SetCondition(&ing.Status, model.IngressCondition{
			Type: model.IngressConditionConfigApplied, Reason: "TestFailed", Message: "nginx: [emerg]",
		})
	})
	require.NoError(t, err)
	path, err := site.ResolveAvailablePath("ingress-web")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("previous config\n"), 0o644))

	require.NoError(t, ReconcileAll(context.Background()))

	assert.Empty(t, env.enqueuedNames(), "retrying a failed apply without a change only fails again")
	applied := requireReconcileCondition(t, loadReconcileTestIngress(t, "web"), model.IngressConditionConfigApplied)
	assert.False(t, applied.Status)
	assert.Equal(t, "TestFailed", applied.Reason)
}

func TestReconcileAllRecordsRenewedCertificate(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "web")
	renewed := reconcileTestNotAfter.Add(90 * 24 * time.Hour)
	state := env.certs["web"]
	state.NotAfter = &renewed
	env.certs["web"] = state

	require.NoError(t, ReconcileAll(context.Background()))

	// Same paths, same rendered files: nothing to apply.
	assert.Empty(t, env.enqueuedNames())
	ing := loadReconcileTestIngress(t, "web")
	require.NotNil(t, ing.Status.CertNotAfter)
	assert.True(t, renewed.Equal(*ing.Status.CertNotAfter))
	cert := requireReconcileCondition(t, ing, model.IngressConditionCertReady)
	assert.True(t, cert.Status)
	assert.Equal(t, reconcileReasonIssued, cert.Reason)
	applied := requireReconcileCondition(t, ing, model.IngressConditionConfigApplied)
	assert.Equal(t, "Applied", applied.Reason)
}

func TestReconcileAllAppliesNewlyReadyCertificate(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "web")
	// The worker applied the HTTP listener while issuance was running.
	env.certs["web"] = CertState{Reason: "Issuing"}
	env.writeFiles(t, loadReconcileTestIngress(t, "web"))
	_, err := UpdateStatus("web", func(ing *model.Ingress) {
		ing.Status.CertID, ing.Status.CertNotAfter = 0, nil
		SetCondition(&ing.Status, model.IngressCondition{Type: model.IngressConditionCertReady, Reason: "Issuing"})
	})
	require.NoError(t, err)
	require.Equal(t, model.IngressPhaseDegraded, loadReconcileTestIngress(t, "web").Status.Phase)

	notAfter := reconcileTestNotAfter
	env.certs["web"] = CertState{Ready: true, CertID: 31, CertPath: "/etc/nginx/ssl/web/fullchain.cer", NotAfter: &notAfter}

	require.NoError(t, ReconcileAll(context.Background()))

	assert.Equal(t, []string{"web"}, env.enqueuedNames())
	ing := loadReconcileTestIngress(t, "web")
	cert := requireReconcileCondition(t, ing, model.IngressConditionCertReady)
	assert.True(t, cert.Status)
	assert.Equal(t, uint64(31), ing.Status.CertID)
	applied := requireReconcileCondition(t, ing, model.IngressConditionConfigApplied)
	assert.Equal(t, "Applied", applied.Reason, "a new certificate is not drift")
}

func TestReconcileAllMarksExpiredCertificate(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "web")
	env.now = reconcileTestNotAfter.Add(time.Hour)

	require.NoError(t, ReconcileAll(context.Background()))

	ing := loadReconcileTestIngress(t, "web")
	cert := requireReconcileCondition(t, ing, model.IngressConditionCertReady)
	assert.False(t, cert.Status)
	assert.Equal(t, reconcileReasonExpired, cert.Reason)
	assert.Equal(t, model.IngressPhaseDegraded, ing.Status.Phase)
	assert.Empty(t, env.enqueuedNames())
}

// Issuance state lives in memory only, so the reconcile pass restarts it for
// an ACME ingress whose certificate is missing or misses a host, e.g. after a
// restart.
func TestReconcileAllEnsuresMissingCertificates(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "issued")
	env.newIngress(t, "missing")
	env.certs["missing"] = CertState{Reason: CertReasonCertNotFound, Message: "issuance has not started yet"}
	env.writeFiles(t, loadReconcileTestIngress(t, "missing"))
	env.newIngress(t, "grown")
	grown := env.certs["grown"]
	grown.Reason = CertReasonIssueFailed
	env.certs["grown"] = grown
	plain := env.newIngress(t, "plain")
	plain.Spec.TLS = model.IngressTLS{Mode: model.IngressTLSModeNone}
	require.NoError(t, model.UseDB().Model(plain).Select("spec").Updates(&model.Ingress{Spec: plain.Spec}).Error)
	env.certs["plain"] = CertState{Reason: CertReasonNotRequired}
	env.writeFiles(t, loadReconcileTestIngress(t, "plain"))

	require.NoError(t, ReconcileAll(context.Background()))

	assert.ElementsMatch(t, []string{"missing", "grown"}, env.ensuredNames())
}

func TestReconcileAllRefreshesBackendHealth(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "web")
	env.avail["127.0.0.1:24002"] = &upstream.Status{Online: false}

	require.NoError(t, ReconcileAll(context.Background()))

	ing := loadReconcileTestIngress(t, "web")
	health := requireReconcileCondition(t, ing, model.IngressConditionBackendsHealthy)
	assert.True(t, health.Status)
	assert.Equal(t, reconcileReasonPartiallyHealthy, health.Reason)
	assert.Equal(t, []model.IngressServerStatus{
		{Backend: "app", Address: "127.0.0.1:24001", Healthy: true},
		{Backend: "app", Address: "127.0.0.1:24002", Healthy: false},
	}, ing.Status.Servers)
	assert.Empty(t, env.enqueuedNames())

	env.avail["127.0.0.1:24001"] = &upstream.Status{Online: false}
	require.NoError(t, ReconcileAll(context.Background()))

	ing = loadReconcileTestIngress(t, "web")
	health = requireReconcileCondition(t, ing, model.IngressConditionBackendsHealthy)
	assert.False(t, health.Status)
	assert.Equal(t, reconcileReasonNoHealthyServer, health.Reason)
	assert.Equal(t, model.IngressPhaseDegraded, ing.Status.Phase)
}

func TestReconcileAllContinuesAfterFailingIngress(t *testing.T) {
	env := setupReconcileTest(t)
	env.newIngress(t, "alpha")
	env.newIngress(t, "beta")
	env.newIngress(t, "gamma")
	env.renderErr["alpha"] = errors.New("render exploded")
	env.certErrs["beta"] = errors.New("certificate store unavailable")
	path, err := managed.Path(UpstreamName("gamma", "app"))
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	env.avail["127.0.0.1:24002"] = &upstream.Status{Online: false}

	err = ReconcileAll(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "alpha")
	assert.Contains(t, err.Error(), "render exploded")
	assert.Contains(t, err.Error(), "beta")
	assert.Equal(t, []string{"gamma"}, env.enqueuedNames())
	// Health is refreshed even for the ingresses that failed.
	for _, name := range []string{"alpha", "beta", "gamma"} {
		health := requireReconcileCondition(t, loadReconcileTestIngress(t, name), model.IngressConditionBackendsHealthy)
		assert.Equal(t, reconcileReasonPartiallyHealthy, health.Reason, name)
	}
}

func TestBackendHealth(t *testing.T) {
	online := &upstream.Status{Online: true}
	offline := &upstream.Status{Online: false}
	spec := func(backends ...model.IngressBackend) *model.IngressSpec {
		return &model.IngressSpec{Backends: backends}
	}
	backend := func(name string, servers ...model.IngressServer) model.IngressBackend {
		return model.IngressBackend{Name: name, Servers: servers}
	}
	server := func(address string, down bool) model.IngressServer {
		return model.IngressServer{Address: address, Down: down}
	}

	cases := []struct {
		name       string
		spec       *model.IngressSpec
		avail      map[string]*upstream.Status
		checkerOn  bool
		wantStatus bool
		wantReason string
		wantHealth []bool
	}{
		{
			name:       "all healthy",
			spec:       spec(backend("app", server("127.0.0.1:1", false), server("127.0.0.1:2", false))),
			avail:      map[string]*upstream.Status{"127.0.0.1:1": online, "127.0.0.1:2": online},
			checkerOn:  true,
			wantStatus: true, wantReason: reconcileReasonAllHealthy,
			wantHealth: []bool{true, true},
		},
		{
			name:       "a down server does not count",
			spec:       spec(backend("app", server("127.0.0.1:1", false), server("127.0.0.1:2", true))),
			avail:      map[string]*upstream.Status{"127.0.0.1:1": online, "127.0.0.1:2": offline},
			checkerOn:  true,
			wantStatus: true, wantReason: reconcileReasonAllHealthy,
			wantHealth: []bool{true, false},
		},
		{
			name: "partially healthy",
			spec: spec(
				backend("app", server("127.0.0.1:1", false), server("127.0.0.1:2", false)),
				backend("api", server("127.0.0.1:3", false)),
			),
			avail:      map[string]*upstream.Status{"127.0.0.1:1": online, "127.0.0.1:2": offline, "127.0.0.1:3": online},
			checkerOn:  true,
			wantStatus: true, wantReason: reconcileReasonPartiallyHealthy,
			wantHealth: []bool{true, false, true},
		},
		{
			name:       "an unchecked server next to a healthy one",
			spec:       spec(backend("app", server("127.0.0.1:1", false), server("127.0.0.1:2", false))),
			avail:      map[string]*upstream.Status{"127.0.0.1:1": online},
			checkerOn:  true,
			wantStatus: true, wantReason: reconcileReasonPartiallyHealthy,
			wantHealth: []bool{true, false},
		},
		{
			name: "one backend without a healthy server",
			spec: spec(
				backend("app", server("127.0.0.1:1", false)),
				backend("api", server("127.0.0.1:2", false)),
			),
			avail:      map[string]*upstream.Status{"127.0.0.1:1": online, "127.0.0.1:2": offline},
			checkerOn:  true,
			wantStatus: false, wantReason: reconcileReasonNoHealthyServer,
			wantHealth: []bool{true, false},
		},
		{
			name:       "every server of a backend down",
			spec:       spec(backend("app", server("127.0.0.1:1", true))),
			avail:      map[string]*upstream.Status{"127.0.0.1:1": online},
			checkerOn:  true,
			wantStatus: false, wantReason: reconcileReasonNoHealthyServer,
			wantHealth: []bool{true},
		},
		{
			name:       "no results yet",
			spec:       spec(backend("app", server("127.0.0.1:1", false))),
			avail:      map[string]*upstream.Status{},
			checkerOn:  true,
			wantStatus: false, wantReason: ReasonUnknown,
			wantHealth: []bool{false},
		},
		{
			name:       "checker disabled ignores stale results",
			spec:       spec(backend("app", server("127.0.0.1:1", false))),
			avail:      map[string]*upstream.Status{"127.0.0.1:1": online},
			checkerOn:  false,
			wantStatus: false, wantReason: ReasonUnknown,
			wantHealth: []bool{false},
		},
		{
			name:       "ipv6 address",
			spec:       spec(backend("app", server("[::1]:24001", false))),
			avail:      map[string]*upstream.Status{"[::1]:24001": online},
			checkerOn:  true,
			wantStatus: true, wantReason: reconcileReasonAllHealthy,
			wantHealth: []bool{true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			servers, condition := backendHealth(tc.spec, tc.avail, tc.checkerOn)

			assert.Equal(t, model.IngressConditionBackendsHealthy, condition.Type)
			assert.Equal(t, tc.wantStatus, condition.Status)
			assert.Equal(t, tc.wantReason, condition.Reason, condition.Message)
			health := make([]bool, len(servers))
			for i, s := range servers {
				health[i] = s.Healthy
			}
			assert.Equal(t, tc.wantHealth, health)
		})
	}
}

func TestBackendHealthUnknownDoesNotDegradePhase(t *testing.T) {
	ing := &model.Ingress{Generation: 1, Status: model.IngressStatus{ObservedGeneration: 1}}
	SetCondition(&ing.Status, model.IngressCondition{Type: model.IngressConditionConfigApplied, Status: true, Reason: "Applied"})
	_, condition := backendHealth(&model.IngressSpec{Backends: []model.IngressBackend{{
		Name: "app", Servers: []model.IngressServer{{Address: "127.0.0.1:1"}},
	}}}, nil, false)
	SetCondition(&ing.Status, condition)

	assert.Equal(t, model.IngressPhaseReady, ComputePhase(ing))
}

func TestStartReconcileStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		startReconcile(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("startReconcile did not stop after the context ended")
	}
}

// The upstream names the renderer derives must be valid managed upstream
// names, or the drift check cannot locate their files.
func TestUpstreamNamesAreManagedNames(t *testing.T) {
	for _, name := range []string{UpstreamName("a", "b"), UpstreamName(strings.Repeat("a", 40), strings.Repeat("b", 16))} {
		assert.NoError(t, managed.ValidateName(name), name)
	}
}
