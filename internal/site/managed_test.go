package site

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

const managedSiteTestContent = "server {\n    listen 80;\n    server_name managed.example.com;\n}\n"

// setupManagedSiteTest prepares a temp nginx tree with the site file of name,
// its row (managedBy may be empty) and optionally the enabled link or the
// maintenance file. It returns the conf dir.
func setupManagedSiteTest(t *testing.T, name, managedBy string, enabled, maintenance bool) string {
	t.Helper()

	confDir, _ := setupSiteMutationTest(t)
	require.NoError(t, model.UseDB().AutoMigrate(&model.Namespace{}, &model.Cert{}))

	originalSyncDelete := syncDelete
	syncDelete = func(context.Context, string) {}
	t.Cleanup(func() {
		WaitForSync()
		syncDelete = originalSyncDelete
	})

	availablePath, err := ResolveAvailablePath(name)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(availablePath, []byte(managedSiteTestContent), 0o644))
	require.NoError(t, query.Site.Create(&model.Site{Path: availablePath, ManagedBy: managedBy}))

	if enabled {
		enabledPath, err := resolveEnabledSymlinkPath(name)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(availablePath, enabledPath))
	}
	if maintenance {
		maintenancePath, err := ResolveEnabledMaintenancePath(name)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(maintenancePath, []byte(managedSiteTestContent), 0o644))
	}
	return confDir
}

func requireManagedError(t *testing.T, err error, ingress string) {
	t.Helper()

	require.Error(t, err)
	require.True(t, IsManagedByIngress(err), "want ErrSiteManagedByIngress, got %v", err)
	var cErr *cosy.Error
	require.True(t, errors.As(err, &cErr))
	assert.Equal(t, int32(40901), cErr.Code)
	assert.Equal(t, []string{ingress}, cErr.Params)
}

func managedTestPathExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false
	}
	require.NoError(t, err)
	return true
}

func TestCheckManaged(t *testing.T) {
	setupManagedSiteTest(t, "ingress-web", "ingress:web", false, false)

	requireManagedError(t, CheckManaged("ingress-web"), "web")
	requireManagedError(t, CheckManaged("ingress-web", Force(false)), "web")
	assert.NoError(t, CheckManaged("ingress-web", Force(true)))
	// A site without a row is never managed.
	assert.NoError(t, CheckManaged("unknown.example.com"))

	availablePath, err := ResolveAvailablePath("plain.example.com")
	require.NoError(t, err)
	require.NoError(t, query.Site.Create(&model.Site{Path: availablePath}))
	assert.NoError(t, CheckManaged("plain.example.com"))
}

func TestCheckManagedPath(t *testing.T) {
	confDir := setupManagedSiteTest(t, "ingress-web", "ingress:web", false, false)

	availablePath, err := ResolveAvailablePath("ingress-web")
	require.NoError(t, err)
	enabledPath, err := ResolveEnabledPath("ingress-web")
	require.NoError(t, err)

	requireManagedError(t, CheckManagedPath(availablePath), "web")
	requireManagedError(t, CheckManagedPath(enabledPath), "web")
	assert.NoError(t, CheckManagedPath(availablePath, Force(true)))
	// The same file name elsewhere is not a site.
	assert.NoError(t, CheckManagedPath(filepath.Join(confDir, "conf.d", "ingress-web")))
}

// managedSiteOperation is one guarded entry point, set up in the state it
// needs and checked for having had no effect when refused.
type managedSiteOperation struct {
	name        string
	enabled     bool
	maintenance bool
	run         func(name string, opts ...MutateOption) error
	unchanged   func(t *testing.T, name string)
}

func managedSiteOperations() []managedSiteOperation {
	ctx := context.Background()
	availableUnchanged := func(t *testing.T, name string) {
		path, err := ResolveAvailablePath(name)
		require.NoError(t, err)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, managedSiteTestContent, string(content))
	}
	enabledIs := func(want bool) func(t *testing.T, name string) {
		return func(t *testing.T, name string) {
			path, err := resolveEnabledSymlinkPath(name)
			require.NoError(t, err)
			assert.Equal(t, want, managedTestPathExists(t, path))
			availableUnchanged(t, name)
		}
	}
	maintenanceIs := func(want bool) func(t *testing.T, name string) {
		return func(t *testing.T, name string) {
			path, err := ResolveEnabledMaintenancePath(name)
			require.NoError(t, err)
			assert.Equal(t, want, managedTestPathExists(t, path))
			availableUnchanged(t, name)
		}
	}

	return []managedSiteOperation{
		{
			name: "save",
			run: func(name string, opts ...MutateOption) error {
				return Save(ctx, name, "server {\n    listen 8080;\n}\n", true, 0, nil, "", opts...)
			},
			unchanged: availableUnchanged,
		},
		{
			name: "rename",
			run: func(name string, opts ...MutateOption) error {
				return Rename(ctx, name, name+"-renamed", opts...)
			},
			unchanged: availableUnchanged,
		},
		{
			name: "enable",
			run: func(name string, opts ...MutateOption) error {
				return Enable(ctx, name, opts...)
			},
			unchanged: enabledIs(false),
		},
		{
			name:    "disable",
			enabled: true,
			run: func(name string, opts ...MutateOption) error {
				return Disable(ctx, name, opts...)
			},
			unchanged: enabledIs(true),
		},
		{
			name: "delete",
			run: func(name string, opts ...MutateOption) error {
				return Delete(ctx, name, opts...)
			},
			unchanged: availableUnchanged,
		},
		{
			name: "enable maintenance",
			run: func(name string, opts ...MutateOption) error {
				return EnableMaintenance(ctx, name, opts...)
			},
			unchanged: maintenanceIs(false),
		},
		{
			name: "enable maintenance with payload",
			run: func(name string, opts ...MutateOption) error {
				return EnableMaintenanceWithPayload(ctx, name, &MaintenancePayload{Contact: "ops"}, opts...)
			},
			unchanged: maintenanceIs(false),
		},
		{
			name:        "disable maintenance",
			maintenance: true,
			run: func(name string, opts ...MutateOption) error {
				return DisableMaintenance(ctx, name, opts...)
			},
			unchanged: maintenanceIs(true),
		},
	}
}

func TestManagedSiteMutationsAreRefusedUnlessForced(t *testing.T) {
	for _, op := range managedSiteOperations() {
		t.Run(op.name+"/refused", func(t *testing.T) {
			setupManagedSiteTest(t, "ingress-web", "ingress:web", op.enabled, op.maintenance)

			requireManagedError(t, op.run("ingress-web"), "web")
			op.unchanged(t, "ingress-web")
		})

		t.Run(op.name+"/forced", func(t *testing.T) {
			setupManagedSiteTest(t, "ingress-web", "ingress:web", op.enabled, op.maintenance)

			assert.NoError(t, op.run("ingress-web", Force(true)))
		})

		t.Run(op.name+"/unmanaged", func(t *testing.T) {
			setupManagedSiteTest(t, "plain.example.com", "", op.enabled, op.maintenance)

			assert.NoError(t, op.run("plain.example.com"))
		})
	}
}

func TestHTTPSOnboardingRefusesManagedSite(t *testing.T) {
	setupManagedSiteTest(t, "ingress-web", "ingress:web", true, false)

	onboarding := &HTTPSOnboarding{Name: "ingress-web"}
	_, err := onboarding.inspectSite()
	requireManagedError(t, err, "web")
}
