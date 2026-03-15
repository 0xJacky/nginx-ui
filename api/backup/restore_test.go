package backup

import "testing"

func TestUploadedBackupPathIgnoresClientFileName(t *testing.T) {
	tempDir := "/tmp/nginx-ui-restore-upload-123"

	got := uploadedBackupPath(tempDir)
	want := "/tmp/nginx-ui-restore-upload-123/uploaded-backup.zip"

	if got != want {
		t.Fatalf("uploadedBackupPath(%q) = %q, want %q", tempDir, got, want)
	}
}
