package ingress

import (
	"context"
	"os"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyWritesFilesEnablesSiteAndReloadsOnce(t *testing.T) {
	env := setupEnv(t)
	a, b := newIngress("a", "a.example.com"), newIngress("b", "b.example.com")
	a.NamespaceID = 0

	results, err := Apply(context.Background(), []*model.Ingress{a, b}, ApplyOptions{})
	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Changed, r.Name)
		assert.Empty(t, r.Error, r.Name)
		require.NotNil(t, r.Rendered)
		assert.Equal(t, r.Rendered.SiteContent, env.read(t, "sites-available", SiteName(r.Name)))
		assert.Equal(t, r.Rendered.Upstreams[UpstreamName(r.Name, "app")],
			env.read(t, "conf.d", "upstream-"+UpstreamName(r.Name, "app")+".conf"))
		target, err := os.Readlink(env.path("sites-enabled", SiteName(r.Name)))
		require.NoError(t, err)
		assert.Equal(t, env.path("sites-available", SiteName(r.Name)), target)

		var site model.Site
		require.NoError(t, model.UseDB().Where("path = ?", env.path("sites-available", SiteName(r.Name))).First(&site).Error)
		assert.Equal(t, ManagedBy(r.Name), site.ManagedBy)
	}
	assert.Equal(t, 1, env.tests())
	assert.Equal(t, 1, env.reloads())
}

func TestApplyUnchangedSkipsTestAndReload(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("same", "same.example.com")
	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)

	results, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	assert.False(t, results[0].Changed)
	assert.Equal(t, 1, env.tests())
	assert.Equal(t, 1, env.reloads())
}

func TestApplyTestFailureRollsBackEveryFile(t *testing.T) {
	env := setupEnv(t)
	good := newIngress("good", "good.example.com")
	_, err := Apply(context.Background(), []*model.Ingress{good}, ApplyOptions{})
	require.NoError(t, err)
	previousSite := env.read(t, "sites-available", "ingress-good")
	previousUpstream := env.read(t, "conf.d", "upstream-ig-good-app.conf")

	changed := newIngress("good", "good.example.com")
	changed.Spec.Backends[0].Servers[0].Down = true
	changed.Spec.Routes[0].ReadTimeout = "90s"
	broken := newIngress("broken", "broken.example.com")
	broken.Spec.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-Marker", Value: brokenMarker}}

	results, err := Apply(context.Background(), []*model.Ingress{changed, broken}, ApplyOptions{})
	requireCode(t, err, ErrApplyFailed)
	for _, r := range results {
		assert.NotEmpty(t, r.Error, r.Name)
		assert.Contains(t, r.TestOutput, "broken config", r.Name)
		assert.False(t, r.Changed, r.Name)
	}

	assert.Equal(t, previousSite, env.read(t, "sites-available", "ingress-good"))
	assert.Equal(t, previousUpstream, env.read(t, "conf.d", "upstream-ig-good-app.conf"))
	assert.False(t, env.exists("sites-available", "ingress-broken"))
	assert.False(t, env.exists("sites-enabled", "ingress-broken"))
	assert.False(t, env.exists("conf.d", "upstream-ig-broken-app.conf"))
	assert.Equal(t, 1, env.reloads(), "a failed test must not reload")

	var count int64
	model.UseDB().Model(&model.Site{}).Where("managed_by = ?", ManagedBy("broken")).Count(&count)
	assert.Zero(t, count)
}

func TestApplyReloadFailureRestoresAndReloadsPreviousConfig(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("reload", "reload.example.com")
	failing := "echo r >> " + env.logPath("reloads") + "; exit 1"
	setReloadCmd(t, failing)

	results, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	requireCode(t, err, ErrApplyFailed)
	assert.NotEmpty(t, results[0].Error)
	assert.False(t, env.exists("sites-available", "ingress-reload"))
	assert.False(t, env.exists("sites-enabled", "ingress-reload"))
}

func TestApplyRefusesUnmanagedSite(t *testing.T) {
	env := setupEnv(t)
	handWritten := "server { listen 80; server_name mine.example.com; }\n"
	require.NoError(t, os.WriteFile(env.path("sites-available", "ingress-mine"), []byte(handWritten), 0o644))
	require.NoError(t, model.UseDB().Create(&model.Site{Path: env.path("sites-available", "ingress-mine")}).Error)

	results, err := Apply(context.Background(), []*model.Ingress{newIngress("mine", "mine.example.com")}, ApplyOptions{})
	requireCode(t, err, ErrApplyFailed)
	assert.Contains(t, results[0].Error, "not managed by this ingress")
	assert.Equal(t, handWritten, env.read(t, "sites-available", "ingress-mine"))
	assert.False(t, env.exists("conf.d", "upstream-ig-mine-app.conf"))
	assert.Zero(t, env.tests())
}

