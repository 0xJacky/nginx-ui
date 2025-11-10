package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	cosylogger "github.com/uozi-tech/cosy/logger"
)

func init() {
	// Initialize logging system to avoid nil pointer exceptions during tests
	cosylogger.Init("debug")
}

// TestIsDeviceBusyError tests the device busy error detection
func TestIsDeviceBusyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "EBUSY syscall error",
			err:      syscall.EBUSY,
			expected: true,
		},
		{
			name:     "device or resource busy string",
			err:      fmt.Errorf("device or resource busy"),
			expected: true,
		},
		{
			name:     "resource busy string",
			err:      fmt.Errorf("resource busy"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("permission denied"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeviceBusyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestUnescapeOctal tests the octal escape sequence unescaping
func TestUnescapeOctal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no escape sequences",
			input:    "/mnt/data",
			expected: "/mnt/data",
		},
		{
			name:     "space escape \\040",
			input:    "/mnt/my\\040folder",
			expected: "/mnt/my folder",
		},
		{
			name:     "multiple escapes",
			input:    "/mnt\\040test\\040dir",
			expected: "/mnt test dir",
		},
		{
			name:     "incomplete escape at end",
			input:    "/mnt\\04",
			expected: "/mnt\\04",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unescapeOctal(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsMountPoint tests mount point detection
func TestIsMountPoint(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "mount-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	assert.NoError(t, err)

	// Test regular directory (should not be a mount point)
	isMountResult := isMountPoint(subDir)
	assert.False(t, isMountResult, "Regular subdirectory should not be detected as mount point")

	// Test root directory
	// Root is typically a mount point on Linux
	rootIsMountResult := isMountPoint("/")
	// We don't assert true here because it depends on the system
	// But we verify the function doesn't panic
	t.Logf("Root directory mount check result: %v", rootIsMountResult)

	// Test non-existent path
	nonExistentIsMountResult := isMountPoint("/non/existent/path")
	assert.False(t, nonExistentIsMountResult, "Non-existent path should return false")
}

// TestClearDirectoryContents tests the directory contents clearing
func TestClearDirectoryContents(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "clear-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create files and subdirectories
	testFile1 := filepath.Join(tempDir, "file1.txt")
	testFile2 := filepath.Join(tempDir, "file2.txt")
	subDir := filepath.Join(tempDir, "subdir")
	subFile := filepath.Join(subDir, "subfile.txt")

	err = os.WriteFile(testFile1, []byte("test content 1"), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(testFile2, []byte("test content 2"), 0644)
	assert.NoError(t, err)

	err = os.MkdirAll(subDir, 0755)
	assert.NoError(t, err)

	err = os.WriteFile(subFile, []byte("sub content"), 0644)
	assert.NoError(t, err)

	// Verify files exist before clearing
	assert.FileExists(t, testFile1)
	assert.FileExists(t, testFile2)
	assert.FileExists(t, subFile)
	assert.DirExists(t, subDir)

	// Clear directory contents
	err = clearDirectoryContents(tempDir)
	assert.NoError(t, err)

	// Verify directory still exists
	assert.DirExists(t, tempDir)

	// Verify all contents are removed
	entries, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Empty(t, entries, "Directory should be empty after clearing")
}

// TestClearDirectoryContentsWithNestedDirs tests clearing nested directory structures
func TestClearDirectoryContentsWithNestedDirs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clear-nested-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create nested structure: tempDir/level1/level2/level3
	level1 := filepath.Join(tempDir, "level1")
	level2 := filepath.Join(level1, "level2")
	level3 := filepath.Join(level2, "level3")
	
	err = os.MkdirAll(level3, 0755)
	assert.NoError(t, err)

	// Add files at each level
	err = os.WriteFile(filepath.Join(level1, "file1.txt"), []byte("level1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(level2, "file2.txt"), []byte("level2"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(level3, "file3.txt"), []byte("level3"), 0644)
	assert.NoError(t, err)

	// Clear contents
	err = clearDirectoryContents(tempDir)
	assert.NoError(t, err)

	// Verify root directory exists but is empty
	assert.DirExists(t, tempDir)
	entries, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

// TestCleanDirectoryPreservingStructure tests the main cleaning function
func TestCleanDirectoryPreservingStructure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clean-structure-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a complex directory structure
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(dir1, "file2.txt")

	err = os.MkdirAll(dir1, 0755)
	assert.NoError(t, err)
	err = os.MkdirAll(dir2, 0755)
	assert.NoError(t, err)
	err = os.WriteFile(file1, []byte("content1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	assert.NoError(t, err)

	// Clean the directory
	err = cleanDirectoryPreservingStructure(tempDir)
	assert.NoError(t, err)

	// Verify root directory exists
	assert.DirExists(t, tempDir)

	// Verify all contents are removed
	entries, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Empty(t, entries, "Directory should be empty after cleaning")
}

// TestCleanDirectoryPreservingStructureEmptyDir tests cleaning an already empty directory
func TestCleanDirectoryPreservingStructureEmptyDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clean-empty-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Clean already empty directory
	err = cleanDirectoryPreservingStructure(tempDir)
	assert.NoError(t, err)

	// Verify directory still exists
	assert.DirExists(t, tempDir)
}

// TestCleanDirectoryPreservingStructureWithSymlinks tests cleaning with symbolic links
func TestCleanDirectoryPreservingStructureWithSymlinks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clean-symlink-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a target file
	targetFile := filepath.Join(tempDir, "target.txt")
	err = os.WriteFile(targetFile, []byte("target content"), 0644)
	assert.NoError(t, err)

	// Create a symlink
	symlinkPath := filepath.Join(tempDir, "link.txt")
	err = os.Symlink(targetFile, symlinkPath)
	assert.NoError(t, err)

	// Verify symlink exists
	_, err = os.Lstat(symlinkPath)
	assert.NoError(t, err)

	// Clean directory
	err = cleanDirectoryPreservingStructure(tempDir)
	assert.NoError(t, err)

	// Verify directory exists and is empty
	assert.DirExists(t, tempDir)
	entries, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

// TestCleanDirectoryPreservingStructureNonExistent tests error handling for non-existent directory
func TestCleanDirectoryPreservingStructureNonExistent(t *testing.T) {
	nonExistentDir := "/tmp/non-existent-dir-12345"
	
	err := cleanDirectoryPreservingStructure(nonExistentDir)
	assert.Error(t, err, "Should return error for non-existent directory")
}

