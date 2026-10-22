package backup

import (
	"bytes"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

func openSnapshotTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	absolute, err := filepath.Abs(path)
	require.NoError(t, err)
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	parameters := uri.Query()
	parameters.Set("_busy_timeout", "5000")
	uri.RawQuery = parameters.Encode()
	database, err := sql.Open("sqlite3", uri.String())
	require.NoError(t, err)
	database.SetMaxOpenConns(1)
	require.NoError(t, database.Ping())
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	return database
}

func execSnapshotTest(t *testing.T, database *sql.DB, statement string, args ...any) {
	t.Helper()
	_, err := database.Exec(statement, args...)
	require.NoError(t, err)
}

func TestSnapshotBackupDatabasePreservesWALBusinessAndRemovesSessionBytes(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "quoted'?#")
	require.NoError(t, os.MkdirAll(directory, 0700))
	source := filepath.Join(directory, "source.db")
	destination := filepath.Join(directory, "snapshot'?.db")
	database := openSnapshotTestDB(t, source)
	execSnapshotTest(t, database, "PRAGMA secure_delete=OFF")
	execSnapshotTest(t, database, "CREATE TABLE business_records (id INTEGER PRIMARY KEY, value TEXT)")
	execSnapshotTest(t, database, "CREATE TABLE auth_tokens (id INTEGER PRIMARY KEY, token TEXT, short_token TEXT)")
	execSnapshotTest(t, database, "CREATE TABLE dev_secure_sessions (id INTEGER PRIMARY KEY, session_id TEXT)")
	deletedMarker := "deleted-jwt-remnant-0123456789"
	execSnapshotTest(t, database, "INSERT INTO auth_tokens(token,short_token) VALUES (?,?)", strings.Repeat(deletedMarker, 200), "deleted-short")
	execSnapshotTest(t, database, "DELETE FROM auth_tokens")
	raw, err := os.ReadFile(source)
	require.NoError(t, err)
	require.True(t, bytes.Contains(raw, []byte(deletedMarker)), "fixture must retain deleted JWT bytes before snapshot")
	var journalMode string
	require.NoError(t, database.QueryRow("PRAGMA journal_mode=WAL").Scan(&journalMode))
	require.Equal(t, "wal", journalMode)
	execSnapshotTest(t, database, "PRAGMA wal_autocheckpoint=0")
	businessMarker := "business-committed-only-in-wal"
	jwtMarker := "active-jwt-marker-0123456789"
	shortMarker := "active-short-marker-0123456789"
	devMarker := "active-dev-session-marker-0123456789"
	execSnapshotTest(t, database, "INSERT INTO business_records(value) VALUES (?)", businessMarker)
	execSnapshotTest(t, database, "INSERT INTO auth_tokens(token,short_token) VALUES (?,?)", jwtMarker, shortMarker)
	execSnapshotTest(t, database, "INSERT INTO dev_secure_sessions(session_id) VALUES (?)", devMarker)
	sourceBefore, err := os.ReadFile(source)
	require.NoError(t, err)
	walBefore, err := os.ReadFile(source + "-wal")
	require.NoError(t, err)
	require.NotContains(t, string(sourceBefore), businessMarker, "business record must still be WAL-only")
	require.Contains(t, string(walBefore), businessMarker)
	require.NoError(t, snapshotBackupDatabase(source, destination))
	copied := openSnapshotTestDB(t, destination)
	var business string
	require.NoError(t, copied.QueryRow("SELECT value FROM business_records").Scan(&business))
	require.Equal(t, businessMarker, business)
	for _, table := range []string{"auth_tokens", "dev_secure_sessions"} {
		var count int
		require.NoError(t, copied.QueryRow("SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count)
		require.NoError(t, database.QueryRow("SELECT count(*) FROM "+table).Scan(&count))
		require.Equal(t, 1, count, "source session rows must remain")
	}
	exported, err := os.ReadFile(destination)
	require.NoError(t, err)
	for _, marker := range []string{deletedMarker, jwtMarker, shortMarker, devMarker} {
		require.NotContains(t, string(exported), marker)
	}
	sourceAfter, err := os.ReadFile(source)
	require.NoError(t, err)
	walAfter, err := os.ReadFile(source + "-wal")
	require.NoError(t, err)
	require.Equal(t, sourceBefore, sourceAfter, "snapshot must not write/checkpoint source database")
	require.Equal(t, walBefore, walAfter, "snapshot must not alter source WAL")
	require.Contains(t, string(sourceAfter), deletedMarker, "source deleted bytes must remain untouched")
	for _, sidecar := range snapshotSidecars(destination) {
		_, err = os.Lstat(sidecar)
		require.True(t, os.IsNotExist(err), sidecar)
	}
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), ".session-snapshot-"), entry.Name())
	}
}

