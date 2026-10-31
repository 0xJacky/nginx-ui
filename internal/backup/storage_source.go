package backup

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// keyFileSuffix is appended to the archive name of an encrypted run.
const keyFileSuffix = ".key"

// StoredBackup is one run of a task kept by a plugin storage backend: the
// archive and, for an encrypted backup, its key file.
type StoredBackup struct {
	// Key is the key of the archive.
	Key string `json:"key"`
	// KeyFile is the key of the security key file, empty when there is none.
	KeyFile string `json:"key_file,omitempty"`
	Size    int64  `json:"size"`
	// CreatedAt is the time of the run, taken from the file name.
	CreatedAt time.Time `json:"created_at"`
	// ModifiedAt is what the backend reports, zero when unknown.
	ModifiedAt time.Time `json:"modified_at,omitzero"`
}

// runFilePrefix is the part of a run's file name before the unix time.
func runFilePrefix(autoBackup *model.AutoBackup) string {
	if autoBackup.BackupType == model.BackupTypeCustomDir {
		return fmt.Sprintf("custom_dir_%s_", autoBackup.GetName())
	}
	return autoBackup.GetName() + "_"
}

// runListPrefix is the storage.list prefix that covers every run of a task.
func runListPrefix(autoBackup *model.AutoBackup) string {
	return constructS3Key(autoBackup.StoragePath, runFilePrefix(autoBackup))
}

// runKeyPattern matches the keys of a task's archives and key files and
// captures the unix time of the run and the key file suffix.
func runKeyPattern(autoBackup *model.AutoBackup) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(runListPrefix(autoBackup)) + `(\d+)\.zip(\.key)?$`)
}

// pluginStorage resolves the source of a task on a plugin backend.
func pluginStorage(autoBackup *model.AutoBackup) (StorageSource, error) {
	if !autoBackup.IsPluginStorage() {
		return nil, ErrPluginStorageRequired
	}
	source, _, err := storageSourceFor(autoBackup.StorageType)
	return source, err
}

// validateSourceStorage checks a task on a plugin backend before a run.
func validateSourceStorage(ctx context.Context, autoBackup *model.AutoBackup) error {
	return ValidatePluginStorage(ctx, autoBackup.StorageType, autoBackup.StoragePath, autoBackup.StorageConfig)
}

// handleSourceStorage stores the files of a run through the plugin backend,
// removes the local copies and applies the retention of the task.
func handleSourceStorage(ctx context.Context, autoBackup *model.AutoBackup, result *ExecutionResult) error {
	source, err := pluginStorage(autoBackup)
	if err != nil {
		return err
	}

	storageType := string(autoBackup.StorageType)
	for _, path := range []string{result.FilePath, result.KeyPath} {
		if path == "" {
			continue
		}
		key := constructS3Key(autoBackup.StoragePath, filepath.Base(path))
		size, putErr := source.Put(ctx, storageType, autoBackup.StorageConfig, key, path)
		if putErr != nil {
			return putErr
		}
		logger.Infof("Backup file stored in %s: key=%s, size=%d bytes", storageType, key, size)
	}

	if err = cleanupLocalBackupFiles(result); err != nil {
		logger.Warnf("Failed to cleanup local backup files: %v", err)
	}

	if autoBackup.RetentionCount > 0 {
		if err = applyRetention(ctx, source, autoBackup); err != nil {
			// The run itself succeeded, the next one retries the cleanup.
			logger.Warnf("Failed to remove old backups of task %s: %v", autoBackup.Name, err)
		}
	}
	return nil
}

