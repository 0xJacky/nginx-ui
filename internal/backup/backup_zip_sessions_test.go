package backup

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

const (
	zipJWTSecret   = "synthetic-backup-jwt-session"
	zipShortSecret = "synthetic-backup-short-session"
	zipDevSecret   = "synthetic-backup-dev-session"
)

type sessionZipFixture struct {
	source   string
	database string
	live     *sql.DB
}

func setupSessionZipFixture(t *testing.T) sessionZipFixture {
	t.Helper()
	root := t.TempDir()
	oldConf, oldDBName := cosysettings.ConfPath, settings.DatabaseSettings.Name
	oldNginx := *settings.NginxSettings
	oldGranted := settings.BackupSettings.GrantedAccessPath
	cosysettings.ConfPath = filepath.Join(root, "app.ini")
	settings.DatabaseSettings.Name = "session-export"
	settings.NginxSettings.ConfigDir = root
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	settings.NginxSettings.HostMode = ""
	settings.NginxSettings.ContainerName = ""
	settings.BackupSettings.GrantedAccessPath = []string{root}
	t.Cleanup(func() {
		cosysettings.ConfPath, settings.DatabaseSettings.Name = oldConf, oldDBName
		*settings.NginxSettings = oldNginx
		settings.BackupSettings.GrantedAccessPath = oldGranted
	})
	require.NoError(t, os.WriteFile(cosysettings.ConfPath, []byte("[app]\n"), 0o600))
	require.NoError(t, os.WriteFile(settings.NginxSettings.ConfigPath, []byte("events {}\nhttp {}\n"), 0o640))
	path := applicationDatabasePath()
	live, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	live.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, live.Close()) })
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL", "PRAGMA wal_autocheckpoint=0",
		"CREATE TABLE auth_tokens (token TEXT, short_token TEXT)",
		"CREATE TABLE dev_secure_sessions (session_id TEXT)",
		"CREATE TABLE business_records (name TEXT)",
	} {
		_, err := live.Exec(statement)
		require.NoError(t, err)
	}
	_, err = live.Exec("INSERT INTO auth_tokens VALUES (?, ?)", zipJWTSecret, zipShortSecret)
	require.NoError(t, err)
	_, err = live.Exec("INSERT INTO dev_secure_sessions VALUES (?)", zipDevSecret)
	require.NoError(t, err)
	_, err = live.Exec("INSERT INTO business_records VALUES ('committed business row')")
	require.NoError(t, err)
	for _, suffix := range []string{"-wal", "-shm"} {
		_, err := os.Stat(path + suffix)
		require.NoError(t, err, "fixture must have an active SQLite sidecar")
	}
	// A non-hot journal-shaped file must also be excluded, without allowing
	// the backup utility to modify the live database's journaling mode.
	journal := append(make([]byte, 512), []byte(zipJWTSecret)...)
	require.NoError(t, os.WriteFile(path+"-journal", journal, 0o600))
	return sessionZipFixture{source: root, database: path, live: live}
}

func readSessionZip(t *testing.T, data []byte) (map[string][]byte, map[string]os.FileMode) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	files, modes := make(map[string][]byte), make(map[string]os.FileMode)
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		input, err := entry.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(input)
		require.NoError(t, err)
		require.NoError(t, input.Close())
		files[entry.Name], modes[entry.Name] = content, entry.Mode()
	}
	return files, modes
}

