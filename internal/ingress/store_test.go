package ingress

import (
	"context"
	"os"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertOwnerMismatchAndForceTransfer(t *testing.T) {
	setupEnv(t)
	upsertBasic(t, "owned", "owned.example.com")

	_, err := Upsert(context.Background(), "owned", UpsertRequest{Owner: "intruder", Spec: basicSpec("owned.example.com")})
	requireCode(t, err, ErrOwnerMismatch)

	ing, err := Upsert(context.Background(), "owned", UpsertRequest{Owner: "intruder", Spec: basicSpec("owned.example.com"), Force: true})
	require.NoError(t, err)
	assert.Equal(t, "intruder", ing.Owner)
	assert.Equal(t, int64(1), ing.Generation, "an owner transfer alone does not bump the generation")
}

func TestUpsertGenerationBumpRules(t *testing.T) {
	setupEnv(t)
	namespace := &model.Namespace{Name: "ns"}
	require.NoError(t, model.UseDB().Create(namespace).Error)

	ing := upsertBasic(t, "gen", "gen.example.com")
	assert.Equal(t, int64(1), ing.Generation)
	assert.Equal(t, model.IngressPhasePending, ing.Status.Phase)
	assert.Equal(t, "ingress-gen", ing.SitePath)

	req := UpsertRequest{Owner: "tester", Spec: basicSpec("gen.example.com")}
	ing, err := Upsert(context.Background(), "gen", req)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ing.Generation, "same spec")

	req.Labels = map[string]string{"service": "x"}
	req.ExternalID = "svc:1"
	ing, err = Upsert(context.Background(), "gen", req)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ing.Generation, "labels and external id")
	assert.Equal(t, "x", ing.Labels["service"])
	assert.Equal(t, "svc:1", ing.ExternalID)

	req.Spec.Routes[0].ReadTimeout = "120s"
	ing, err = Upsert(context.Background(), "gen", req)
	require.NoError(t, err)
	assert.Equal(t, int64(2), ing.Generation, "spec change")

	req.NamespaceID = namespace.ID
	ing, err = Upsert(context.Background(), "gen", req)
	require.NoError(t, err)
	assert.Equal(t, int64(3), ing.Generation, "namespace change")
}

func TestUpsertValidates(t *testing.T) {
	setupEnv(t)
	_, err := Upsert(context.Background(), "Bad_Name", UpsertRequest{Owner: "tester", Spec: basicSpec("a.example.com")})
	requireCode(t, err, ErrInvalidName)
	_, err = Upsert(context.Background(), "ok", UpsertRequest{Owner: "bad owner", Spec: basicSpec("a.example.com")})
	requireCode(t, err, ErrInvalidOwner)
	spec := basicSpec("a.example.com")
	spec.Hosts = nil
	_, err = Upsert(context.Background(), "ok", UpsertRequest{Owner: "tester", Spec: spec})
	requireCode(t, err, ErrInvalidHost)
}

func TestUpsertHostConflict(t *testing.T) {
	setupEnv(t)
	upsertBasic(t, "first", "shared.example.com")
	_, err := Upsert(context.Background(), "second", UpsertRequest{Owner: "other", Spec: basicSpec("SHARED.example.com")})
	requireCode(t, err, ErrHostConflict)
	assert.Contains(t, err.Error(), "first")
}

func TestUpsertUpstreamNameCollision(t *testing.T) {
	setupEnv(t)
	spec := basicSpec("a.example.com")
	spec.Backends[0].Name = "b-c"
	spec.Routes[0].Backend = "b-c"
	_, err := Upsert(context.Background(), "a", UpsertRequest{Owner: "tester", Spec: spec})
	require.NoError(t, err)

	spec = basicSpec("ab.example.com")
	spec.Backends[0].Name = "c"
	spec.Routes[0].Backend = "c"
	_, err = Upsert(context.Background(), "a-b", UpsertRequest{Owner: "tester", Spec: spec})
	requireCode(t, err, ErrInvalidBackend)
	assert.Contains(t, err.Error(), "ig-a-b-c")
}

func TestUpsertRefusesUnmanagedSite(t *testing.T) {
	env := setupEnv(t)
	require.NoError(t, os.WriteFile(env.path("sites-available", "ingress-taken"), []byte("server {}\n"), 0o644))
	_, err := Upsert(context.Background(), "taken", UpsertRequest{Owner: "tester", Spec: basicSpec("taken.example.com")})
	requireCode(t, err, ErrSiteConflict)
}

func TestSyncOwnerPrunesOnlyTheOwnersIngresses(t *testing.T) {
	env := setupEnv(t)
	for _, item := range []struct{ name, owner string }{{"a1", "alpha"}, {"a2", "alpha"}, {"b1", "beta"}} {
		_, err := Upsert(context.Background(), item.name, UpsertRequest{Owner: item.owner, Spec: basicSpec(item.name + ".example.com")})
		require.NoError(t, err)
	}
	env.startWorker(t)
	for _, name := range []string{"a1", "a2", "b1"} {
		ing, err := Get(name)
		require.NoError(t, err)
		waitApplied(t, ing)
	}
	reloadsBefore := env.reloads()

	// a3 takes over the host of the pruned a2 in the same sync.
	items, err := SyncOwner(context.Background(), "alpha", SyncRequest{Items: []SyncItem{
		{Name: "a1", Spec: basicSpec("a1.example.com")},
		{Name: "a3", Spec: basicSpec("a2.example.com")},
	}}, true)
	require.NoError(t, err)
	names := make([]string, 0, len(items))
	for _, ing := range items {
		names = append(names, ing.Name)
	}
	assert.Equal(t, []string{"a1", "a3"}, names)

	a3, err := Get("a3")
	require.NoError(t, err)
	waitApplied(t, a3)
	eventually(t, func() bool {
		_, err := Get("a2")
		return err != nil
	}, "a2 was not pruned")

	_, err = Get("b1")
	require.NoError(t, err, "another owner's ingress must survive a prune")
	assert.True(t, env.exists("sites-enabled", "ingress-b1"))
	assert.False(t, env.exists("sites-available", "ingress-a2"))
	assert.Equal(t, reloadsBefore+1, env.reloads(), "the sync lands in one reload")
}