// listStoredRuns lists the runs of a task, newest first.
func listStoredRuns(ctx context.Context, source StorageSource, autoBackup *model.AutoBackup) ([]StoredBackup, error) {
	objects, err := source.List(ctx, string(autoBackup.StorageType), autoBackup.StorageConfig, runListPrefix(autoBackup))
	if err != nil {
		return nil, err
	}

	pattern := runKeyPattern(autoBackup)
	runs := map[int64]*StoredBackup{}
	for _, object := range objects {
		match := pattern.FindStringSubmatch(object.Key)
		if match == nil {
			continue
		}
		unix, parseErr := strconv.ParseInt(match[1], 10, 64)
		if parseErr != nil {
			continue
		}
		run, ok := runs[unix]
		if !ok {
			run = &StoredBackup{CreatedAt: time.Unix(unix, 0).UTC()}
			runs[unix] = run
		}
		if match[2] == keyFileSuffix {
			run.KeyFile = object.Key
			continue
		}
		run.Key = object.Key
		run.Size = object.Size
		run.ModifiedAt = object.ModifiedAt
	}

	list := make([]StoredBackup, 0, len(runs))
	for _, run := range runs {
		list = append(list, *run)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	return list, nil
}

// applyRetention deletes the runs of a task past its retention count. A run
// is its archive and its key file; a key file whose archive is gone counts
// as a run as well, so it is cleaned up too.
func applyRetention(ctx context.Context, source StorageSource, autoBackup *model.AutoBackup) error {
	runs, err := listStoredRuns(ctx, source, autoBackup)
	if err != nil {
		return err
	}
	if len(runs) <= autoBackup.RetentionCount {
		return nil
	}

	for _, run := range runs[autoBackup.RetentionCount:] {
		if err = deleteStoredRun(ctx, source, autoBackup, run); err != nil {
			return err
		}
		logger.Infof("Removed backup of task %s from %s: %s", autoBackup.Name, autoBackup.StorageType, run.CreatedAt.Format(time.RFC3339))
	}
	return nil
}

func deleteStoredRun(ctx context.Context, source StorageSource, autoBackup *model.AutoBackup, run StoredBackup) error {
	for _, key := range []string{run.Key, run.KeyFile} {
		if key == "" {
			continue
		}
		if err := source.Delete(ctx, string(autoBackup.StorageType), autoBackup.StorageConfig, key); err != nil {
			return err
		}
	}
	return nil
}

// ListStoredBackups lists the runs a plugin storage backend keeps for a task,
// newest first.
func ListStoredBackups(ctx context.Context, autoBackup *model.AutoBackup) ([]StoredBackup, error) {
	source, err := pluginStorage(autoBackup)
	if err != nil {
		return nil, err
	}
	runs, err := listStoredRuns(ctx, source, autoBackup)
	if err != nil {
		return nil, err
	}
	// A key file whose archive is gone cannot be restored or picked.
	archives := runs[:0]
	for _, run := range runs {
		if run.Key != "" {
			archives = append(archives, run)
		}
	}
	return archives, nil
}

// findStoredRun returns the run of a task whose archive is key.
func findStoredRun(ctx context.Context, source StorageSource, autoBackup *model.AutoBackup, key string) (StoredBackup, error) {
	match := runKeyPattern(autoBackup).FindStringSubmatch(key)
	if match == nil || match[2] != "" {
		return StoredBackup{}, cosy.WrapErrorWithParams(ErrStoredBackupNotFound, key)
	}
	runs, err := listStoredRuns(ctx, source, autoBackup)
	if err != nil {
		return StoredBackup{}, err
	}
	for _, run := range runs {
		if run.Key == key {
			return run, nil
		}
	}
	return StoredBackup{}, cosy.WrapErrorWithParams(ErrStoredBackupNotFound, key)
}

// DeleteStoredBackup removes one run of a task, its archive and its key file.
func DeleteStoredBackup(ctx context.Context, autoBackup *model.AutoBackup, key string) error {
	source, err := pluginStorage(autoBackup)
	if err != nil {
		return err
	}
	run, err := findStoredRun(ctx, source, autoBackup, key)
	if err != nil {
		return err
	}
	return deleteStoredRun(ctx, source, autoBackup, run)
}

// RestoreStoredOptions selects what a restore from a stored run replaces.
type RestoreStoredOptions struct {
	RestoreNginx   bool
	RestoreNginxUI bool
	VerifyHash     bool
}

// RestoreStoredBackup fetches one run of a task from its plugin backend and
// restores it like an uploaded backup. The caller restarts what was
// restored, as for an upload.
func RestoreStoredBackup(ctx context.Context, autoBackup *model.AutoBackup, key string, options RestoreStoredOptions) (RestoreResult, error) {
	if autoBackup.BackupType != model.BackupTypeNginxAndNginxUI {
		return RestoreResult{}, ErrStoredBackupNotRestorable
	}
	source, err := pluginStorage(autoBackup)
	if err != nil {
		return RestoreResult{}, err
	}
	run, err := findStoredRun(ctx, source, autoBackup, key)
	if err != nil {
		return RestoreResult{}, err
	}
	if run.KeyFile == "" {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrBackupFileNotFound, key+keyFileSuffix)
	}

	tempDir, err := os.MkdirTemp("", "nginx-ui-restore-stored-*")
	if err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrCreateTempDir, err.Error())
	}
	defer os.RemoveAll(tempDir)

	storageType := string(autoBackup.StorageType)
	archivePath := filepath.Join(tempDir, "backup.zip")
	if _, err = source.Get(ctx, storageType, autoBackup.StorageConfig, run.Key, archivePath); err != nil {
		return RestoreResult{}, err
	}
	keyPath := filepath.Join(tempDir, "backup.key")
	if _, err = source.Get(ctx, storageType, autoBackup.StorageConfig, run.KeyFile, keyPath); err != nil {
		return RestoreResult{}, err
	}

	aesKey, aesIv, err := readKeyFile(keyPath)
	if err != nil {
		return RestoreResult{}, err
	}

	restoreDir, err := os.MkdirTemp("", "nginx-ui-restore-*")
	if err != nil {
		return RestoreResult{}, cosy.WrapErrorWithParams(ErrCreateRestoreDir, err.Error())
	}
	result, err := Restore(RestoreOptions{
		BackupPath:     archivePath,
		AESKey:         aesKey,
		AESIv:          aesIv,
		RestoreDir:     restoreDir,
		RestoreNginx:   options.RestoreNginx,
		RestoreNginxUI: options.RestoreNginxUI,
		VerifyHash:     options.VerifyHash,
	})
	if err != nil || (!options.RestoreNginx && !options.RestoreNginxUI) {
		_ = os.RemoveAll(restoreDir)
	}
	return result, err
}