func TestSnapshotBackupDatabaseFailsClosedAndCleansTemporaryFiles(t *testing.T) {
	for _, mode := range []string{"invalid source", "missing source", "delete rejected", "existing destination", "existing destination sidecar"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			source := filepath.Join(directory, "source.db")
			destination := filepath.Join(directory, "result.db")
			switch mode {
			case "invalid source":
				require.NoError(t, os.WriteFile(source, []byte("not a sqlite database"), 0600))
			case "delete rejected":
				database := openSnapshotTestDB(t, source)
				execSnapshotTest(t, database, "CREATE TABLE auth_tokens (token TEXT)")
				execSnapshotTest(t, database, "INSERT INTO auth_tokens VALUES ('synthetic-active-session')")
				execSnapshotTest(t, database, "CREATE TRIGGER reject_session_delete BEFORE DELETE ON auth_tokens BEGIN SELECT RAISE(ABORT,'reject snapshot deletion'); END")
			case "existing destination":
				require.NoError(t, os.WriteFile(destination, []byte("keep existing destination"), 0600))
			case "existing destination sidecar":
				require.NoError(t, os.WriteFile(destination+"-wal", []byte("keep existing sidecar"), 0600))
			}
			require.Error(t, snapshotBackupDatabase(source, destination))
			entries, err := os.ReadDir(directory)
			require.NoError(t, err)
			for _, entry := range entries {
				require.False(t, strings.HasPrefix(entry.Name(), ".session-snapshot-"), entry.Name())
			}
			if mode == "existing destination" {
				content, err := os.ReadFile(destination)
				require.NoError(t, err)
				require.Equal(t, "keep existing destination", string(content))
			} else {
				_, err := os.Stat(destination)
				require.True(t, os.IsNotExist(err))
			}
			if mode == "missing source" {
				_, err := os.Stat(source)
				require.True(t, os.IsNotExist(err), "read-only snapshot must never create a source database")
			}
		})
	}
}

func TestSnapshotBackupDatabaseSupportsOlderDatabaseWithoutSessionTables(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.db")
	destination := filepath.Join(directory, "result.db")
	database := openSnapshotTestDB(t, source)
	execSnapshotTest(t, database, "CREATE TABLE business_records (value TEXT)")
	execSnapshotTest(t, database, "INSERT INTO business_records VALUES ('preserved')")
	require.NoError(t, snapshotBackupDatabase(source, destination))
	copied := openSnapshotTestDB(t, destination)
	var value string
	require.NoError(t, copied.QueryRow("SELECT value FROM business_records").Scan(&value))
	require.Equal(t, "preserved", value)
}

func configureSnapshotApplicationDB(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	previousPath, previousName := cosysettings.ConfPath, settings.DatabaseSettings.Name
	cosysettings.ConfPath = filepath.Join(root, "app.ini")
	settings.DatabaseSettings.Name = "custom-database"
	t.Cleanup(func() { cosysettings.ConfPath = previousPath; settings.DatabaseSettings.Name = previousName })
	return applicationDatabasePath()
}

