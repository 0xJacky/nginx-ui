package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/minio/minio-go/v7"
	"github.com/uozi-tech/cosy/logger"
)

// autoBackupFilePrefix returns the filename prefix shared by every file that
// an auto backup task writes. A backup file is named <prefix><unix time>.zip.
func autoBackupFilePrefix(autoBackup *model.AutoBackup) string {
	if autoBackup.BackupType == model.BackupTypeCustomDir {
		return fmt.Sprintf("custom_dir_%s_", autoBackup.GetName())
	}

	return autoBackup.GetName() + "_"
}

// autoBackupFilePattern matches the backup files written by an auto backup
// task: <prefix><unix time>.zip and, for encrypted backups, the <...>.zip.key
// file stored next to it. The first submatch is the unix time.
func autoBackupFilePattern(autoBackup *model.AutoBackup) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(autoBackupFilePrefix(autoBackup)) + `(\d+)\.zip(?:\.key)?$`)
}

// expiredBackupFiles returns the names of the files that fall outside the
// newest keep backups of an auto backup task. A backup is a .zip file together
// with its optional .key file, so both are kept or removed together. Names that
// do not match the task's naming scheme are never returned.
//
// Parameters:
//   - names: File names found in the storage location
//   - pattern: Pattern returned by autoBackupFilePattern
//   - keep: Number of newest backups to keep; zero or less keeps everything
//
// Returns:
//   - []string: Names of the files to delete, oldest backup first
func expiredBackupFiles(names []string, pattern *regexp.Regexp, keep int) []string {
	if keep <= 0 {
		return nil
	}

	filesByTime := make(map[int64][]string)
	for _, name := range names {
		matches := pattern.FindStringSubmatch(name)
		if matches == nil {
			continue
		}

		backupTime, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			continue
		}
		filesByTime[backupTime] = append(filesByTime[backupTime], name)
	}

	backupTimes := make([]int64, 0, len(filesByTime))
	for backupTime := range filesByTime {
		backupTimes = append(backupTimes, backupTime)
	}
	if len(backupTimes) <= keep {
		return nil
	}
	sort.Slice(backupTimes, func(i, j int) bool { return backupTimes[i] < backupTimes[j] })

	var expired []string
	for _, backupTime := range backupTimes[:len(backupTimes)-keep] {
		files := filesByTime[backupTime]
		sort.Strings(files)
		expired = append(expired, files...)
	}

	return expired
}

// deletableBackupFiles drops the files that were just written from the list of
// expired files, so a backup is never deleted by the run that created it.
func deletableBackupFiles(expired []string, result *ExecutionResult) []string {
	created := map[string]bool{filepath.Base(result.FilePath): true}
	if result.KeyPath != "" {
		created[filepath.Base(result.KeyPath)] = true
	}

	deletable := make([]string, 0, len(expired))
	for _, name := range expired {
		if !created[name] {
			deletable = append(deletable, name)
		}
	}

	return deletable
}

// shareBackupFiles reports whether two auto backup tasks write backups with the
// same file name prefix into the same storage location, so the retention policy
// of either task would count and delete the other task's backups. Locations are
// compared loosely (case-insensitive paths, S3 endpoint ignored): a doubtful
// match only skips pruning, while a missed one deletes another task's backups.
func shareBackupFiles(a, b *model.AutoBackup) bool {
	if a.StorageType != b.StorageType || autoBackupFilePrefix(a) != autoBackupFilePrefix(b) {
		return false
	}

	switch a.StorageType {
	case model.StorageTypeLocal:
		return strings.EqualFold(normalizeLocalStoragePath(a.StoragePath), normalizeLocalStoragePath(b.StoragePath))
	case model.StorageTypeS3:
		return strings.EqualFold(a.S3Bucket, b.S3Bucket) &&
			strings.Trim(a.StoragePath, "/") == strings.Trim(b.StoragePath, "/")
	default:
		return false
	}
}

func normalizeLocalStoragePath(path string) string {
	if absPath, err := filepath.Abs(path); err == nil {
		return absPath
	}

	return filepath.Clean(path)
}

// autoBackupsSharingFiles returns the other auto backup tasks whose backups the
// retention policy of autoBackup cannot tell apart from its own.
func autoBackupsSharingFiles(autoBackup *model.AutoBackup) ([]*model.AutoBackup, error) {
	q := query.AutoBackup
	others, err := q.Where(q.ID.Neq(autoBackup.ID), q.StorageType.Eq(string(autoBackup.StorageType))).Find()
	if err != nil {
		return nil, err
	}

	var sharing []*model.AutoBackup
	for _, other := range others {
		if shareBackupFiles(autoBackup, other) {
			sharing = append(sharing, other)
		}
	}

	return sharing, nil
}

