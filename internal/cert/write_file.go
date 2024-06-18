package cert

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"os"
	"path/filepath"
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
		return fmt.Errorf("ssl_certificate_path: %s is not under the nginx conf path: %s",
			c.SSLCertificatePath, nginxConfPath)
	}

	if !helper.IsUnderDirectory(c.SSLCertificateKeyPath, nginxConfPath) {
		return fmt.Errorf("ssl_certificate_key_path: %s is not under the nginx conf path: %s",
			c.SSLCertificateKeyPath, nginxConfPath)
	}

	// MkdirAll creates a directory named path, along with any necessary parents,
	// and returns nil, or else returns an error.
	// The permission bits perm (before umask) are used for all directories that MkdirAll creates.
	// If path is already a directory, MkdirAll does nothing and returns nil.

	err = os.MkdirAll(filepath.Dir(c.SSLCertificatePath), 0644)
	if err != nil {
		return
	}

	err = os.MkdirAll(filepath.Dir(c.SSLCertificateKeyPath), 0644)
	if err != nil {
		return
	}

	if c.SSLCertificate != "" {
		err = os.WriteFile(c.SSLCertificatePath, []byte(c.SSLCertificate), 0644)
		if err != nil {
			return
		}
	}

	if c.SSLCertificateKey != "" {
		err = os.WriteFile(c.SSLCertificateKeyPath, []byte(c.SSLCertificateKey), 0644)
		if err != nil {
			return
		}
	}

	return
}
