package config

import (
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/samber/lo"
	"gorm.io/gen/field"
)

// SaveDirectorySyncTargets stores the deployment targets of a directory. Files
// below it inherit those targets, so a whole tree can be replicated instead of
// being deployed file by file.
func SaveDirectorySyncTargets(dir string, nodeIDs []uint64, overwrite bool) error {
	q := query.Config
	record, err := q.Assign(field.Attrs(&model.Config{
		Filepath: dir,
		Name:     filepath.Base(dir),
		IsDir:    true,
	})).Where(q.Filepath.Eq(dir)).FirstOrCreate()
	if err != nil {
		return err
	}

	_, err = q.Where(q.ID.Eq(record.ID)).
		Select(q.IsDir, q.SyncNodeIds, q.SyncOverwrite).
		Updates(&model.Config{
			IsDir:         true,
			SyncNodeIds:   nodeIDs,
			SyncOverwrite: overwrite,
		})

	return err
}

// InheritedSyncTargets returns the deployment targets of the closest ancestor
// directory of absPath. The deepest directory wins so a nested override can
// narrow the targets of its parent.
func InheritedSyncTargets(absPath string) (nodeIDs []uint64, overwrite bool) {
	q := query.Config
	directories, err := q.Where(q.IsDir.Is(true)).Find()
	if err != nil {
		return nil, false
	}

	best := ""
	for _, directory := range directories {
		if len(directory.SyncNodeIds) == 0 {
			continue
		}
		if !helper.IsUnderDirectory(absPath, directory.Filepath) {
			continue
		}
		if len(directory.Filepath) <= len(best) {
			continue
		}

		best = directory.Filepath
		nodeIDs = directory.SyncNodeIds
		overwrite = directory.SyncOverwrite
	}

	return nodeIDs, overwrite
}

// EffectiveSyncTargets merges the targets configured on the file itself with the
// ones inherited from its directory.
func EffectiveSyncTargets(cfg *model.Config) (nodeIDs []uint64, overwrite bool) {
	if cfg == nil {
		return nil, false
	}

	inherited, inheritedOverwrite := InheritedSyncTargets(cfg.Filepath)

	return lo.Uniq(append(append([]uint64{}, cfg.SyncNodeIds...), inherited...)),
		cfg.SyncOverwrite || inheritedOverwrite
}
