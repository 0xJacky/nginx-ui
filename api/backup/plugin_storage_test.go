package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	internalbackup "github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
)

const apiStorageType = "plugin:api-store"

// apiStorage serves apiStorageType from memory.
type apiStorage struct {
	mu        sync.Mutex
	objects   map[string][]byte
	validated int
}

func (s *apiStorage) StorageBackends() []internalbackup.StorageBackend {
	return []internalbackup.StorageBackend{{Type: apiStorageType, Name: "API store", Fields: []internalbackup.StorageField{
		{Key: "bucket", DisplayName: "Bucket", Required: true},
	}}}
}

func (s *apiStorage) Validate(_ context.Context, _ string, config map[string]string) error {
	s.mu.Lock()
	s.validated++
	s.mu.Unlock()
	if config["bucket"] == "invalid" {
		return cosy.WrapErrorWithParams(plugin.ErrStorageConfigInvalid, "bucket", "no such bucket")
	}
	return nil
}

func (s *apiStorage) Put(_ context.Context, _ string, _ map[string]string, key, sourcePath string) (int64, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	return int64(len(data)), nil
}

func (s *apiStorage) Get(_ context.Context, _ string, _ map[string]string, key, targetPath string) (int64, error) {
	s.mu.Lock()
	data, ok := s.objects[key]
	s.mu.Unlock()
	if !ok {
		return 0, fmt.Errorf("no object %s", key)
	}
	return int64(len(data)), os.WriteFile(targetPath, data, 0o600)
}

func (s *apiStorage) List(_ context.Context, _ string, _ map[string]string, prefix string) ([]internalbackup.StoredObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var objects []internalbackup.StoredObject
	for key, data := range s.objects {
		if strings.HasPrefix(key, prefix) {
			objects = append(objects, internalbackup.StoredObject{Key: key, Size: int64(len(data))})
		}
	}
	return objects, nil
}

func (s *apiStorage) Delete(_ context.Context, _ string, _ map[string]string, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

func (s *apiStorage) validations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.validated
}

var registerAPIStorage = sync.OnceValue(func() *apiStorage {
	storage := &apiStorage{objects: map[string][]byte{}}
	internalbackup.RegisterStorageSource(storage)
	return storage
})

func newPluginStorageRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.AutoBackup{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "auto-backup.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)

	admin := &model.User{Model: model.Model{ID: 301}, Name: "admin", Status: true, OTPSecret: []byte("enabled")}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", admin)
		c.Next()
	})
	InitAutoBackupRouter(router.Group("/"))
	return router, internaluser.SetSecureSessionID(admin.ID)
}

