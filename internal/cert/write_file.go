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

	// MkdirAll creates a directory named path, along with any necessary parents,
	// and returns nil, or else returns an error.
	// The permission bits perm (before umask) are used for all directories that MkdirAll creates.
	// If path is already a directory, MkdirAll does nothing and returns nil.

	err = os.MkdirAll(filepath.Dir(c.SSLCertificatePath), 0755)
	if err != nil {
		return
	}

	err = os.MkdirAll(filepath.Dir(c.SSLCertificateKeyPath), 0755)
	if err != nil {
		return
	}

	if c.SSLCertificate != "" {
		err = os.WriteFile(c.SSLCertificatePath, []byte(c.SSLCertificate), 0755)
		if err != nil {
			return
		}
	}

	if c.SSLCertificateKey != "" {
		err = os.WriteFile(c.SSLCertificateKeyPath, []byte(c.SSLCertificateKey), 0755)
		if err != nil {
			return
		}
	}

	return
}
