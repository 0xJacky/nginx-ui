package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceFilesWithRollbackRestoresOriginals(t *testing.T) {
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "app.ini")
	secondDestination := filepath.Join(directory, "database.db")
	require.NoError(t, os.WriteFile(firstDestination, []byte("old-config"), 0o600))
	require.NoError(t, os.WriteFile(secondDestination, []byte("old-db"), 0o600))

	firstStage := filepath.Join(directory, "config.stage")
	missingSecondStage := filepath.Join(directory, "missing.stage")
	require.NoError(t, os.WriteFile(firstStage, []byte("new-config"), 0o600))

	err := replaceFilesWithRollback([]stagedFileReplacement{
		{destination: firstDestination, staged: firstStage},
		{destination: secondDestination, staged: missingSecondStage},
	}, nil)
	require.Error(t, err)

	firstContent, err := os.ReadFile(firstDestination)
	require.NoError(t, err)
	secondContent, err := os.ReadFile(secondDestination)
	require.NoError(t, err)
	assert.Equal(t, "old-config", string(firstContent))
	assert.Equal(t, "old-db", string(secondContent))
}

func TestReplaceDirectoryInstallsCompleteCandidate(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "nginx")
	candidate := filepath.Join(parent, "candidate")
	require.NoError(t, os.MkdirAll(destination, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(destination, "old.conf"), []byte("old"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(candidate, "sites"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(candidate, "nginx.conf"), []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(candidate, "sites", "app.conf"), []byte("site"), 0o600))

	require.NoError(t, replaceDirectory(candidate, destination))
	_, err := os.Stat(filepath.Join(destination, "old.conf"))
	assert.True(t, os.IsNotExist(err))
	content, err := os.ReadFile(filepath.Join(destination, "sites", "app.conf"))
	require.NoError(t, err)
	assert.Equal(t, "site", string(content))
}

func TestCrossTargetFailureRollsBackEarlierDirectoryReplacement(t *testing.T) {
	nginxParent := t.TempDir()
	nginxDestination := filepath.Join(nginxParent, "nginx")
	nginxCandidate := filepath.Join(nginxParent, "candidate")
	require.NoError(t, os.MkdirAll(nginxDestination, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nginxDestination, "nginx.conf"), []byte("old"), 0o600))
	require.NoError(t, os.MkdirAll(nginxCandidate, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nginxCandidate, "nginx.conf"), []byte("new"), 0o600))

	appliedDirectory, err := applyDirectoryWithRollback(nginxCandidate, nginxDestination)
	require.NoError(t, err)
	fileParent := t.TempDir()
	fileDestination := filepath.Join(fileParent, "app.ini")
	require.NoError(t, os.WriteFile(fileDestination, []byte("old-config"), 0o600))
	_, err = applyFilesWithRollback([]stagedFileReplacement{{
		destination: fileDestination,
		staged:      filepath.Join(fileParent, "missing-stage"),
	}}, nil)
	require.Error(t, err)
	require.NoError(t, appliedDirectory.Rollback())

	content, err := os.ReadFile(filepath.Join(nginxDestination, "nginx.conf"))
	require.NoError(t, err)
	assert.Equal(t, "old", string(content))
}
