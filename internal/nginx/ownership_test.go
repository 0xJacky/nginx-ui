package nginx

import (
	"os"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

type ownershipFileInfo struct {
	sys any
}

func (ownershipFileInfo) Name() string       { return "private.key" }
func (ownershipFileInfo) Size() int64        { return 0 }
func (ownershipFileInfo) Mode() os.FileMode  { return 0640 }
func (ownershipFileInfo) ModTime() time.Time { return time.Time{} }
func (ownershipFileInfo) IsDir() bool        { return false }
func (i ownershipFileInfo) Sys() any         { return i.sys }

func TestOwnershipReadsSFTPFileStat(t *testing.T) {
	ownership, ok := Ownership(ownershipFileInfo{
		sys: &sftp.FileStat{UID: 1234, GID: 5678},
	})
	if !ok {
		t.Fatal("Ownership did not recognize SFTP file metadata")
	}
	if ownership.UID != 1234 || ownership.GID != 5678 {
		t.Fatalf("Ownership = %+v, want UID 1234 and GID 5678", ownership)
	}
}
