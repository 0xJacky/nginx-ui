package ingress

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckManagedPathCoversSiteAndUpstreamFiles(t *testing.T) {
	env := setupEnv(t)
	upsertBasic(t, "web", "web.example.com")
	require.NoError(t, model.UseDB().Create(&model.Site{
		Path: env.path("sites-available", "ingress-web"), ManagedBy: ManagedBy("web"),
	}).Error)

	owner, err := UpstreamOwner("ig-web-app")
	require.NoError(t, err)
	assert.Equal(t, "web", owner)
	owner, err = UpstreamOwner("ig-web-other")
	require.NoError(t, err)
	assert.Empty(t, owner)

	err = CheckManagedPath(env.path("conf.d", "upstream-ig-web-app.conf"), false)
	requireCode(t, err, managed.ErrManagedByIngress)
	assert.True(t, IsManagedError(err))
	err = CheckManagedPath(env.path("sites-available", "ingress-web"), false)
	assert.True(t, site.IsManagedByIngress(err), "got %v", err)
	assert.True(t, IsManagedError(err))

	assert.NoError(t, CheckManagedPath(env.path("conf.d", "upstream-ig-web-app.conf"), true))
	assert.NoError(t, CheckManagedPath(env.path("conf.d", "upstream-app.conf"), false))
	assert.NoError(t, CheckManagedPath(env.path("conf.d", "other.conf"), false))
	assert.NoError(t, CheckUpstream("app", false))
	requireCode(t, CheckUpstream("ig-web-app", false), managed.ErrManagedByIngress)
}

func TestUpstreamNameUsesTheReservedPrefix(t *testing.T) {
	requireCode(t, managed.CheckUserName(UpstreamName("web", "app")), managed.ErrReservedName)
	assert.NoError(t, managed.CheckUserName("web-app"))
}
