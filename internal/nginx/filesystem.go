package nginx

import (
	"context"
	"errors"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/pkg/sftp"
)

var ErrInvalidTargetFilesystem = errors.New("nginx target filesystem requires an explicit valid access mode")

type targetFile interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.Closer
}

type targetFilesystem interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, os.FileMode) error
	ReadDir(string) ([]os.DirEntry, error)
	Stat(string) (os.FileInfo, error)
	Lstat(string) (os.FileInfo, error)
	Readlink(string) (string, error)
	Symlink(string, string) error
	Remove(string) error
	RemoveAll(string) error
	Rename(string, string) error
	Mkdir(string, os.FileMode) error
	MkdirAll(string, os.FileMode) error
	Open(string) (targetFile, error)
	OpenFile(string, int, os.FileMode) (targetFile, error)
	Chmod(string, os.FileMode) error
	Chtimes(string, time.Time, time.Time) error
	Glob(string) ([]string, error)
	EvalSymlinks(string) (string, error)
}

type localTargetFilesystem struct{}

func (localTargetFilesystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (localTargetFilesystem) WriteFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}
func (localTargetFilesystem) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }
func (localTargetFilesystem) Stat(path string) (os.FileInfo, error)      { return os.Stat(path) }
func (localTargetFilesystem) Lstat(path string) (os.FileInfo, error)     { return os.Lstat(path) }
func (localTargetFilesystem) Readlink(path string) (string, error)       { return os.Readlink(path) }
func (localTargetFilesystem) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}
func (localTargetFilesystem) Remove(path string) error    { return os.Remove(path) }
func (localTargetFilesystem) RemoveAll(path string) error { return os.RemoveAll(path) }
func (localTargetFilesystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
func (localTargetFilesystem) Mkdir(path string, mode os.FileMode) error {
	return os.Mkdir(path, mode)
}
func (localTargetFilesystem) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}
func (localTargetFilesystem) Open(path string) (targetFile, error) { return os.Open(path) }
func (localTargetFilesystem) OpenFile(path string, flag int, mode os.FileMode) (targetFile, error) {
	return os.OpenFile(path, flag, mode)
}
func (localTargetFilesystem) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}
func (localTargetFilesystem) Chtimes(path string, atime, mtime time.Time) error {
	return os.Chtimes(path, atime, mtime)
}
func (localTargetFilesystem) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}
func (localTargetFilesystem) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

type sftpTargetFilesystem struct{}

func (sftpTargetFilesystem) client() (*sftp.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return sharedSSHClient().SFTP(ctx)
}

func (fs sftpTargetFilesystem) ReadFile(path string) ([]byte, error) {
	client, err := fs.client()
	if err != nil {
		return nil, err
	}
	file, err := client.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}
func (fs sftpTargetFilesystem) WriteFile(path string, data []byte, mode os.FileMode) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	_, statErr := client.Stat(path)
	isNew := os.IsNotExist(statErr)
	if statErr != nil && !isNew {
		return statErr
	}
	file, err := client.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if isNew {
		return client.Chmod(path, mode)
	}
	return nil
}
func (fs sftpTargetFilesystem) ReadDir(path string) ([]os.DirEntry, error) {
	client, err := fs.client()
	if err != nil {
		return nil, err
	}
	infos, err := client.ReadDir(path)
	if err != nil {
		return nil, err
	}
	entries := make([]os.DirEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, iofs.FileInfoToDirEntry(info))
	}
	return entries, nil
}
func (fs sftpTargetFilesystem) Stat(path string) (os.FileInfo, error) {
	client, err := fs.client()
	if err != nil {
		return nil, err
	}
	return client.Stat(path)
}
func (fs sftpTargetFilesystem) Lstat(path string) (os.FileInfo, error) {
	client, err := fs.client()
	if err != nil {
		return nil, err
	}
	return client.Lstat(path)
}
func (fs sftpTargetFilesystem) Readlink(path string) (string, error) {
	client, err := fs.client()
	if err != nil {
		return "", err
	}
	return client.ReadLink(path)
}
func (fs sftpTargetFilesystem) Symlink(oldname, newname string) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.Symlink(oldname, newname)
}
func (fs sftpTargetFilesystem) Remove(path string) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.Remove(path)
}
func (fs sftpTargetFilesystem) RemoveAll(path string) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.RemoveAll(path)
}
func (fs sftpTargetFilesystem) Rename(oldpath, newpath string) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.Rename(oldpath, newpath)
}
func (fs sftpTargetFilesystem) Mkdir(path string, _ os.FileMode) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.Mkdir(path)
}
func (fs sftpTargetFilesystem) MkdirAll(path string, _ os.FileMode) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.MkdirAll(path)
}
func (fs sftpTargetFilesystem) Open(path string) (targetFile, error) {
	client, err := fs.client()
	if err != nil {
		return nil, err
	}
	return client.Open(path)
}
func (fs sftpTargetFilesystem) OpenFile(path string, flag int, _ os.FileMode) (targetFile, error) {
	client, err := fs.client()
	if err != nil {
		return nil, err
	}
	return client.OpenFile(path, flag)
}
func (fs sftpTargetFilesystem) Chmod(path string, mode os.FileMode) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.Chmod(path, mode)
}
func (fs sftpTargetFilesystem) Chtimes(path string, atime, mtime time.Time) error {
	client, err := fs.client()
	if err != nil {
		return err
	}
	return client.Chtimes(path, atime, mtime)
}
func (fs sftpTargetFilesystem) Glob(pattern string) ([]string, error) {
	client, err := fs.client()
	if err != nil {
		return nil, err
	}
	return client.Glob(pattern)
}
func (fs sftpTargetFilesystem) EvalSymlinks(path string) (string, error) {
	client, err := fs.client()
	if err != nil {
		return "", err
	}
	return client.RealPath(path)
}

