package backup

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoBackupFilePrefixMatchesGeneratedFilenames(t *testing.T) {
	nginx := &model.AutoBackup{Name: "daily backup", BackupType: model.BackupTypeNginxAndNginxUI}
	custom := &model.AutoBackup{Name: "daily backup", BackupType: model.BackupTypeCustomDir}

	assert.Equal(t, "daily_backup_123.zip", fmt.Sprintf("%s%d.zip", autoBackupFilePrefix(nginx), int64(123)))
	assert.Equal(t, "custom_dir_daily_backup_123.zip", fmt.Sprintf("%s%d.zip", autoBackupFilePrefix(custom), int64(123)))
}

func TestExpiredBackupFiles(t *testing.T) {
	nginx := &model.AutoBackup{Name: "daily", BackupType: model.BackupTypeNginxAndNginxUI}
	names := []string{
		"daily_100.zip", "daily_100.zip.key",
		"daily_200.zip", "daily_200.zip.key",
		"daily_300.zip", "daily_300.zip.key",
		"daily_400.zip", "daily_400.zip.key",
		// Files that belong to something else are never expired.
		"daily_500.txt",
		"daily_x.zip",
		"daily_.zip",
		"daily_1_100.zip",
		"nightly_100.zip",
		"custom_dir_daily_100.zip",
		"my_daily_100.zip",
		"daily_600.zip.key.bak",
	}

	tests := []struct {
		name     string
		keep     int
		expected []string
	}{
		{
			name:     "zero keeps everything",
			keep:     0,
			expected: nil,
		},
		{
			name:     "negative keeps everything",
			keep:     -1,
			expected: nil,
		},
		{
			name:     "keep more than exist",
			keep:     10,
			expected: nil,
		},
		{
			name:     "keep exactly all",
			keep:     4,
			expected: nil,
		},
		{
			name:     "keep newest two",
			keep:     2,
			expected: []string{"daily_100.zip", "daily_100.zip.key", "daily_200.zip", "daily_200.zip.key"},
		},
		{
			name:     "keep newest one",
			keep:     1,
			expected: []string{"daily_100.zip", "daily_100.zip.key", "daily_200.zip", "daily_200.zip.key", "daily_300.zip", "daily_300.zip.key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, expiredBackupFiles(names, autoBackupFilePattern(nginx), tt.keep))
		})
	}
}

func TestExpiredBackupFilesOrdersByTimeNotByName(t *testing.T) {
	task := &model.AutoBackup{Name: "daily", BackupType: model.BackupTypeNginxAndNginxUI}
	// Lexical order would put 9 after 10 and 100.
	names := []string{"daily_9.zip", "daily_10.zip", "daily_100.zip"}

	assert.Equal(t, []string{"daily_9.zip"}, expiredBackupFiles(names, autoBackupFilePattern(task), 2))
}

func TestExpiredBackupFilesMatchesCustomDirectoryBackups(t *testing.T) {
	task := &model.AutoBackup{Name: "site files", BackupType: model.BackupTypeCustomDir}
	names := []string{
		"custom_dir_site_files_100.zip",
		"custom_dir_site_files_200.zip",
		"site_files_100.zip",
	}

	assert.Equal(t, []string{"custom_dir_site_files_100.zip"}, expiredBackupFiles(names, autoBackupFilePattern(task), 1))
}

func writeTestFiles(t *testing.T, dir string, names ...string) {
	t.Helper()

	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("data"), 0600))
	}
}

func listTestDir(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	return names
}

func TestPruneOldBackupsLocalKeepsNewestBackups(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir,
		"daily_100.zip", "daily_100.zip.key",
		"daily_200.zip", "daily_200.zip.key",
		"daily_300.zip", "daily_300.zip.key",
		"notes.txt", "nightly_100.zip",
	)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "daily_50.zip"), 0700))

	task := &model.AutoBackup{
		Name:           "daily",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    dir,
		RetentionCount: 2,
	}
	pruneOldBackups(task, &ExecutionResult{
		FilePath: filepath.Join(dir, "daily_300.zip"),
		KeyPath:  filepath.Join(dir, "daily_300.zip.key"),
	})

	assert.Equal(t, []string{
		"daily_200.zip", "daily_200.zip.key",
		"daily_300.zip", "daily_300.zip.key",
		"daily_50.zip", "nightly_100.zip", "notes.txt",
	}, listTestDir(t, dir))
}

func TestPruneOldBackupsLocalKeepsEverythingByDefault(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, "daily_100.zip", "daily_200.zip", "daily_300.zip")

	task := &model.AutoBackup{
		Name:        "daily",
		BackupType:  model.BackupTypeNginxAndNginxUI,
		StorageType: model.StorageTypeLocal,
		StoragePath: dir,
	}
	pruneOldBackups(task, &ExecutionResult{FilePath: filepath.Join(dir, "daily_300.zip")})

	assert.Equal(t, []string{"daily_100.zip", "daily_200.zip", "daily_300.zip"}, listTestDir(t, dir))
}

func TestPruneOldBackupsLocalCustomDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, "custom_dir_site_100.zip", "custom_dir_site_200.zip", "site_100.zip")

	task := &model.AutoBackup{
		Name:           "site",
		BackupType:     model.BackupTypeCustomDir,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    dir,
		RetentionCount: 1,
	}
	pruneOldBackups(task, &ExecutionResult{FilePath: filepath.Join(dir, "custom_dir_site_200.zip")})

	assert.Equal(t, []string{"custom_dir_site_200.zip", "site_100.zip"}, listTestDir(t, dir))
}

func TestPruneOldBackupsLocalNeverDeletesTheNewBackup(t *testing.T) {
	dir := t.TempDir()
	// The host clock went backwards, so the backup that was just written looks
	// older than the existing ones.
	writeTestFiles(t, dir, "daily_100.zip", "daily_200.zip", "daily_300.zip")

	task := &model.AutoBackup{
		Name:           "daily",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    dir,
		RetentionCount: 1,
	}
	pruneOldBackups(task, &ExecutionResult{FilePath: filepath.Join(dir, "daily_100.zip")})

	assert.Equal(t, []string{"daily_100.zip", "daily_300.zip"}, listTestDir(t, dir))
}

func TestPruneOldBackupsLocalMissingStoragePathDoesNotPanic(t *testing.T) {
	task := &model.AutoBackup{
		Name:           "daily",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    filepath.Join(t.TempDir(), "missing"),
		RetentionCount: 1,
	}

	assert.NotPanics(t, func() {
		pruneOldBackups(task, &ExecutionResult{FilePath: "daily_100.zip"})
	})
}

// fakeS3 is a minimal S3 server that supports listing the objects of a bucket
// and deleting a single object, which is all the retention cleanup needs.
type fakeS3 struct {
	mu          sync.Mutex
	bucket      string
	keys        map[string]bool
	deleted     []string
	failDeletes map[string]bool
}

type fakeS3Object struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int    `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type fakeS3Prefix struct {
	Prefix string `xml:"Prefix"`
}

type fakeS3ListResult struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Xmlns          string         `xml:"xmlns,attr"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix"`
	Delimiter      string         `xml:"Delimiter"`
	KeyCount       int            `xml:"KeyCount"`
	MaxKeys        int            `xml:"MaxKeys"`
	IsTruncated    bool           `xml:"IsTruncated"`
	Contents       []fakeS3Object `xml:"Contents"`
	CommonPrefixes []fakeS3Prefix `xml:"CommonPrefixes"`
}

func newFakeS3(t *testing.T, bucket string, keys ...string) (*fakeS3, *httptest.Server) {
	t.Helper()

	fake := &fakeS3{bucket: bucket, keys: make(map[string]bool), failDeletes: make(map[string]bool)}
	for _, key := range keys {
		fake.keys[key] = true
	}

	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	return fake, server
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	objectKey, ok := strings.CutPrefix(r.URL.Path, "/"+f.bucket+"/")
	switch {
	case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
		f.list(w, r.URL.Query().Get("prefix"), r.URL.Query().Get("delimiter"))
	case r.Method == http.MethodDelete && ok && f.failDeletes[objectKey]:
		http.Error(w, "denied", http.StatusForbidden)
	case r.Method == http.MethodDelete && ok && objectKey != "":
		delete(f.keys, objectKey)
		f.deleted = append(f.deleted, objectKey)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unsupported request: "+r.Method+" "+r.URL.String(), http.StatusNotImplemented)
	}
}

func (f *fakeS3) list(w http.ResponseWriter, prefix, delimiter string) {
	result := fakeS3ListResult{
		Xmlns:     "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:      f.bucket,
		Prefix:    prefix,
		Delimiter: delimiter,
		MaxKeys:   1000,
	}

	seenPrefixes := make(map[string]bool)
	for key := range f.keys {
		rest, ok := strings.CutPrefix(key, prefix)
		if !ok {
			continue
		}
		if index := strings.Index(rest, delimiter); delimiter != "" && index >= 0 {
			commonPrefix := prefix + rest[:index+len(delimiter)]
			if !seenPrefixes[commonPrefix] {
				seenPrefixes[commonPrefix] = true
				result.CommonPrefixes = append(result.CommonPrefixes, fakeS3Prefix{Prefix: commonPrefix})
			}
			continue
		}
		result.Contents = append(result.Contents, fakeS3Object{
			Key:          key,
			LastModified: "2026-01-01T00:00:00.000Z",
			ETag:         "\"etag\"",
			Size:         4,
			StorageClass: "STANDARD",
		})
	}
	result.KeyCount = len(result.Contents) + len(result.CommonPrefixes)

	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(result)
}

