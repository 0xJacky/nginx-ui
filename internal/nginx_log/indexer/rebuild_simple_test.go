package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBasicOptimizationLogic tests the core optimization logic without complex mocks
func TestBasicOptimizationLogic(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	
	// Create test files
	activeLogPath := filepath.Join(tmpDir, "access.log")
	compressedLogPath := filepath.Join(tmpDir, "access.log.1.gz")
	
	if err := os.WriteFile(activeLogPath, []byte("test log content\n"), 0644); err != nil {
		t.Fatalf("Failed to create active log file: %v", err)
	}
	if err := os.WriteFile(compressedLogPath, []byte("compressed content"), 0644); err != nil {
		t.Fatalf("Failed to create compressed log file: %v", err)
	}
	
	// Create rebuild manager with no persistence (should always process)
	config := DefaultRebuildConfig()
	rm := &RebuildManager{
		persistence: nil,
		config:      config,
	}
	
	// Test active file without persistence
	activeFile := &LogGroupFile{
		Path:         activeLogPath,
		IsCompressed: false,
	}
	
	shouldProcess, reason := rm.shouldProcessFile(activeFile)
	if !shouldProcess {
		t.Errorf("Expected to process active file without persistence, got false: %s", reason)
	}
	t.Logf("✅ Active file without persistence: shouldProcess=%v, reason=%s", shouldProcess, reason)
	
	// Test compressed file without persistence
	compressedFile := &LogGroupFile{
		Path:         compressedLogPath,
		IsCompressed: true,
	}
	
	shouldProcess, reason = rm.shouldProcessFile(compressedFile)
	if !shouldProcess {
		t.Errorf("Expected to process compressed file without persistence, got false: %s", reason)
	}
	t.Logf("✅ Compressed file without persistence: shouldProcess=%v, reason=%s", shouldProcess, reason)
}