func requireSanitizedZipDatabase(t *testing.T, data []byte) {
	t.Helper()
	for _, secret := range []string{zipJWTSecret, zipShortSecret, zipDevSecret} {
		require.NotContains(t, string(data), secret, "removed sessions must not remain in free pages")
	}
	path := filepath.Join(t.TempDir(), "exported.db")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	database, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer database.Close()
	for _, table := range []string{"auth_tokens", "dev_secure_sessions"} {
		var count int
		require.NoError(t, database.QueryRow("SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count)
	}
	var business string
	require.NoError(t, database.QueryRow("SELECT name FROM business_records").Scan(&business))
	require.Equal(t, "committed business row", business, "snapshot must include committed WAL data")
	var integrity string
	require.NoError(t, database.QueryRow("PRAGMA integrity_check").Scan(&integrity))
	require.Equal(t, "ok", integrity)
}

func requireNoSessionSidecars(t *testing.T, files map[string][]byte) {
	t.Helper()
	for _, suffix := range databaseSidecarSuffixes {
		require.NotContains(t, files, "session-export.db"+suffix)
	}
}

func TestBackupZipSanitizesLiveDatabaseForEveryArchiveWriter(t *testing.T) {
	for _, method := range []string{"file", "buffer", "nginx staging", "custom directory"} {
		t.Run(method, func(t *testing.T) {
			fixture := setupSessionZipFixture(t)
			var archive []byte
			switch method {
			case "file":
				path := filepath.Join(t.TempDir(), "output.zip")
				require.NoError(t, createZipArchive(path, fixture.source))
				var err error
				archive, err = os.ReadFile(path)
				require.NoError(t, err)
			case "buffer":
				var output bytes.Buffer
				require.NoError(t, createZipArchiveToBuffer(&output, fixture.source))
				archive = output.Bytes()
			case "nginx staging":
				stage := t.TempDir()
				require.NoError(t, backupNginxFiles(stage))
				var output bytes.Buffer
				require.NoError(t, createZipArchiveToBuffer(&output, stage))
				archive = output.Bytes()
			case "custom directory":
				destination := filepath.Join(fixture.source, "exports")
				require.NoError(t, os.Mkdir(destination, 0o700))
				result, err := createCustomDirectoryBackup(&model.AutoBackup{
					Name: "session-proof", BackupType: model.BackupTypeCustomDir,
					StorageType: model.StorageTypeLocal, StoragePath: destination, BackupPath: fixture.source,
				})
				require.NoError(t, err)
				archive, err = os.ReadFile(result.FilePath)
				require.NoError(t, err)
			}
			files, modes := readSessionZip(t, archive)
			requireSanitizedZipDatabase(t, files["session-export.db"])
			requireNoSessionSidecars(t, files)
			require.Equal(t, []byte("events {}\nhttp {}\n"), files["nginx.conf"])
			if runtime.GOOS != "windows" && method != "nginx staging" {
				require.Equal(t, os.FileMode(0o640), modes["nginx.conf"].Perm())
			}
			var liveSessions int
			require.NoError(t, fixture.live.QueryRow("SELECT count(*) FROM auth_tokens").Scan(&liveSessions))
			require.Equal(t, 1, liveSessions, "export must not revoke live sessions")
		})
	}
}

func TestBackupZipRecognizesHardLinksWithoutFollowingSymlinks(t *testing.T) {
	fixture := setupSessionZipFixture(t)
	if err := os.Link(fixture.database, filepath.Join(fixture.source, "db-alias.bin")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	require.NoError(t, os.Link(fixture.database+"-wal", filepath.Join(fixture.source, "sidecar-alias.bin")))
	if err := os.Symlink(fixture.database, filepath.Join(fixture.source, "db-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	require.NoError(t, os.Symlink(fixture.database+"-wal", filepath.Join(fixture.source, "sidecar-link")))
	require.NoError(t, os.Symlink("nginx.conf", filepath.Join(fixture.source, "ordinary-link")))
	var output bytes.Buffer
	require.NoError(t, createZipArchiveToBuffer(&output, fixture.source))
	files, modes := readSessionZip(t, output.Bytes())
	requireSanitizedZipDatabase(t, files["db-alias.bin"])
	require.Equal(t, files["session-export.db"], files["db-alias.bin"], "one snapshot is reused across inode aliases")
	require.NotContains(t, files, "sidecar-alias.bin")
	require.NotContains(t, files, "sidecar-link")
	require.Equal(t, os.ModeSymlink, modes["db-link"]&os.ModeSymlink)
	require.Equal(t, []byte(fixture.database), files["db-link"])
	require.Equal(t, os.ModeSymlink, modes["ordinary-link"]&os.ModeSymlink)
	require.Equal(t, []byte("nginx.conf"), files["ordinary-link"])
}

func TestBackupZipRejectsDatabaseOutputAliasAndSnapshotFailure(t *testing.T) {
	t.Run("output is a database hard link", func(t *testing.T) {
		fixture := setupSessionZipFixture(t)
		output := filepath.Join(fixture.source, "dangerous-output.zip")
		if err := os.Link(fixture.database, output); err != nil {
			t.Skipf("hard links unavailable: %v", err)
		}
		require.Error(t, createZipArchive(output, fixture.source))
		var count int
		require.NoError(t, fixture.live.QueryRow("SELECT count(*) FROM auth_tokens").Scan(&count))
		require.Equal(t, 1, count)
	})
	t.Run("invalid live database fails closed", func(t *testing.T) {
		root := t.TempDir()
		oldConf, oldName := cosysettings.ConfPath, settings.DatabaseSettings.Name
		cosysettings.ConfPath, settings.DatabaseSettings.Name = filepath.Join(root, "app.ini"), "broken"
		t.Cleanup(func() { cosysettings.ConfPath, settings.DatabaseSettings.Name = oldConf, oldName })
		require.NoError(t, os.WriteFile(applicationDatabasePath(), []byte(zipJWTSecret), 0o600))
		var output bytes.Buffer
		require.Error(t, createZipArchiveToBuffer(&output, root))
		require.NotContains(t, output.String(), zipJWTSecret)
	})
}

func TestBackupExportDoesNotSanitizeUnrelatedDatabaseOrPrivateRollbackCopy(t *testing.T) {
	fixture := setupSessionZipFixture(t)
	unrelatedPath := filepath.Join(fixture.source, "business.db")
	unrelated, err := sql.Open("sqlite3", unrelatedPath)
	require.NoError(t, err)
	_, err = unrelated.Exec("CREATE TABLE auth_tokens (token TEXT)")
	require.NoError(t, err)
	_, err = unrelated.Exec("INSERT INTO auth_tokens VALUES ('unrelated-business-session')")
	require.NoError(t, err)
	require.NoError(t, unrelated.Close())
	var output bytes.Buffer
	require.NoError(t, createZipArchiveToBuffer(&output, fixture.source))
	files, _ := readSessionZip(t, output.Bytes())
	requireSanitizedZipDatabase(t, files["session-export.db"])
	unrelatedBytes, err := os.ReadFile(unrelatedPath)
	require.NoError(t, err)
	require.Equal(t, unrelatedBytes, files["business.db"], "other applications' databases are ordinary business data")

	// The non-exporting copy helper is also used for restore rollback. Its
	// private copies must retain exact state, unlike downloadable backups.
	_, err = fixture.live.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	require.NoError(t, err)
	rawDirectory := t.TempDir()
	require.NoError(t, copyDirectory(fixture.source, rawDirectory))
	original, err := os.ReadFile(fixture.database)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(rawDirectory, filepath.Base(fixture.database)))
	require.NoError(t, err)
	require.Equal(t, original, raw)
	require.Contains(t, string(raw), zipJWTSecret)
	for _, suffix := range databaseSidecarSuffixes {
		_, err := os.Stat(filepath.Join(rawDirectory, filepath.Base(fixture.database)+suffix))
		require.NoError(t, err, "generic restore/rollback copying retains sidecars")
	}
}
