package nginx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// openForWrite replaced a Stat round trip with an exclusive create, so the
// new/existing decision that drives the chmod must survive every outcome the
// SFTP server can produce.
func TestOpenForWrite(t *testing.T) {
	errFailure := errors.New("sftp: \"Failure\" (SSH_FX_FAILURE)")
	errPermission := os.ErrPermission

	tests := []struct {
		name      string
		createErr error
		truncErr  error
		wantNew   bool
		wantErr   error
		wantFlags []int
	}{
		{
			name:      "new file is created exclusively",
			wantNew:   true,
			wantFlags: []int{os.O_WRONLY | os.O_CREATE | os.O_EXCL},
		},
		{
			name:      "existing file is truncated without chmod",
			createErr: errFailure,
			wantNew:   false,
			wantFlags: []int{os.O_WRONLY | os.O_CREATE | os.O_EXCL, os.O_WRONLY | os.O_TRUNC},
		},
		{
			name:      "missing parent reports the create failure",
			createErr: os.ErrNotExist,
			truncErr:  os.ErrNotExist,
			wantErr:   os.ErrNotExist,
			wantFlags: []int{os.O_WRONLY | os.O_CREATE | os.O_EXCL, os.O_WRONLY | os.O_TRUNC},
		},
		{
			name:      "unwritable directory reports the create failure",
			createErr: errPermission,
			truncErr:  os.ErrNotExist,
			wantErr:   errPermission,
			wantFlags: []int{os.O_WRONLY | os.O_CREATE | os.O_EXCL, os.O_WRONLY | os.O_TRUNC},
		},
		{
			name:      "unwritable existing file reports the truncate failure",
			createErr: errFailure,
			truncErr:  errPermission,
			wantErr:   errPermission,
			wantFlags: []int{os.O_WRONLY | os.O_CREATE | os.O_EXCL, os.O_WRONLY | os.O_TRUNC},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotFlags []int
			open := func(flags int) (targetFile, error) {
				gotFlags = append(gotFlags, flags)
				var err error
				if flags&os.O_EXCL != 0 {
					err = tt.createErr
				} else {
					err = tt.truncErr
				}
				if err != nil {
					return nil, err
				}
				return os.Create(filepath.Join(t.TempDir(), "handle"))
			}

			file, isNew, err := openForWrite(open)
			if file != nil {
				_ = file.Close()
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("openForWrite() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && file == nil {
				t.Fatal("openForWrite() returned a nil file without an error")
			}
			if tt.wantErr != nil && file != nil {
				t.Fatal("openForWrite() returned a file together with an error")
			}
			if isNew != tt.wantNew {
				t.Fatalf("openForWrite() isNew = %v, want %v", isNew, tt.wantNew)
			}
			if len(gotFlags) != len(tt.wantFlags) {
				t.Fatalf("open flags = %v, want %v", gotFlags, tt.wantFlags)
			}
			for i := range gotFlags {
				if gotFlags[i] != tt.wantFlags[i] {
					t.Fatalf("open flags = %v, want %v", gotFlags, tt.wantFlags)
				}
			}
		})
	}
}

// The exclusive create must observe the same outcome on a real filesystem:
// *os.File satisfies targetFile, so the helper can be driven through os.OpenFile.
func TestOpenForWriteAgainstLocalFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nginx.conf")
	open := func(flags int) (targetFile, error) {
		return os.OpenFile(path, flags, 0o644)
	}

	file, isNew, err := openForWrite(open)
	if err != nil {
		t.Fatalf("first openForWrite() error = %v", err)
	}
	if !isNew {
		t.Fatal("first openForWrite() isNew = false, want true")
	}
	if _, err = file.Write([]byte("first")); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	file, isNew, err = openForWrite(open)
	if err != nil {
		t.Fatalf("second openForWrite() error = %v", err)
	}
	if isNew {
		t.Fatal("second openForWrite() isNew = true, want false")
	}
	if _, err = file.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "x" {
		t.Fatalf("existing file content = %q, want it truncated to %q", content, "x")
	}

	missing := filepath.Join(t.TempDir(), "missing", "nginx.conf")
	_, _, err = openForWrite(func(flags int) (targetFile, error) {
		return os.OpenFile(missing, flags, 0o644)
	})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("openForWrite() under a missing directory error = %v, want %v", err, os.ErrNotExist)
	}
}