var resolveTargetFilesystem = func() (targetFilesystem, error) {
	switch settings.NginxSettings.ControlMode() {
	case settings.ControlModeLocal, settings.ControlModeExternalContainer:
		return localTargetFilesystem{}, nil
	case settings.ControlModeHostViaSSH:
		switch settings.NginxSettings.HostAccessMode {
		case settings.HostAccessModeSFTP:
			return sftpTargetFilesystem{}, nil
		case settings.HostAccessModeMounted:
			return localTargetFilesystem{}, nil
		default:
			return nil, ErrInvalidTargetFilesystem
		}
	default:
		return nil, ErrInvalidTargetFilesystem
	}
}

func targetFS() (targetFilesystem, error) { return resolveTargetFilesystem() }

func UsesSFTPTarget() (bool, error) {
	switch settings.NginxSettings.ControlMode() {
	case settings.ControlModeLocal, settings.ControlModeExternalContainer:
		return false, nil
	case settings.ControlModeHostViaSSH:
		switch settings.NginxSettings.HostAccessMode {
		case settings.HostAccessModeSFTP:
			return true, nil
		case settings.HostAccessModeMounted:
			return false, nil
		default:
			return false, ErrInvalidTargetFilesystem
		}
	default:
		return false, ErrInvalidTargetFilesystem
	}
}

func ReadFile(path string) ([]byte, error) {
	fs, err := targetFS()
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(path)
}

func WriteFile(path string, data []byte, mode os.FileMode) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.WriteFile(path, data, mode)
}

func ReadDir(path string) ([]os.DirEntry, error) {
	fs, err := targetFS()
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(path)
}

func Stat(path string) (os.FileInfo, error) {
	fs, err := targetFS()
	if err != nil {
		return nil, err
	}
	return fs.Stat(path)
}

func Exists(path string) (bool, error) {
	_, err := Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// EntryExists reports whether any directory entry lives at path, including a
// dangling symlink. Exists follows symlinks and misses those, SymlinkExists
// misses regular files.
func EntryExists(path string) (bool, error) {
	_, err := Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func SymlinkExists(path string) (bool, error) {
	info, err := Lstat(path)
	if err == nil {
		return info.Mode()&os.ModeSymlink != 0, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func Lstat(path string) (os.FileInfo, error) {
	fs, err := targetFS()
	if err != nil {
		return nil, err
	}
	return fs.Lstat(path)
}

func Readlink(path string) (string, error) {
	fs, err := targetFS()
	if err != nil {
		return "", err
	}
	return fs.Readlink(path)
}

func Symlink(oldname, newname string) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.Symlink(oldname, newname)
}

func Remove(path string) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.Remove(path)
}

func RemoveAll(path string) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.RemoveAll(path)
}

func Rename(oldpath, newpath string) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.Rename(oldpath, newpath)
}

func Mkdir(path string, mode os.FileMode) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.Mkdir(path, mode)
}

func MkdirAll(path string, mode os.FileMode) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.MkdirAll(path, mode)
}

func Open(path string) (targetFile, error) {
	fs, err := targetFS()
	if err != nil {
		return nil, err
	}
	return fs.Open(path)
}

func OpenFile(path string, flag int, mode os.FileMode) (targetFile, error) {
	fs, err := targetFS()
	if err != nil {
		return nil, err
	}
	return fs.OpenFile(path, flag, mode)
}

func Chmod(path string, mode os.FileMode) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.Chmod(path, mode)
}

func Chtimes(path string, atime, mtime time.Time) error {
	fs, err := targetFS()
	if err != nil {
		return err
	}
	return fs.Chtimes(path, atime, mtime)
}

func Glob(pattern string) ([]string, error) {
	fs, err := targetFS()
	if err != nil {
		return nil, err
	}
	return fs.Glob(pattern)
}

func EvalSymlinks(path string) (string, error) {
	fs, err := targetFS()
	if err != nil {
		return "", err
	}
	return fs.EvalSymlinks(path)
}
