package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
	_ "github.com/mattn/go-sqlite3"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

var databaseSidecarSuffixes = []string{"-wal", "-shm", "-journal"}

// applicationDatabasePath mirrors the application's database name/config path
// contract. An unset config path is valid for standalone archive utilities.
func applicationDatabasePath() string {
	if cosysettings.ConfPath == "" {
		return ""
	}
	name := settings.DatabaseSettings.Name
	if name == "" {
		name = "database"
	}
	return filepath.Join(filepath.Dir(cosysettings.ConfPath), name+".db")
}

// applicationDatabaseFile recognizes the configured database and its sidecars,
// including symlink and hardlink aliases. Unresolvable filesystem identities
// fail closed instead of letting an unclassified live database enter a backup.
func applicationDatabaseFile(path string, info os.FileInfo) (database bool, sidecar bool, err error) {
	configured := applicationDatabasePath()
	if configured == "" {
		return false, false, nil
	}
	if info == nil {
		return false, false, fmt.Errorf("classify application database: missing file information for %s", path)
	}
	live, err := resolveDatabaseComparisonPath(configured, 0)
	if err != nil {
		return false, false, fmt.Errorf("resolve application database: %w", err)
	}
	candidate, err := resolveDatabaseComparisonPath(path, 0)
	if err != nil {
		return false, false, fmt.Errorf("resolve backup file %s: %w", path, err)
	}
	current, statErr := os.Stat(path)
	if statErr != nil && !os.IsNotExist(statErr) {
		return false, false, fmt.Errorf("stat backup file %s: %w", path, statErr)
	}
	liveInfo, liveErr := os.Stat(configured)
	if liveErr != nil && !os.IsNotExist(liveErr) {
		return false, false, fmt.Errorf("stat application database: %w", liveErr)
	}
	if liveErr == nil && !liveInfo.Mode().IsRegular() {
		return false, false, fmt.Errorf("application database is not a regular file: %s", configured)
	}
	if candidate == live || (statErr == nil && liveErr == nil && os.SameFile(current, liveInfo)) {
		return true, false, nil
	}
	for _, suffix := range databaseSidecarSuffixes {
		// SQLite may use the resolved database name for WAL files even when the
		// application opened the main file through a configured symlink.
		for _, base := range []string{configured, live} {
			companion := base + suffix
			canonical, resolveErr := resolveDatabaseComparisonPath(companion, 0)
			if resolveErr != nil {
				return false, false, fmt.Errorf("resolve database sidecar: %w", resolveErr)
			}
			if candidate == canonical {
				return false, true, nil
			}
			companionInfo, companionErr := os.Stat(companion)
			if companionErr != nil {
				if os.IsNotExist(companionErr) {
					continue
				}
				return false, false, fmt.Errorf("stat database sidecar: %w", companionErr)
			}
			if statErr == nil && os.SameFile(current, companionInfo) {
				return false, true, nil
			}
		}
	}
	return false, false, nil
}

// resolveDatabaseComparisonPath also resolves existing ancestors and dangling
// leaf links, so an absent database does not break ordinary directory backups.
func resolveDatabaseComparisonPath(path string, depth int) (string, error) {
	if depth > 64 {
		return "", fmt.Errorf("database path resolution limit exceeded: %s", path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	info, statErr := os.Lstat(absolute)
	if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(absolute)
		if readErr != nil {
			return "", readErr
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(absolute), target)
		}
		return resolveDatabaseComparisonPath(target, depth+1)
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return "", statErr
	}
	parent := filepath.Dir(absolute)
	if parent == absolute {
		return "", err
	}
	resolvedParent, err := resolveDatabaseComparisonPath(parent, depth+1)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(absolute)), nil
}