func (f *fakeS3) remainingKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	keys := make([]string, 0, len(f.keys))
	for key := range f.keys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

func newS3RetentionTask(server *httptest.Server, storagePath string, retention int) *model.AutoBackup {
	return &model.AutoBackup{
		Name:              "daily",
		BackupType:        model.BackupTypeNginxAndNginxUI,
		StorageType:       model.StorageTypeS3,
		StoragePath:       storagePath,
		S3Endpoint:        server.URL,
		S3Bucket:          "test-bucket",
		S3AccessKeyID:     "test-access-key",
		S3SecretAccessKey: "test-secret-key",
		RetentionCount:    retention,
	}
}

func TestPruneOldBackupsS3KeepsNewestBackups(t *testing.T) {
	fake, server := newFakeS3(t, "test-bucket",
		"backups/daily_100.zip", "backups/daily_100.zip.key",
		"backups/daily_200.zip", "backups/daily_200.zip.key",
		"backups/daily_300.zip", "backups/daily_300.zip.key",
		"backups/notes.txt",
		"backups/nightly_100.zip",
		"backups/nested/daily_50.zip",
		"other/daily_10.zip",
		"daily_20.zip",
	)

	task := newS3RetentionTask(server, "/backups/", 2)
	pruneOldBackups(task, &ExecutionResult{
		FilePath: filepath.Join(os.TempDir(), "daily_300.zip"),
		KeyPath:  filepath.Join(os.TempDir(), "daily_300.zip.key"),
	})

	assert.Equal(t, []string{
		"backups/daily_200.zip", "backups/daily_200.zip.key",
		"backups/daily_300.zip", "backups/daily_300.zip.key",
		"backups/nested/daily_50.zip",
		"backups/nightly_100.zip",
		"backups/notes.txt",
		"daily_20.zip",
		"other/daily_10.zip",
	}, fake.remainingKeys())
	assert.ElementsMatch(t, []string{"backups/daily_100.zip", "backups/daily_100.zip.key"}, fake.deleted)
}

func TestPruneOldBackupsS3BucketRoot(t *testing.T) {
	fake, server := newFakeS3(t, "test-bucket",
		"daily_100.zip", "daily_200.zip", "backups/daily_50.zip",
	)

	task := newS3RetentionTask(server, "/", 1)
	pruneOldBackups(task, &ExecutionResult{FilePath: filepath.Join(os.TempDir(), "daily_200.zip")})

	assert.Equal(t, []string{"backups/daily_50.zip", "daily_200.zip"}, fake.remainingKeys())
}

func TestPruneOldBackupsS3KeepsEverythingByDefault(t *testing.T) {
	fake, server := newFakeS3(t, "test-bucket", "backups/daily_100.zip", "backups/daily_200.zip")

	task := newS3RetentionTask(server, "backups", 0)
	pruneOldBackups(task, &ExecutionResult{FilePath: filepath.Join(os.TempDir(), "daily_200.zip")})

	assert.Equal(t, []string{"backups/daily_100.zip", "backups/daily_200.zip"}, fake.remainingKeys())
	assert.Empty(t, fake.deleted)
}

func TestS3ClientPruneBackupsReportsListingFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	task := newS3RetentionTask(server, "backups", 1)
	s3Client, err := NewS3Client(task)
	require.NoError(t, err)

	err = s3Client.PruneBackups(t.Context(), task, &ExecutionResult{FilePath: "daily_200.zip"})
	assert.Error(t, err)
}

func TestS3ClientPruneBackupsContinuesAfterADeleteFailure(t *testing.T) {
	fake, server := newFakeS3(t, "test-bucket",
		"backups/daily_100.zip", "backups/daily_200.zip", "backups/daily_300.zip",
	)
	fake.failDeletes["backups/daily_100.zip"] = true

	task := newS3RetentionTask(server, "backups", 1)
	s3Client, err := NewS3Client(task)
	require.NoError(t, err)

	err = s3Client.PruneBackups(t.Context(), task, &ExecutionResult{FilePath: "daily_300.zip"})
	assert.ErrorContains(t, err, "backups/daily_100.zip")
	assert.Equal(t, []string{"backups/daily_100.zip", "backups/daily_300.zip"}, fake.remainingKeys())
}