func pluginStorageRequest(router http.Handler, method, path, body, session string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		req.Header.Set("X-Secure-Session-ID", session)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestPluginStorageBackendsAreOfferedAndValidated(t *testing.T) {
	storage := registerAPIStorage()
	router, session := newPluginStorageRouter(t)

	recorder := pluginStorageRequest(router, http.MethodGet, "/auto_backup/storage_backends", "", "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var backends struct {
		Data []internalbackup.StorageBackend `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &backends))
	require.Equal(t, "local", backends.Data[0].Type)
	require.Equal(t, "s3", backends.Data[1].Type)
	require.Contains(t, recorder.Body.String(), apiStorageType)

	task := func(config string) string {
		return `{"name":"daily","backup_type":"nginx_and_nginx_ui","storage_type":"` + apiStorageType +
			`","storage_path":"nginx-ui","cron_expression":"0 0 * * *","enabled":false,"retention_count":3,"storage_config":` + config + `}`
	}

	// A required field left empty, or a value the plugin rejects, is not stored.
	before := storage.validations()
	recorder = pluginStorageRequest(router, http.MethodPost, "/auto_backup", task(`{}`), session)
	require.Contains(t, recorder.Body.String(), `"code":55202`)
	require.Equal(t, before, storage.validations())
	recorder = pluginStorageRequest(router, http.MethodPost, "/auto_backup", task(`{"bucket":"invalid"}`), session)
	require.Contains(t, recorder.Body.String(), `"code":55202`)
	count, err := query.AutoBackup.Count()
	require.NoError(t, err)
	require.Zero(t, count)

	recorder = pluginStorageRequest(router, http.MethodPost, "/auto_backup", task(`{"bucket":"backups"}`), session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created model.AutoBackup
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &created))
	require.Equal(t, 3, created.RetentionCount)
	require.Equal(t, "backups", created.StorageConfig["bucket"])

	var raw string
	require.NoError(t, model.UseDB().Raw("SELECT storage_config FROM auto_backups WHERE id = ?", created.ID).Scan(&raw).Error)
	require.NotContains(t, raw, "backups", "the values are encrypted at rest")

	// Renaming the task leaves the storage alone and is not validated.
	validated := storage.validations()
	path := "/auto_backup/" + strconv.FormatUint(created.ID, 10)
	recorder = pluginStorageRequest(router, http.MethodPost, path, `{"name":"nightly"}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, validated, storage.validations())

	// Changing the values is checked against the stored type.
	recorder = pluginStorageRequest(router, http.MethodPost, path, `{"storage_config":{"bucket":"invalid"}}`, session)
	require.Contains(t, recorder.Body.String(), `"code":55202`)

	recorder = pluginStorageRequest(router, http.MethodPost, "/auto_backup/test_storage",
		`{"name":"daily","storage_type":"`+apiStorageType+`","storage_path":"nginx-ui","storage_config":{"bucket":"backups"}}`, session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	recorder = pluginStorageRequest(router, http.MethodPost, "/auto_backup/test_storage",
		`{"name":"daily","storage_type":"local","storage_path":"/tmp"}`, session)
	require.NotEqual(t, http.StatusOK, recorder.Code)
}

func TestStoredBackupsCanBeListedAndDeleted(t *testing.T) {
	storage := registerAPIStorage()
	router, session := newPluginStorageRouter(t)

	stored := &model.AutoBackup{
		Name:           "weekly",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    apiStorageType,
		StoragePath:    "remote",
		StorageConfig:  map[string]string{"bucket": "backups"},
		CronExpression: "0 0 * * 0",
	}
	require.NoError(t, query.AutoBackup.Create(stored))
	storage.mu.Lock()
	storage.objects["remote/weekly_100.zip"] = []byte("archive")
	storage.objects["remote/weekly_100.zip.key"] = []byte("key")
	storage.objects["remote/weekly_200.zip"] = []byte("newer")
	storage.mu.Unlock()

	base := "/auto_backup/" + strconv.FormatUint(stored.ID, 10) + "/stored"
	recorder := pluginStorageRequest(router, http.MethodGet, base, "", "")
	require.NotEqual(t, http.StatusOK, recorder.Code, "listing needs a secure session")

	recorder = pluginStorageRequest(router, http.MethodGet, base, "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var runs struct {
		Data []internalbackup.StoredBackup `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &runs))
	require.Len(t, runs.Data, 2)
	require.Equal(t, "remote/weekly_200.zip", runs.Data[0].Key)
	require.Equal(t, "remote/weekly_100.zip.key", runs.Data[1].KeyFile)

	recorder = pluginStorageRequest(router, http.MethodDelete, base+"?key="+url.QueryEscape("remote/weekly_100.zip"), "", session)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	storage.mu.Lock()
	_, archiveLeft := storage.objects["remote/weekly_100.zip"]
	_, keyLeft := storage.objects["remote/weekly_100.zip.key"]
	storage.mu.Unlock()
	require.False(t, archiveLeft)
	require.False(t, keyLeft)

	recorder = pluginStorageRequest(router, http.MethodDelete, base+"?key="+url.QueryEscape("../etc/passwd"), "", session)
	require.Contains(t, recorder.Body.String(), `"code":4918`)

	// A run without a key file cannot be restored.
	recorder = pluginStorageRequest(router, http.MethodPost, base+"/restore",
		`{"key":"remote/weekly_200.zip","restore_nginx":false,"restore_nginx_ui":false}`, session)
	require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"code":4511`)
}