func TestApplicationDatabaseFileRecognizesAliasesAndSidecars(t *testing.T) {
	source := configureSnapshotApplicationDB(t)
	require.NoError(t, os.WriteFile(source, []byte("database identity fixture"), 0600))
	check := func(path string, wantDatabase, wantSidecar bool) {
		t.Helper()
		info, err := os.Lstat(path)
		require.NoError(t, err)
		database, sidecar, err := applicationDatabaseFile(path, info)
		require.NoError(t, err)
		require.Equal(t, wantDatabase, database, path)
		require.Equal(t, wantSidecar, sidecar, path)
	}
	check(source, true, false)
	ordinary := filepath.Join(filepath.Dir(source), "ordinary.db")
	require.NoError(t, os.WriteFile(ordinary, []byte("ordinary"), 0600))
	check(ordinary, false, false)
	alias := filepath.Join(t.TempDir(), "hardlink.data")
	if err := os.Link(source, alias); err == nil {
		check(alias, true, false)
	}
	symlink := filepath.Join(t.TempDir(), "symlink.data")
	if err := os.Symlink(source, symlink); err == nil {
		check(symlink, true, false)
	}
	for _, suffix := range databaseSidecarSuffixes {
		sidecar := source + suffix
		require.NoError(t, os.WriteFile(sidecar, []byte("sidecar identity"), 0600))
		check(sidecar, false, true)
		linked := filepath.Join(t.TempDir(), "sidecar-alias")
		if err := os.Link(sidecar, linked); err == nil {
			check(linked, false, true)
		}
		linked = filepath.Join(t.TempDir(), "sidecar-link")
		if err := os.Symlink(sidecar, linked); err == nil {
			check(linked, false, true)
		}
	}
}

func TestApplicationDatabaseFileHandlesResolvedConfigDatabaseAndMissingFiles(t *testing.T) {
	source := configureSnapshotApplicationDB(t)
	real := filepath.Join(t.TempDir(), "real.db")
	require.NoError(t, os.WriteFile(real, []byte("real database"), 0600))
	if err := os.Symlink(real, source); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	require.NoError(t, os.WriteFile(real+"-wal", []byte("real WAL"), 0600))
	for _, path := range []string{real, real + "-wal"} {
		info, err := os.Lstat(path)
		require.NoError(t, err)
		database, sidecar, err := applicationDatabaseFile(path, info)
		require.NoError(t, err)
		require.Equal(t, path == real, database)
		require.Equal(t, path != real, sidecar)
	}
	require.NoError(t, os.Remove(source))
	ordinary := filepath.Join(t.TempDir(), "ordinary.conf")
	require.NoError(t, os.WriteFile(ordinary, []byte("ordinary"), 0600))
	info, err := os.Lstat(ordinary)
	require.NoError(t, err)
	database, sidecar, err := applicationDatabaseFile(ordinary, info)
	require.NoError(t, err)
	require.False(t, database)
	require.False(t, sidecar)
	require.NoError(t, os.WriteFile(source+"-wal", []byte("orphaned WAL"), 0600))
	info, err = os.Lstat(source + "-wal")
	require.NoError(t, err)
	database, sidecar, err = applicationDatabaseFile(source+"-wal", info)
	require.NoError(t, err)
	require.False(t, database)
	require.True(t, sidecar)
}

func TestApplicationDatabaseFileFailsClosedForUnresolvableDatabase(t *testing.T) {
	source := configureSnapshotApplicationDB(t)
	ordinary := filepath.Join(t.TempDir(), "ordinary.conf")
	require.NoError(t, os.WriteFile(ordinary, []byte("ordinary"), 0600))
	info, err := os.Lstat(ordinary)
	require.NoError(t, err)
	if err := os.Symlink(source, source); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, _, err = applicationDatabaseFile(ordinary, info)
	require.Error(t, err)
}
