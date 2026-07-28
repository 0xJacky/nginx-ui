package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	cosysettings "github.com/uozi-tech/cosy/settings"
)

func TestRestoreDoesNotDecryptOrExtractUnselectedComponents(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	configPath := filepath.Join(tempDir, "config", "config.ini")
	if err := os.WriteFile(configPath, []byte("[app]\nName = Selection Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalConfPath := cosysettings.ConfPath
	cosysettings.ConfPath = configPath
	defer func() { cosysettings.ConfPath = originalConfPath }()

	backupResult, err := Backup()
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(tempDir, backupResult.BackupName)
	if err := os.WriteFile(backupPath, backupResult.BackupContent, 0o644); err != nil {
		t.Fatal(err)
	}
	key, err := DecodeFromBase64(backupResult.AESKey)
	if err != nil {
		t.Fatal(err)
	}
	iv, err := DecodeFromBase64(backupResult.AESIv)
	if err != nil {
		t.Fatal(err)
	}

	restoreDir := filepath.Join(tempDir, "restore-selection")
	result, err := Restore(RestoreOptions{
		BackupPath: backupPath,
		AESKey:     key,
		AESIv:      iv,
		RestoreDir: restoreDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.NginxRestored || result.NginxUIRestored {
		t.Fatalf("unselected components were reported as restored: %+v", result)
	}
	for _, directory := range []string{NginxDir, NginxUIDir} {
		if _, err := os.Stat(filepath.Join(restoreDir, directory)); !os.IsNotExist(err) {
			t.Fatalf("unselected component %q was extracted: %v", directory, err)
		}
	}
	for _, encryptedArchive := range []string{NginxZipName, NginxUIZipName} {
		if _, err := zip.OpenReader(filepath.Join(restoreDir, encryptedArchive)); err == nil {
			t.Fatalf("unselected component %q was decrypted", encryptedArchive)
		}
	}
}