// applyRetentionPolicy prunes old backups of a task after a successful run. It
// skips pruning, and warns, when another task writes backups with the same file
// names to the same storage location, because their backups cannot be told
// apart and pruning would delete the other task's backups.
//
// Parameters:
//   - autoBackup: The auto backup configuration
//   - result: The backup execution result containing the file paths just written
func applyRetentionPolicy(autoBackup *model.AutoBackup, result *ExecutionResult) {
	if autoBackup.RetentionCount <= 0 {
		return
	}

	sharing, err := autoBackupsSharingFiles(autoBackup)
	if err != nil {
		logger.Warnf("Skipped pruning old backups of task %s: failed to check other backup tasks: %v", autoBackup.Name, err)
		return
	}

	if len(sharing) > 0 {
		names := make([]string, 0, len(sharing))
		for _, other := range sharing {
			names = append(names, other.Name)
		}
		conflictNames := strings.Join(names, ", ")

		logger.Warnf("Skipped pruning old backups of task %s: task(s) %s write backups with the same file names to the same storage location",
			autoBackup.Name, conflictNames)
		notification.Warning("Auto Backup Retention Skipped",
			"Old backups of task %{backup_name} were not deleted because task %{conflict_names} writes backups with the same file names to the same storage location. Rename one of the tasks or change its storage path.",
			map[string]interface{}{
				"backup_id":      autoBackup.ID,
				"backup_name":    autoBackup.Name,
				"conflict_names": conflictNames,
			},
		)
		return
	}

	pruneOldBackups(autoBackup, result)
}

// pruneOldBackups applies the retention policy of an auto backup task after a
// successful backup. It does nothing unless the task keeps a limited number of
// backups. Failing to delete old backups is logged and does not fail the task,
// because the new backup has already been stored.
//
// Parameters:
//   - autoBackup: The auto backup configuration
//   - result: The backup execution result containing the file paths just written
func pruneOldBackups(autoBackup *model.AutoBackup, result *ExecutionResult) {
	if autoBackup.RetentionCount <= 0 {
		return
	}

	switch autoBackup.StorageType {
	case model.StorageTypeLocal:
		pruneLocalBackups(autoBackup, result)
	case model.StorageTypeS3:
		s3Client, err := NewS3Client(autoBackup)
		if err != nil {
			logger.Warnf("Failed to prune old backups of task %s: %v", autoBackup.Name, err)
			return
		}
		if err := s3Client.PruneBackups(context.Background(), autoBackup, result); err != nil {
			logger.Warnf("Failed to prune old backups of task %s: %v", autoBackup.Name, err)
		}
	}
}

// pruneLocalBackups deletes the backups of a task in its local storage path that
// exceed the task's retention count.
func pruneLocalBackups(autoBackup *model.AutoBackup, result *ExecutionResult) {
	entries, err := os.ReadDir(autoBackup.StoragePath)
	if err != nil {
		logger.Warnf("Failed to list backups of task %s in %s: %v", autoBackup.Name, autoBackup.StoragePath, err)
		return
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}

	expired := expiredBackupFiles(names, autoBackupFilePattern(autoBackup), autoBackup.RetentionCount)
	for _, name := range deletableBackupFiles(expired, result) {
		path := filepath.Join(autoBackup.StoragePath, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Warnf("Failed to delete old backup file %s: %v", path, err)
			continue
		}
		logger.Infof("Deleted old backup file %s", path)
	}
}

// PruneBackups deletes the backups of a task in its S3 storage path that exceed
// the task's retention count. Only objects directly under the storage path whose
// names match the task's backup naming scheme are considered.
//
// Parameters:
//   - ctx: Context for the S3 operations
//   - autoBackup: The auto backup configuration
//   - result: The backup execution result containing the file paths just written
//
// Returns:
//   - error: Standard error if listing fails, or joined errors of the objects that
//     could not be deleted; a failed deletion does not stop the others
func (s3c *S3Client) PruneBackups(ctx context.Context, autoBackup *model.AutoBackup, result *ExecutionResult) error {
	keyPrefix := constructS3Key(autoBackup.StoragePath, "")

	var names []string
	for object := range s3c.client.ListObjects(ctx, s3c.bucket, minio.ListObjectsOptions{Prefix: keyPrefix}) {
		if object.Err != nil {
			return fmt.Errorf("failed to list S3 objects: %w", object.Err)
		}
		names = append(names, strings.TrimPrefix(object.Key, keyPrefix))
	}

	expired := expiredBackupFiles(names, autoBackupFilePattern(autoBackup), autoBackup.RetentionCount)
	var deleteErrs []error
	for _, name := range deletableBackupFiles(expired, result) {
		key := keyPrefix + name
		if err := s3c.client.RemoveObject(ctx, s3c.bucket, key, minio.RemoveObjectOptions{}); err != nil {
			deleteErrs = append(deleteErrs, fmt.Errorf("failed to delete S3 object %s: %w", key, err))
			continue
		}
		logger.Infof("Deleted old backup object from S3: bucket=%s, key=%s", s3c.bucket, key)
	}

	return errors.Join(deleteErrs...)
}
