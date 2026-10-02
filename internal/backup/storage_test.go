package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const memoryStorageType = "plugin:memory"

// memoryStorage is a StorageSource that keeps objects in memory and reads
// and writes the files it is handed, as the plugin adapter does.
type memoryStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
	deleted []string
}

func (m *memoryStorage) StorageBackends() []StorageBackend {
	return []StorageBackend{{Type: memoryStorageType, Name: "Memory", PluginID: "io.github.example.memory", Fields: []StorageField{
		{Key: "bucket", DisplayName: "Bucket", Required: true},
		{Key: "token", DisplayName: "Token", Secret: true},
	}}}
}

func (m *memoryStorage) Validate(_ context.Context, _ string, config map[string]string) error {
	if config["bucket"] == "invalid" {
		return cosy.WrapErrorWithParams(plugin.ErrStorageConfigInvalid, "bucket", "no such bucket")
	}
	return nil
}

func (m *memoryStorage) Put(_ context.Context, _ string, _ map[string]string, key, sourcePath string) (int64, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = data
	return int64(len(data)), nil
}

func (m *memoryStorage) Get(_ context.Context, _ string, _ map[string]string, key, targetPath string) (int64, error) {
	m.mu.Lock()
	data, ok := m.objects[key]
	m.mu.Unlock()
	if !ok {
		return 0, fmt.Errorf("no object %s", key)
	}
	return int64(len(data)), os.WriteFile(targetPath, data, 0o600)
}

func (m *memoryStorage) List(_ context.Context, _ string, _ map[string]string, prefix string) ([]StoredObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var objects []StoredObject
	for key, data := range m.objects {
		if strings.HasPrefix(key, prefix) {
			objects = append(objects, StoredObject{Key: key, Size: int64(len(data))})
		}
	}
	return objects, nil
}

func (m *memoryStorage) Delete(_ context.Context, _ string, _ map[string]string, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	m.deleted = append(m.deleted, key)
	return nil
}

func (m *memoryStorage) reset(objects map[string][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects = objects
	m.deleted = nil
}

func (m *memoryStorage) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.objects))
	for key := range m.objects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var registerMemoryStorage = sync.OnceValue(func() *memoryStorage {
	storage := &memoryStorage{objects: map[string][]byte{}}
	RegisterStorageSource(storage)
	return storage
})

func pluginTask(backupType model.BackupType) *model.AutoBackup {
	return &model.AutoBackup{
		Name:           "daily",
		BackupType:     backupType,
		StorageType:    memoryStorageType,
		StoragePath:    "/nginx-ui/",
		StorageConfig:  map[string]string{"bucket": "backups"},
		CronExpression: "0 0 * * *",
	}
}

func TestStorageBackendsListTheBuiltinsFirst(t *testing.T) {
	registerMemoryStorage()

	backends := StorageBackends()
	require.GreaterOrEqual(t, len(backends), 3)
	assert.Equal(t, "local", backends[0].Type)
	assert.True(t, backends[0].BuiltIn)
	assert.Equal(t, "s3", backends[1].Type)
	var found bool
	for _, backend := range backends[2:] {
		if backend.Type == memoryStorageType {
			found = true
			assert.False(t, backend.BuiltIn)
			assert.Len(t, backend.Fields, 2)
		}
	}
	assert.True(t, found)
}

