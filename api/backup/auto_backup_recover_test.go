package backup

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cron"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// createRecoverableAutoBackup stores an auto backup task. Enabled defaults to
// true in the schema, so a disabled task is switched off after it is created.
func createRecoverableAutoBackup(t *testing.T, name string, enabled bool) *model.AutoBackup {
	t.Helper()

	task := &model.AutoBackup{
		Name:           name,
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    t.TempDir(),
		CronExpression: "0 0 * * *",
		Enabled:        true,
	}
	require.NoError(t, model.UseDB().Create(task).Error)
	if !enabled {
		require.NoError(t, model.UseDB().Model(task).Update("enabled", false).Error)
		task.Enabled = false
	}
	t.Cleanup(func() {
		_ = cron.RemoveAutoBackupJob(task.ID)
	})
	return task
}

// listedAutoBackupIDs returns the ids the list endpoint shows, from the trash
// when trash is true.
func listedAutoBackupIDs(t *testing.T, router http.Handler, trash bool) []uint64 {
	t.Helper()

	path := "/auto_backup"
	if trash {
		path += "?trash=true"
	}
	recorder := pluginStorageRequest(router, http.MethodGet, path, "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var list struct {
		Data []model.AutoBackup `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &list), recorder.Body.String())
	ids := make([]uint64, 0, len(list.Data))
	for _, item := range list.Data {
		ids = append(ids, item.ID)
	}
	return ids
}

func autoBackupPath(id uint64) string {
	return "/auto_backup/" + strconv.FormatUint(id, 10)
}

func TestRestoreAutoBackupRecoversSoftDeletedTask(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run("enabled="+strconv.FormatBool(enabled), func(t *testing.T) {
			router, session := newPluginStorageRouter(t)
			task := createRecoverableAutoBackup(t, "daily backup", enabled)
			path := autoBackupPath(task.ID)

			recorder := pluginStorageRequest(router, http.MethodDelete, path, "", session)
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
			require.False(t, cron.HasAutoBackupJob(task.ID))
			require.NotContains(t, listedAutoBackupIDs(t, router, false), task.ID)
			require.Contains(t, listedAutoBackupIDs(t, router, true), task.ID)

			// The trash view recovers a task with PATCH and an empty body.
			recorder = pluginStorageRequest(router, http.MethodPatch, path, "{}", session)
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())

			require.Contains(t, listedAutoBackupIDs(t, router, false), task.ID)
			require.NotContains(t, listedAutoBackupIDs(t, router, true), task.ID)

			recorder = pluginStorageRequest(router, http.MethodGet, path, "", "")
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

			var recovered model.AutoBackup
			require.NoError(t, model.UseDB().First(&recovered, task.ID).Error)
			require.Equal(t, task.Name, recovered.Name)
			require.Equal(t, enabled, recovered.Enabled)
			require.Equal(t, task.CronExpression, recovered.CronExpression)
			// Deleting dropped the job; recovering schedules it again only for an
			// enabled task.
			require.Equal(t, enabled, cron.HasAutoBackupJob(task.ID))
		})
	}
}

func TestRestoreAutoBackupUnknownIDIsNotFound(t *testing.T) {
	router, session := newPluginStorageRouter(t)
	task := createRecoverableAutoBackup(t, "daily backup", false)

	recorder := pluginStorageRequest(router, http.MethodPatch, autoBackupPath(task.ID+1000), "{}", session)
	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.False(t, cron.HasAutoBackupJob(task.ID+1000))

	// The existing task is left alone.
	require.Contains(t, listedAutoBackupIDs(t, router, false), task.ID)
}

func TestRestoreAutoBackupOnActiveTaskIsNoOp(t *testing.T) {
	router, session := newPluginStorageRouter(t)
	task := createRecoverableAutoBackup(t, "daily backup", true)

	// Cosy's Recover accepts a row that is not in the trash and leaves it as
	// it is, as it does for ACME users and namespaces.
	recorder := pluginStorageRequest(router, http.MethodPatch, autoBackupPath(task.ID), "{}", session)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())

	require.Contains(t, listedAutoBackupIDs(t, router, false), task.ID)
	var stored model.AutoBackup
	require.NoError(t, model.UseDB().First(&stored, task.ID).Error)
	require.True(t, stored.Enabled)
	require.Nil(t, stored.DeletedAt)
	require.True(t, cron.HasAutoBackupJob(task.ID))
}

func TestRestoreAutoBackupKeepsSecureSessionRules(t *testing.T) {
	newRouter := func(t *testing.T, scopes ...string) http.Handler {
		t.Helper()
		// Reuse the database setup; the router below replaces the admin user
		// with a service token principal.
		newPluginStorageRouter(t)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{
				PublicID: "token",
				Name:     "token",
				Scopes:   scopes,
			})
			c.Next()
		})
		InitAutoBackupRouter(router.Group("/"))
		return router
	}

	t.Run("read scope service token is refused", func(t *testing.T) {
		router := newRouter(t, model.APITokenScopeRead)
		task := createRecoverableAutoBackup(t, "daily backup", false)
		require.NoError(t, model.UseDB().Delete(task).Error)

		recorder := pluginStorageRequest(router, http.MethodPatch, autoBackupPath(task.ID), "{}", "")
		require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedAutoBackupIDs(t, router, false), task.ID)
		require.False(t, cron.HasAutoBackupJob(task.ID))
	})

	t.Run("write scope service token recovers", func(t *testing.T) {
		router := newRouter(t, model.APITokenScopeRead, model.APITokenScopeWrite)
		task := createRecoverableAutoBackup(t, "daily backup", false)
		require.NoError(t, model.UseDB().Delete(task).Error)

		recorder := pluginStorageRequest(router, http.MethodPatch, autoBackupPath(task.ID), "{}", "")
		require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		require.Contains(t, listedAutoBackupIDs(t, router, false), task.ID)
	})

	t.Run("OTP user without a secure session is refused", func(t *testing.T) {
		router, _ := newPluginStorageRouter(t)
		task := createRecoverableAutoBackup(t, "daily backup", false)
		require.NoError(t, model.UseDB().Delete(task).Error)

		recorder := pluginStorageRequest(router, http.MethodPatch, autoBackupPath(task.ID), "{}", "")
		require.NotEqual(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		require.NotContains(t, listedAutoBackupIDs(t, router, false), task.ID)
	})
}