// readKeyFile parses the "<base64 key>:<base64 iv>" security key file of an
// encrypted run.
func readKeyFile(path string) (key, iv []byte, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, cosy.WrapErrorWithParams(ErrReadFile, err.Error())
	}
	parts := strings.Split(strings.TrimSpace(string(content)), ":")
	if len(parts) != 2 {
		return nil, nil, ErrInvalidSecurityToken
	}
	if key, err = base64.StdEncoding.DecodeString(parts[0]); err != nil {
		return nil, nil, cosy.WrapErrorWithParams(ErrInvalidAESKey, err.Error())
	}
	if iv, err = base64.StdEncoding.DecodeString(parts[1]); err != nil {
		return nil, nil, cosy.WrapErrorWithParams(ErrInvalidAESIV, err.Error())
	}
	return key, iv, nil
}

// TestPluginStorage checks a task on a plugin backend before it is saved:
// the configuration is valid and the backend lists the task's key prefix. It
// returns how many runs the backend already keeps there.
func TestPluginStorage(ctx context.Context, autoBackup *model.AutoBackup) (int, error) {
	if err := validateSourceStorage(ctx, autoBackup); err != nil {
		return 0, err
	}
	source, err := pluginStorage(autoBackup)
	if err != nil {
		return 0, err
	}
	runs, err := listStoredRuns(ctx, source, autoBackup)
	if err != nil {
		return 0, err
	}
	return len(runs), nil
}
