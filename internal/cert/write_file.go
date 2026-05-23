package cert

import (
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

type Content struct {
	SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
	SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
	SSLCertificate        string `json:"ssl_certificate"`
	SSLCertificateKey     string `json:"ssl_certificate_key"`
}

func (c *Content) WriteFile() (err error) {
	if c.SSLCertificatePath == "" || c.SSLCertificateKeyPath == "" {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	if !helper.IsUnderDirectory(c.SSLCertificatePath, nginxConfPath) {
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), c.SSLCertificatePath, nginxConfPath)
	}

	if !helper.IsUnderDirectory(c.SSLCertificateKeyPath, nginxConfPath) {
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), c.SSLCertificateKeyPath, nginxConfPath)
	}

	err = os.MkdirAll(filepath.Dir(c.SSLCertificatePath), 0755)
	if err != nil {
		return
	}

	err = os.MkdirAll(filepath.Dir(c.SSLCertificateKeyPath), 0755)
	if err != nil {
		return
	}

	if err = ensureWritableFileTarget(c.SSLCertificatePath); err != nil {
		return
	}
	if err = ensureWritableFileTarget(c.SSLCertificateKeyPath); err != nil {
		return
	}

	tmpFiles := make(map[string]string, 2)
	defer func() {
		for _, tmpPath := range tmpFiles {
			_ = os.Remove(tmpPath)
		}
	}()

	if c.SSLCertificate != "" {
		if tmpFiles[c.SSLCertificatePath], err = writeTempFileNextTo(c.SSLCertificatePath, []byte(c.SSLCertificate), 0644); err != nil {
			return
		}
	}

	if c.SSLCertificateKey != "" {
		if tmpFiles[c.SSLCertificateKeyPath], err = writeTempFileNextTo(c.SSLCertificateKeyPath, []byte(c.SSLCertificateKey), 0600); err != nil {
			return
		}
	}

	for targetPath, tmpPath := range tmpFiles {
		if err = replaceFile(tmpPath, targetPath); err != nil {
			return
		}
		delete(tmpFiles, targetPath)
	}

	return
}

func ensureWritableFileTarget(path string) error {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return &os.PathError{Op: "write", Path: path, Err: os.ErrInvalid}
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writeTempFileNextTo(path string, content []byte, perm os.FileMode) (string, error) {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	if _, err = tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err = tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err = tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}

func replaceFile(tmpPath, targetPath string) error {
	if err := os.Rename(tmpPath, targetPath); err == nil {
		return nil
	}

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, targetPath)
}