func TestShareBackupFiles(t *testing.T) {
	local := func(name string, backupType model.BackupType, storagePath string) *model.AutoBackup {
		return &model.AutoBackup{Name: name, BackupType: backupType, StorageType: model.StorageTypeLocal, StoragePath: storagePath}
	}
	s3 := func(name, bucket, storagePath string) *model.AutoBackup {
		return &model.AutoBackup{
			Name:        name,
			BackupType:  model.BackupTypeNginxAndNginxUI,
			StorageType: model.StorageTypeS3,
			StoragePath: storagePath,
			S3Bucket:    bucket,
		}
	}

	tests := []struct {
		name     string
		a, b     *model.AutoBackup
		expected bool
	}{
		{
			name:     "same name and path",
			a:        local("daily", model.BackupTypeNginxAndNginxUI, "/backups"),
			b:        local("daily", model.BackupTypeNginxAndNginxUI, "/backups/"),
			expected: true,
		},
		{
			name:     "names that sanitize to the same file name",
			a:        local("web prod", model.BackupTypeNginxAndNginxUI, "/backups"),
			b:        local("web/prod", model.BackupTypeNginxAndNginxUI, "/backups"),
			expected: true,
		},
		{
			name:     "custom directory task and a task named after its prefix",
			a:        local("web", model.BackupTypeCustomDir, "/backups"),
			b:        local("custom_dir_web", model.BackupTypeNginxAndNginxUI, "/backups"),
			expected: true,
		},
		{
			name:     "same name in different paths",
			a:        local("daily", model.BackupTypeNginxAndNginxUI, "/backups/a"),
			b:        local("daily", model.BackupTypeNginxAndNginxUI, "/backups/b"),
			expected: false,
		},
		{
			name:     "same name of different backup types",
			a:        local("daily", model.BackupTypeNginxAndNginxUI, "/backups"),
			b:        local("daily", model.BackupTypeCustomDir, "/backups"),
			expected: false,
		},
		{
			name:     "different names",
			a:        local("daily", model.BackupTypeNginxAndNginxUI, "/backups"),
			b:        local("daily_1", model.BackupTypeNginxAndNginxUI, "/backups"),
			expected: false,
		},
		{
			name:     "local and S3",
			a:        local("daily", model.BackupTypeNginxAndNginxUI, "backups"),
			b:        s3("daily", "bucket", "backups"),
			expected: false,
		},
		{
			name:     "same S3 bucket and path",
			a:        s3("daily", "bucket", "/backups/"),
			b:        s3("daily", "bucket", "backups"),
			expected: true,
		},
		{
			name:     "different S3 buckets",
			a:        s3("daily", "bucket-a", "backups"),
			b:        s3("daily", "bucket-b", "backups"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, shareBackupFiles(tt.a, tt.b))
			assert.Equal(t, tt.expected, shareBackupFiles(tt.b, tt.a))
		})
	}
}

func useRetentionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AutoBackup{}, &model.Notification{}, &model.ExternalNotify{}))

	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	return db
}

func TestApplyRetentionPolicySkipsWhenAnotherTaskSharesFiles(t *testing.T) {
	db := useRetentionTestDB(t)
	dir := t.TempDir()
	writeTestFiles(t, dir, "web_prod_100.zip", "web_prod_200.zip", "web_prod_300.zip")

	task := &model.AutoBackup{
		Name:           "web prod",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    dir,
		CronExpression: "0 0 * * *",
		RetentionCount: 1,
	}
	other := &model.AutoBackup{
		Name:           "web/prod",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    dir,
		CronExpression: "0 0 * * *",
	}
	require.NoError(t, db.Create(task).Error)
	require.NoError(t, db.Create(other).Error)

	applyRetentionPolicy(task, &ExecutionResult{FilePath: filepath.Join(dir, "web_prod_300.zip")})

	assert.Equal(t, []string{"web_prod_100.zip", "web_prod_200.zip", "web_prod_300.zip"}, listTestDir(t, dir))
}

func TestApplyRetentionPolicyPrunesWhenNoOtherTaskSharesFiles(t *testing.T) {
	db := useRetentionTestDB(t)
	dir := t.TempDir()
	writeTestFiles(t, dir, "daily_100.zip", "daily_200.zip", "daily_300.zip", "weekly_100.zip")

	task := &model.AutoBackup{
		Name:           "daily",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    dir,
		CronExpression: "0 0 * * *",
		RetentionCount: 1,
	}
	other := &model.AutoBackup{
		Name:           "weekly",
		BackupType:     model.BackupTypeNginxAndNginxUI,
		StorageType:    model.StorageTypeLocal,
		StoragePath:    dir,
		CronExpression: "0 0 * * 0",
	}
	require.NoError(t, db.Create(task).Error)
	require.NoError(t, db.Create(other).Error)

	applyRetentionPolicy(task, &ExecutionResult{FilePath: filepath.Join(dir, "daily_300.zip")})

	assert.Equal(t, []string{"daily_300.zip", "weekly_100.zip"}, listTestDir(t, dir))
}