func TestValidatePluginStorage(t *testing.T) {
	registerMemoryStorage()
	ctx := t.Context()

	assert.NoError(t, ValidatePluginStorage(ctx, memoryStorageType, "nginx-ui/daily", map[string]string{"bucket": "b"}))
	assert.NoError(t, ValidatePluginStorage(ctx, memoryStorageType, "/", map[string]string{"bucket": "b"}))

	for _, tc := range []struct {
		name        string
		storageType model.StorageType
		path        string
		config      map[string]string
		want        error
	}{
		{"missing required field", memoryStorageType, "x", map[string]string{"token": "t"}, plugin.ErrStorageConfigInvalid},
		{"rejected by the plugin", memoryStorageType, "x", map[string]string{"bucket": "invalid"}, plugin.ErrStorageConfigInvalid},
		{"backend gone", "plugin:gone", "x", map[string]string{"bucket": "b"}, plugin.ErrStorageBackendUnavailable},
		{"malformed type", "Plugin Gone", "x", nil, ErrAutoBackupUnsupportedType},
		{"traversal in prefix", memoryStorageType, "a/../b", map[string]string{"bucket": "b"}, ErrInvalidStorageKeyPrefix},
		{"empty segment", memoryStorageType, "a//b", map[string]string{"bucket": "b"}, ErrInvalidStorageKeyPrefix},
		{"backslash", memoryStorageType, `a\b`, map[string]string{"bucket": "b"}, ErrInvalidStorageKeyPrefix},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePluginStorage(ctx, tc.storageType, tc.path, tc.config)
			assertErrorCode(t, err, tc.want)
		})
	}

	// The auto backup validation accepts plugin types and rejects others.
	assert.NoError(t, ValidateAutoBackupConfig(&model.AutoBackup{StorageType: memoryStorageType}))
	assert.Error(t, ValidateAutoBackupConfig(&model.AutoBackup{StorageType: "not a type"}))
	assert.Error(t, ValidateAutoBackupConfig(&model.AutoBackup{StorageType: memoryStorageType, RetentionCount: -1}))
}

func assertErrorCode(t *testing.T, err, want error) {
	t.Helper()
	got, ok := errors.AsType[*cosy.Error](err)
	require.True(t, ok, "err = %v, want a cosy error", err)
	wantErr, _ := errors.AsType[*cosy.Error](want)
	assert.Equal(t, wantErr.Code, got.Code, "err = %v", err)
}

func TestHandleSourceStorageStoresTheRunAndAppliesRetention(t *testing.T) {
	storage := registerMemoryStorage()
	storage.reset(map[string][]byte{
		"nginx-ui/daily_1000.zip":     []byte("old"),
		"nginx-ui/daily_1000.zip.key": []byte("old key"),
		"nginx-ui/daily_2000.zip":     []byte("older"),
		"nginx-ui/daily_2000.zip.key": []byte("older key"),
		// Another task sharing the prefix, and a stray file, are left alone.
		"nginx-ui/daily_extra_1.zip": []byte("other task"),
		"nginx-ui/daily_notes.txt":   []byte("notes"),
		"nginx-ui/daily_500.zip.key": []byte("orphan key"),
	})

	dir := t.TempDir()
	archive := filepath.Join(dir, "daily_3000.zip")
	key := archive + ".key"
	require.NoError(t, os.WriteFile(archive, []byte("archive"), 0o600))
	require.NoError(t, os.WriteFile(key, []byte("a2V5:aXY="), 0o600))

	task := pluginTask(model.BackupTypeNginxAndNginxUI)
	task.RetentionCount = 2
	require.NoError(t, handleSourceStorage(t.Context(), task, &ExecutionResult{FilePath: archive, KeyPath: key}))

	assert.Equal(t, []string{
		"nginx-ui/daily_2000.zip",
		"nginx-ui/daily_2000.zip.key",
		"nginx-ui/daily_3000.zip",
		"nginx-ui/daily_3000.zip.key",
		"nginx-ui/daily_extra_1.zip",
		"nginx-ui/daily_notes.txt",
	}, storage.keys())
	for _, path := range []string{archive, key} {
		_, err := os.Stat(path)
		assert.True(t, errors.Is(err, os.ErrNotExist), "local copy %s was not removed", path)
	}

	runs, err := ListStoredBackups(t.Context(), task)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, "nginx-ui/daily_3000.zip", runs[0].Key)
	assert.Equal(t, "nginx-ui/daily_3000.zip.key", runs[0].KeyFile)
	assert.Equal(t, int64(7), runs[0].Size)
	assert.Equal(t, time.Unix(3000, 0).UTC(), runs[0].CreatedAt)

	require.NoError(t, DeleteStoredBackup(t.Context(), task, "nginx-ui/daily_2000.zip"))
	assert.NotContains(t, storage.keys(), "nginx-ui/daily_2000.zip.key")
	assertErrorCode(t, DeleteStoredBackup(t.Context(), task, "nginx-ui/daily_extra_1.zip"), ErrStoredBackupNotFound)
	assertErrorCode(t, DeleteStoredBackup(t.Context(), task, "nginx-ui/daily_3000.zip.key"), ErrStoredBackupNotFound)
}