func TestApplyAdoptsOwnFileWithoutSiteRow(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("own", "own.example.com")
	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	require.NoError(t, model.UseDB().Unscoped().Where("managed_by = ?", ManagedBy("own")).Delete(&model.Site{}).Error)

	_, err = Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	var site model.Site
	require.NoError(t, model.UseDB().Where("managed_by = ?", ManagedBy("own")).First(&site).Error)
	assert.Equal(t, env.path("sites-available", "ingress-own"), site.Path)
}

func TestApplyRemovesStaleUpstreams(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("multi", "multi.example.com")
	ing.Spec.Backends = append(ing.Spec.Backends, model.IngressBackend{
		Name: "old", Servers: []model.IngressServer{{Address: "127.0.0.1:24009"}},
	})
	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	require.True(t, env.exists("conf.d", "upstream-ig-multi-old.conf"))

	// Another ingress whose upstream shares the prefix must survive.
	other := newIngress("multi-old", "other.example.com")
	other.Spec.Backends[0].Name = "x"
	other.Spec.Routes[0].Backend = "x"
	require.NoError(t, model.UseDB().Create(other).Error)
	_, err = Apply(context.Background(), []*model.Ingress{other}, ApplyOptions{})
	require.NoError(t, err)

	ing.Spec.Backends = ing.Spec.Backends[:1]
	_, err = Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	assert.False(t, env.exists("conf.d", "upstream-ig-multi-old.conf"))
	assert.True(t, env.exists("conf.d", "upstream-ig-multi-app.conf"))
	assert.True(t, env.exists("conf.d", "upstream-ig-multi-old-x.conf"))
}

func TestApplyDryRunRollsBackAndNeverReloads(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("dry", "dry.example.com")

	results, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{DryRun: true})
	require.NoError(t, err)
	assert.True(t, results[0].Changed)
	assert.NotNil(t, results[0].Rendered)
	assert.False(t, env.exists("sites-available", "ingress-dry"))
	assert.False(t, env.exists("sites-enabled", "ingress-dry"))
	assert.False(t, env.exists("conf.d", "upstream-ig-dry-app.conf"))
	assert.Equal(t, 1, env.tests())
	assert.Zero(t, env.reloads())

	ing.Spec.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-Marker", Value: brokenMarker}}
	results, err = Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{DryRun: true})
	requireCode(t, err, ErrApplyFailed)
	assert.Contains(t, results[0].TestOutput, "broken config")
	assert.False(t, env.exists("sites-available", "ingress-dry"))
	assert.Zero(t, env.reloads())

	var count int64
	model.UseDB().Model(&model.Site{}).Count(&count)
	assert.Zero(t, count)
}

func TestApplyRefusesRemoteNamespace(t *testing.T) {
	env := setupEnv(t)
	namespace := &model.Namespace{Name: "edge", DeployMode: model.DeployModeRemote}
	require.NoError(t, model.UseDB().Create(namespace).Error)
	ing := newIngress("remote", "remote.example.com")
	ing.NamespaceID = namespace.ID

	results, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	requireCode(t, err, ErrApplyFailed)
	assert.Contains(t, results[0].Error, "remote nodes only")
	assert.False(t, env.exists("sites-available", "ingress-remote"))
}

func TestApplyRecordsNamespaceOnSiteRow(t *testing.T) {
	env := setupEnv(t)
	namespace := &model.Namespace{Name: "local", DeployMode: model.DeployModeLocal}
	require.NoError(t, model.UseDB().Create(namespace).Error)
	ing := newIngress("ns", "ns.example.com")
	ing.NamespaceID = namespace.ID

	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	var site model.Site
	require.NoError(t, model.UseDB().Where("path = ?", env.path("sites-available", "ingress-ns")).First(&site).Error)
	assert.Equal(t, namespace.ID, site.NamespaceID)
}

func TestRemoveDeletesFilesAndSiteRow(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("gone", "gone.example.com")
	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)

	require.NoError(t, Remove(context.Background(), []*model.Ingress{ing}))
	assert.False(t, env.exists("sites-available", "ingress-gone"))
	assert.False(t, env.exists("sites-enabled", "ingress-gone"))
	assert.False(t, env.exists("conf.d", "upstream-ig-gone-app.conf"))
	assert.Equal(t, 2, env.reloads())

	var count int64
	model.UseDB().Model(&model.Site{}).Count(&count)
	assert.Zero(t, count)
}