func snapshotSQLiteConnection(path, mode string, action func(*sql.Conn) error) (err error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	parameters := uri.Query()
	parameters.Set("mode", mode)
	parameters.Set("_busy_timeout", "5000")
	uri.RawQuery = parameters.Encode()
	database, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return err
	}
	database.SetMaxOpenConns(1)
	defer func() { err = errors.Join(err, database.Close()) }()
	connection, err := database.Conn(context.Background())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, connection.Close()) }()
	return action(connection)
}

// snapshotBackupDatabase creates a WAL-consistent read-only snapshot, then
// removes reusable sessions and their free-page remnants from the staged copy.
// Source connections never change journaling, checkpoint, delete, or vacuum the
// live database. No failed snapshot is ever replaced with a raw file copy.
func snapshotBackupDatabase(source, destination string) (err error) {
	for _, path := range append([]string{destination}, snapshotSidecars(destination)...) {
		if _, existingErr := os.Lstat(path); existingErr == nil {
			return fmt.Errorf("backup database destination already exists: %s", path)
		} else if !os.IsNotExist(existingErr) {
			return existingErr
		}
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".session-snapshot-*.db")
	if err != nil {
		return err
	}
	staged := temporary.Name()
	defer func() {
		for _, path := range append([]string{staged}, snapshotSidecars(staged)...) {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				err = errors.Join(err, removeErr)
			}
		}
	}()
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = snapshotSQLiteConnection(source, "ro", func(connection *sql.Conn) error {
		_, snapshotErr := connection.ExecContext(context.Background(), "VACUUM INTO ?", staged)
		return snapshotErr
	}); err != nil {
		return fmt.Errorf("snapshot application database: %w", err)
	}
	if err = snapshotSQLiteConnection(staged, "rw", func(connection *sql.Conn) error {
		var mode string
		if pragmaErr := connection.QueryRowContext(context.Background(), "PRAGMA journal_mode=DELETE").Scan(&mode); pragmaErr != nil {
			return pragmaErr
		}
		if !strings.EqualFold(mode, "delete") {
			return fmt.Errorf("staged database cannot use rollback journaling: %s", mode)
		}
		var secureDelete int
		if pragmaErr := connection.QueryRowContext(context.Background(), "PRAGMA secure_delete=ON").Scan(&secureDelete); pragmaErr != nil {
			return pragmaErr
		}
		if secureDelete != 1 {
			return fmt.Errorf("staged database secure_delete is not enabled")
		}
		for _, table := range []string{"auth_tokens", "dev_secure_sessions"} {
			var exists int
			if queryErr := connection.QueryRowContext(context.Background(), "SELECT count(*) FROM sqlite_schema WHERE type='table' AND name=?", table).Scan(&exists); queryErr != nil {
				return queryErr
			}
			if exists == 0 {
				continue
			}
			if _, deleteErr := connection.ExecContext(context.Background(), `DELETE FROM "`+table+`"`); deleteErr != nil {
				return fmt.Errorf("remove backup sessions from %s: %w", table, deleteErr)
			}
		}
		_, vacuumErr := connection.ExecContext(context.Background(), "VACUUM")
		return vacuumErr
	}); err != nil {
		return fmt.Errorf("sanitize backup database: %w", err)
	}
	for _, path := range snapshotSidecars(staged) {
		if _, sidecarErr := os.Lstat(path); sidecarErr == nil {
			return fmt.Errorf("staged database left a sidecar: %s", path)
		} else if !os.IsNotExist(sidecarErr) {
			return sidecarErr
		}
	}
	if _, existingErr := os.Lstat(destination); existingErr == nil {
		return fmt.Errorf("backup database destination appeared during snapshot: %s", destination)
	} else if !os.IsNotExist(existingErr) {
		return existingErr
	}
	if err = os.Rename(staged, destination); err != nil {
		return err
	}
	return nil
}

func snapshotSidecars(path string) []string {
	paths := make([]string, 0, len(databaseSidecarSuffixes))
	for _, suffix := range databaseSidecarSuffixes {
		paths = append(paths, path+suffix)
	}
	return paths
}
