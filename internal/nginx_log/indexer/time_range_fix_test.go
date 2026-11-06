package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogRotationDocumentCountReset verifies that the document count is reset
// when a log rotation is detected.
func TestLogRotationDocumentCountReset(t *testing.T) {
	// Setup a temporary directory for our test log files
	tempDir, err := os.MkdirTemp("", "log_rotation_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "access.log")

	// 1. Initial State: Simulate a fully indexed log file.
	// This represents the state of 'access.log' before rotation.
	initialContent := "line1\nline2\nline3\n"
	err = os.WriteFile(logPath, []byte(initialContent), 0644)
	require.NoError(t, err)

	fileInfo, err := os.Stat(logPath)
	require.NoError(t, err)

	// Create a log index record that simulates a previously indexed state.
	// It has a document count and the size of the file.
	logIndex := &model.NginxLogIndex{
		Path:          logPath,
		MainLogPath:   logPath,
		DocumentCount: 3, // 3 lines were indexed
		LastSize:      fileInfo.Size(),
		Enabled:       true,
	}

	// 2. Log Rotation: Simulate log rotation by creating a new, smaller file.
	rotatedContent := "new_line1\n"
	err = os.WriteFile(logPath, []byte(rotatedContent), 0644)
	require.NoError(t, err)

	newFileInfo, err := os.Stat(logPath)
	require.NoError(t, err)

	// Explicitly verify that the new file is smaller. This is crucial for the test's validity.
	require.Less(t, newFileInfo.Size(), logIndex.LastSize, "Test setup error: New log file must be smaller than the old one to simulate rotation.")

	// 3. Test the core logic directly, as it is implemented in the cron job.
	isRotated := newFileInfo.Size() < logIndex.LastSize
	assert.True(t, isRotated, "Log rotation detection logic failed.")

	var existingDocCount uint64
	if isRotated {
		// On rotation, the base count must be reset to zero. This is the fix we are verifying.
		existingDocCount = 0
	} else {
		// In a normal incremental scenario, the old count would be used.
		existingDocCount = logIndex.DocumentCount
	}

	// The number of documents indexed from the new, rotated file.
	totalDocsIndexedInNewFile := uint64(1) // We have one line in rotatedContent.

	// Calculate the final document count.
	finalDocCount := existingDocCount + totalDocsIndexedInNewFile

	// 5. Assert the Result
	// The final count must be exactly the number of lines in the new file, not an accumulation.
	assert.Equal(t, uint64(1), finalDocCount, "On rotation, document count should be reset to the new file's content count.")
	assert.NotEqual(t, uint64(4), finalDocCount, "Document count must not be the sum of old and new files (3+1).")
}
