package cert

import (
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