func TestSyncOwnerRefusesAnotherOwnersIngress(t *testing.T) {
	setupEnv(t)
	_, err := Upsert(context.Background(), "theirs", UpsertRequest{Owner: "beta", Spec: basicSpec("theirs.example.com")})
	require.NoError(t, err)

	_, err = SyncOwner(context.Background(), "alpha", SyncRequest{Items: []SyncItem{
		{Name: "mine", Spec: basicSpec("mine.example.com")},
		{Name: "theirs", Spec: basicSpec("theirs.example.com")},
	}}, true)
	requireCode(t, err, ErrOwnerMismatch)
	_, err = Get("mine")
	requireCode(t, err, ErrNotFound)

	_, err = SyncOwner(context.Background(), "alpha", SyncRequest{Items: []SyncItem{
		{Name: "dup", Spec: basicSpec("dup1.example.com")},
		{Name: "dup", Spec: basicSpec("dup2.example.com")},
	}}, false)
	requireCode(t, err, ErrInvalidName)

	_, err = SyncOwner(context.Background(), "alpha", SyncRequest{Items: []SyncItem{
		{Name: "x", Spec: basicSpec("same.example.com")},
		{Name: "y", Spec: basicSpec("same.example.com")},
	}}, false)
	requireCode(t, err, ErrHostConflict)
}

func TestPatchServer(t *testing.T) {
	setupEnv(t)
	spec := basicSpec("patch.example.com")
	spec.Backends[0].Servers = append(spec.Backends[0].Servers, model.IngressServer{Address: "[::1]:24002"})
	_, err := Upsert(context.Background(), "patch", UpsertRequest{Owner: "tester", Spec: spec})
	require.NoError(t, err)

	ing, err := PatchServer(context.Background(), "patch", "app", ServerPatch{Owner: "tester", Address: "[0:0::1]:24002", Down: true})
	require.NoError(t, err)
	assert.Equal(t, int64(2), ing.Generation)
	assert.True(t, ing.Spec.Backends[0].Servers[1].Down)
	assert.False(t, ing.Spec.Backends[0].Servers[0].Down)

	ing, err = PatchServer(context.Background(), "patch", "app", ServerPatch{Owner: "tester", Address: "[::1]:24002", Down: true})
	require.NoError(t, err)
	assert.Equal(t, int64(2), ing.Generation, "no change, no bump")

	_, err = PatchServer(context.Background(), "patch", "app", ServerPatch{Owner: "tester", Address: "127.0.0.1:9", Down: true})
	requireCode(t, err, ErrServerNotFound)
	_, err = PatchServer(context.Background(), "patch", "nope", ServerPatch{Owner: "tester", Address: "127.0.0.1:24001", Down: true})
	requireCode(t, err, ErrServerNotFound)
	_, err = PatchServer(context.Background(), "patch", "app", ServerPatch{Owner: "intruder", Address: "127.0.0.1:24001", Down: true})
	requireCode(t, err, ErrOwnerMismatch)
	_, err = PatchServer(context.Background(), "missing", "app", ServerPatch{Owner: "tester", Address: "127.0.0.1:24001"})
	requireCode(t, err, ErrNotFound)
}

func TestDryRunDoesNotStore(t *testing.T) {
	env := setupEnv(t)
	result, err := DryRun(context.Background(), "dry", UpsertRequest{Owner: "tester", Spec: basicSpec("dry.example.com")})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	require.NotNil(t, result.Rendered)
	assert.Contains(t, result.Rendered.SiteContent, "server_name dry.example.com;")
	_, err = Get("dry")
	requireCode(t, err, ErrNotFound)
	assert.False(t, env.exists("sites-available", "ingress-dry"))
	assert.Zero(t, env.reloads())

	spec := basicSpec("dry.example.com")
	spec.Policy.ExtraHeaders = []model.IngressHeaderSet{{Name: "X-Marker", Value: brokenMarker}}
	result, err = DryRun(context.Background(), "dry", UpsertRequest{Owner: "tester", Spec: spec})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.TestOutput, "broken config")
}

func TestListFiltersByOwner(t *testing.T) {
	setupEnv(t)
	_, err := Upsert(context.Background(), "b", UpsertRequest{Owner: "one", Spec: basicSpec("b.example.com")})
	require.NoError(t, err)
	_, err = Upsert(context.Background(), "a", UpsertRequest{Owner: "two", Spec: basicSpec("a.example.com")})
	require.NoError(t, err)

	all, err := List("")
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "a", all[0].Name)
	one, err := List("one")
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, "b", one[0].Name)
	none, err := List("nobody")
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}