func TestRemoveRollsBackWhenTestFails(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("kept", "kept.example.com")
	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	// A hand-written file elsewhere that breaks the test keeps the removal
	// from going through.
	require.NoError(t, os.WriteFile(env.path("conf.d", "other.conf"), []byte(brokenMarker), 0o644))

	requireCode(t, Remove(context.Background(), []*model.Ingress{ing}), ErrApplyFailed)
	assert.True(t, env.exists("sites-available", "ingress-kept"))
	assert.True(t, env.exists("sites-enabled", "ingress-kept"))
	assert.True(t, env.exists("conf.d", "upstream-ig-kept-app.conf"))
}

func TestApplyRemovesForcedMaintenanceVariant(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("maint", "maint.example.com")
	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)

	// A forced maintenance switch replaced the enabled link.
	require.NoError(t, os.Remove(env.path("sites-enabled", "ingress-maint")))
	require.NoError(t, os.WriteFile(env.path("sites-enabled", "ingress-maint_nginx_ui_maintenance"), []byte("server {}\n"), 0o644))

	results, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	assert.True(t, results[0].Changed)
	assert.True(t, env.exists("sites-enabled", "ingress-maint"))
	assert.False(t, env.exists("sites-enabled", "ingress-maint_nginx_ui_maintenance"))
}

// A managed upstream the user created under a generated name is never
// overwritten: neither the store nor the apply worker accepts it.
func TestApplyRefusesForeignManagedUpstream(t *testing.T) {
	env := setupEnv(t)
	foreign := "upstream ig-own-app {\n    server 10.0.0.1:80;\n}\n"
	require.NoError(t, os.WriteFile(env.path("conf.d", "upstream-ig-own-app.conf"), []byte(foreign), 0o644))

	results, err := Apply(context.Background(), []*model.Ingress{newIngress("own", "own.example.com")}, ApplyOptions{})
	requireCode(t, err, ErrApplyFailed)
	assert.Contains(t, results[0].Error, "ig-own-app")
	assert.Equal(t, foreign, env.read(t, "conf.d", "upstream-ig-own-app.conf"))
	assert.False(t, env.exists("sites-available", "ingress-own"))

	_, err = Upsert(context.Background(), "own", UpsertRequest{Owner: "tester", Spec: basicSpec("own.example.com")})
	requireCode(t, err, ErrUpstreamConflict)
}

// The ingress's own upstream files are rewritten: the ones its site file
// lists, and an identical leftover of an interrupted apply.
func TestApplyAcceptsOwnUpstreamFiles(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("own", "own.example.com")
	results, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)

	// Listed by the site file, even though it was edited by hand.
	require.NoError(t, os.WriteFile(env.path("conf.d", "upstream-ig-own-app.conf"), []byte("edited\n"), 0o644))
	_, err = Upsert(context.Background(), "own", UpsertRequest{Owner: "tester", Spec: basicSpec("own.example.com")})
	require.NoError(t, err)
	_, err = Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)

	// Identical content without a site file.
	require.NoError(t, os.Remove(env.path("sites-available", "ingress-own")))
	require.NoError(t, os.Remove(env.path("sites-enabled", "ingress-own")))
	assert.Equal(t, results[0].Rendered.Upstreams["ig-own-app"], env.read(t, "conf.d", "upstream-ig-own-app.conf"))
	_, err = Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
}

// The site list backfills a row without managed_by for a file it finds on
// disk; the ingress recognizes its marker and claims that row.
func TestApplyClaimsBackfilledSiteRow(t *testing.T) {
	env := setupEnv(t)
	ing := newIngress("own", "own.example.com")
	_, err := Apply(context.Background(), []*model.Ingress{ing}, ApplyOptions{})
	require.NoError(t, err)
	sitePath := env.path("sites-available", "ingress-own")
	require.NoError(t, model.UseDB().Model(&model.Site{}).Where("path = ?", sitePath).Update("managed_by", "").Error)

	_, err = Upsert(context.Background(), "own", UpsertRequest{Owner: "tester", Spec: basicSpec("own.example.com")})
	require.NoError(t, err)
	var row model.Site
	require.NoError(t, model.UseDB().Where("path = ?", sitePath).First(&row).Error)
	assert.Equal(t, ManagedBy("own"), row.ManagedBy)

	// A row without managed_by whose file lacks the marker stays the user's.
	require.NoError(t, os.WriteFile(env.path("sites-available", "ingress-mine"), []byte("server {}\n"), 0o644))
	require.NoError(t, model.UseDB().Create(&model.Site{Path: env.path("sites-available", "ingress-mine")}).Error)
	_, err = Upsert(context.Background(), "mine", UpsertRequest{Owner: "tester", Spec: basicSpec("mine.example.com")})
	requireCode(t, err, ErrSiteConflict)
}