func TestRestoreStoredBackupFetchesTheArchiveAndItsKey(t *testing.T) {
	storage := registerMemoryStorage()
	storage.reset(map[string][]byte{
		"nginx-ui/daily_3000.zip":     []byte("not a zip archive"),
		"nginx-ui/daily_3000.zip.key": []byte("a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2U=:aXZpdml2aXZpdml2aXZpdg=="),
		"nginx-ui/daily_4000.zip":     []byte("no key file"),
	})
	task := pluginTask(model.BackupTypeNginxAndNginxUI)

	// The archive and the key were fetched, the restore itself rejects the
	// archive.
	_, err := RestoreStoredBackup(t.Context(), task, "nginx-ui/daily_3000.zip", RestoreStoredOptions{})
	assertErrorCode(t, err, ErrExtractArchive)

	_, err = RestoreStoredBackup(t.Context(), task, "nginx-ui/daily_4000.zip", RestoreStoredOptions{})
	assertErrorCode(t, err, ErrBackupFileNotFound)
	_, err = RestoreStoredBackup(t.Context(), task, "elsewhere/daily_3000.zip", RestoreStoredOptions{})
	assertErrorCode(t, err, ErrStoredBackupNotFound)

	custom := pluginTask(model.BackupTypeCustomDir)
	_, err = RestoreStoredBackup(t.Context(), custom, "nginx-ui/custom_dir_daily_3000.zip", RestoreStoredOptions{})
	assert.ErrorIs(t, err, ErrStoredBackupNotRestorable)

	local := pluginTask(model.BackupTypeNginxAndNginxUI)
	local.StorageType = model.StorageTypeLocal
	_, err = ListStoredBackups(t.Context(), local)
	assert.ErrorIs(t, err, ErrPluginStorageRequired)
}

func TestReadKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.key")
	require.NoError(t, os.WriteFile(path, []byte("a2V5:aXY=\n"), 0o600))
	key, iv, err := readKeyFile(path)
	require.NoError(t, err)
	assert.Equal(t, "key", string(key))
	assert.Equal(t, "iv", string(iv))

	require.NoError(t, os.WriteFile(path, []byte("no separator"), 0o600))
	_, _, err = readKeyFile(path)
	assert.ErrorIs(t, err, ErrInvalidSecurityToken)
}

func TestExecuteAutoBackupStoresThroughAPluginBackend(t *testing.T) {
	storage := registerMemoryStorage()
	storage.reset(map[string][]byte{})

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AutoBackup{}, &model.Notification{}, &model.ExternalNotify{}))
	originalDB := model.UseDB()
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() {
		model.Use(originalDB)
		if originalDB != nil {
			query.SetDefault(originalDB)
		}
	})

	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "site.conf"), []byte("server {}"), 0o600))
	originalGrantedAccessPaths := settings.BackupSettings.GrantedAccessPath
	settings.BackupSettings.GrantedAccessPath = []string{rootDir}
	t.Cleanup(func() { settings.BackupSettings.GrantedAccessPath = originalGrantedAccessPaths })

	task := pluginTask(model.BackupTypeCustomDir)
	task.BackupPath = sourceDir
	require.NoError(t, db.Create(task).Error)

	require.NoError(t, ExecuteAutoBackup(task))

	keys := storage.keys()
	require.Len(t, keys, 1)
	assert.True(t, strings.HasPrefix(keys[0], "nginx-ui/custom_dir_daily_"), keys[0])
	assert.True(t, strings.HasSuffix(keys[0], ".zip"), keys[0])

	var stored model.AutoBackup
	require.NoError(t, db.First(&stored, task.ID).Error)
	assert.Equal(t, model.BackupStatusSuccess, stored.LastBackupStatus)
	assert.Equal(t, map[string]string{"bucket": "backups"}, stored.StorageConfig, "the values survive the encrypted column")

	// A backend whose plugin is gone fails the run.
	gone := pluginTask(model.BackupTypeCustomDir)
	gone.Name = "gone"
	gone.BackupPath = sourceDir
	gone.StorageType = "plugin:gone"
	require.NoError(t, db.Create(gone).Error)
	err = ExecuteAutoBackup(gone)
	assertErrorCode(t, err, plugin.ErrStorageBackendUnavailable)
	var failed model.AutoBackup
	require.NoError(t, db.First(&failed, gone.ID).Error)
	assert.Equal(t, model.BackupStatusFailed, failed.LastBackupStatus)
}
