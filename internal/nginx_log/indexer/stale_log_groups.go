package indexer

import (
	"errors"
	"io/fs"
	"os"
	"sort"

	"github.com/uozi-tech/cosy/logger"
)

// configuredGroupsLocked returns the main log paths of every log declared by
// the nginx configuration (or registered as an nginx default log). The caller
// must hold cacheMutex.
func (lm *LogFileManager) configuredGroupsLocked() map[string]bool {
	groups := make(map[string]bool, len(lm.logCache))
	for path := range lm.logCache {
		groups[getMainLogPathFromFile(path)] = true
	}
	return groups
}

// isStaleGroup reports whether a log group recorded in the index metadata is
// gone: no configuration declares it any more and its main log file no longer
// exists on disk. Both conditions are required. A log that is still on disk
// keeps its group even when no configuration declares it, and a configured log
// whose file is temporarily missing (e.g. rotated away) keeps it as well. Any
// error other than "does not exist" keeps the group, so a permission problem
// never drops index data.
func isStaleGroup(mainLogPath string, configuredGroups map[string]bool) bool {
	if configuredGroups[mainLogPath] {
		return false
	}
	_, err := os.Stat(mainLogPath)
	return errors.Is(err, fs.ErrNotExist)
}

// StaleLogGroups returns the main log paths of the log groups in the index
// metadata that are gone (see isStaleGroup), in a stable order. Groups with a
// file that is indexing or queued are left out, so an indexing job never has
// its group deleted underneath it.
func (lm *LogFileManager) StaleLogGroups() ([]string, error) {
	lm.cacheMutex.RLock()
	configuredGroups := lm.configuredGroupsLocked()
	lm.cacheMutex.RUnlock()

	indexes, err := lm.persistence.GetAllLogIndexes()
	if err != nil {
		return nil, err
	}

	busy := make(map[string]bool)
	candidates := make(map[string]bool)
	lm.indexingMutex.RLock()
	for _, idx := range indexes {
		group := idx.MainLogPath
		if group == "" {
			group = getMainLogPathFromFile(idx.Path)
		}
		if lm.indexingStatus[idx.Path] || lm.indexingStatus[group] ||
			idx.IndexStatus == string(IndexStatusIndexing) || idx.IndexStatus == string(IndexStatusQueued) {
			busy[group] = true
		}
		candidates[group] = true
	}
	lm.indexingMutex.RUnlock()

	var stale []string
	for group := range candidates {
		if !busy[group] && isStaleGroup(group, configuredGroups) {
			stale = append(stale, group)
		}
	}
	sort.Strings(stale)
	return stale, nil
}

// PruneStaleLogGroups removes the index documents and the index metadata of
// every stale log group, e.g. the access log of a site that was deleted
// together with its log file. Log files on disk are never touched.
//
// isBusy, when not nil, reports whether an indexing task currently holds a log
// group; such a group is skipped and retried on the next call. When deleting a
// group's documents fails, its metadata is kept as well, because the metadata
// is what lists the files whose documents still have to be deleted.
func (lm *LogFileManager) PruneStaleLogGroups(isBusy func(mainLogPath string) bool) {
	groups, err := lm.StaleLogGroups()
	if err != nil {
		logger.Warnf("Failed to look up stale nginx log index groups: %v", err)
		return
	}

	for _, group := range groups {
		if isBusy != nil && isBusy(group) {
			continue
		}

		if lm.indexer != nil {
			if err := lm.indexer.DeleteIndexByLogGroup(group, lm); err != nil {
				logger.Warnf("Failed to delete index documents of stale log group %s: %v", group, err)
				continue
			}
		}

		if err := lm.DeleteIndexMetadataByGroup(group); err != nil {
			logger.Warnf("Failed to delete index metadata of stale log group %s: %v", group, err)
			continue
		}

		logger.Infof("Removed the index of log group %s: it is no longer configured and its log file is gone", group)
	}
}
