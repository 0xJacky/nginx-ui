package backup

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunAutoBackupExecutesDisabledConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AutoBackup{}, &model.Notification{}, &model.ExternalNotify{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "source")
	storageDir := filepath.Join(rootDir, "storage")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "nginx.conf"), []byte("events {}"), 0o600))

	originalGrantedAccessPaths := settings.BackupSettings.GrantedAccessPath
	settings.BackupSettings.GrantedAccessPath = []string{rootDir}
	t.Cleanup(func() {
		settings.BackupSettings.GrantedAccessPath = originalGrantedAccessPaths
	})

	autoBackup := &model.AutoBackup{
		Name:           "daily backup",
		BackupType:     model.BackupTypeCustomDir,
		StorageType:    model.StorageTypeLocal,
		BackupPath:     sourceDir,
		StoragePath:    storageDir,
		CronExpression: "0 0 * * *",
		Enabled:        true,
	}
	require.NoError(t, db.Create(autoBackup).Error)
	require.NoError(t, db.Model(autoBackup).Update("enabled", false).Error)

	router := gin.New()
	router.POST("/auto_backup/:id/run", RunAutoBackup)

	req := httptest.NewRequest(http.MethodPost, "/auto_backup/"+strconv.FormatUint(autoBackup.ID, 10)+"/run", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var updated model.AutoBackup
	require.NoError(t, db.First(&updated, autoBackup.ID).Error)
	require.False(t, updated.Enabled)
	require.Equal(t, model.BackupStatusSuccess, updated.LastBackupStatus)
	require.NotNil(t, updated.LastBackupTime)
	require.Empty(t, updated.LastBackupError)

	backupFiles, err := filepath.Glob(filepath.Join(storageDir, "custom_dir_daily_backup_*.zip"))
	require.NoError(t, err)
	require.Len(t, backupFiles, 1)
}

func TestRunAutoBackupAppliesRetentionCount(t *testing.T) {
	tests := []struct {
		name           string
		retentionCount int
		expectedOld    []string
	}{
		{
			name:           "keeps all backups by default",
			retentionCount: 0,
			expectedOld:    []string{"custom_dir_daily_backup_100.zip", "custom_dir_daily_backup_200.zip", "custom_dir_daily_backup_300.zip"},
		},
		{
			name:           "deletes the oldest backups",
			retentionCount: 2,
			expectedOld:    []string{"custom_dir_daily_backup_300.zip"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&model.AutoBackup{}, &model.Notification{}, &model.ExternalNotify{}))

			model.Use(db)
			query.Use(db)
			query.SetDefault(db)

			rootDir := t.TempDir()
			sourceDir := filepath.Join(rootDir, "source")
			storageDir := filepath.Join(rootDir, "storage")
			require.NoError(t, os.MkdirAll(sourceDir, 0o755))
			require.NoError(t, os.MkdirAll(storageDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "nginx.conf"), []byte("events {}"), 0o600))
			for _, name := range []string{"custom_dir_daily_backup_100.zip", "custom_dir_daily_backup_200.zip", "custom_dir_daily_backup_300.zip", "unrelated.txt"} {
				require.NoError(t, os.WriteFile(filepath.Join(storageDir, name), []byte("old"), 0o600))
			}

			originalGrantedAccessPaths := settings.BackupSettings.GrantedAccessPath
			settings.BackupSettings.GrantedAccessPath = []string{rootDir}
			t.Cleanup(func() {
				settings.BackupSettings.GrantedAccessPath = originalGrantedAccessPaths
			})

			autoBackup := &model.AutoBackup{
				Name:           "daily backup",
				BackupType:     model.BackupTypeCustomDir,
				StorageType:    model.StorageTypeLocal,
				BackupPath:     sourceDir,
				StoragePath:    storageDir,
				CronExpression: "0 0 * * *",
				Enabled:        true,
				RetentionCount: tt.retentionCount,
			}
			require.NoError(t, db.Create(autoBackup).Error)

			router := gin.New()
			router.POST("/auto_backup/:id/run", RunAutoBackup)

			req := httptest.NewRequest(http.MethodPost, "/auto_backup/"+strconv.FormatUint(autoBackup.ID, 10)+"/run", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

			var updated model.AutoBackup
			require.NoError(t, db.First(&updated, autoBackup.ID).Error)
			require.Equal(t, model.BackupStatusSuccess, updated.LastBackupStatus)
			require.Equal(t, tt.retentionCount, updated.RetentionCount)

			backupFiles, err := filepath.Glob(filepath.Join(storageDir, "custom_dir_daily_backup_*.zip"))
			require.NoError(t, err)
			names := make([]string, 0, len(backupFiles))
			for _, file := range backupFiles {
				names = append(names, filepath.Base(file))
			}
			// The backup that was just created is always kept, next to the retained old ones.
			require.Len(t, names, len(tt.expectedOld)+1, names)
			require.Subset(t, names, tt.expectedOld)
			require.FileExists(t, filepath.Join(storageDir, "unrelated.txt"))
		})
	}
}
