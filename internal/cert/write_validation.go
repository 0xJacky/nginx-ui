package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

// ValidateSync requires a complete matching pair before a sync creates records,
// files, or directories. Ordinary edits may omit one unchanged half of the pair.
func (c *Content) ValidateSync() error {
	if c.SSLCertificate == "" || c.SSLCertificateKey == "" {
		return ErrCertificatePathsRequired
	}
	return c.validateWrite()
}

func (c *Content) validateWrite() error {
	certPath, err := validateCertificateWritePath(c.SSLCertificatePath)
	if err != nil {
		return err
	}
	keyPath, err := validateCertificateWritePath(c.SSLCertificateKeyPath)
	if err != nil {
		return err
	}
	if certPath == keyPath {
		return ErrCertificatePathsRequired
	}
	certificate, privateKey := []byte(c.SSLCertificate), []byte(c.SSLCertificateKey)
	if len(certificate) == 0 {
		certificate, err = nginx.ReadFile(c.SSLCertificatePath)
		if err != nil {
			return err
		}
	}
	if len(privateKey) == 0 {
		privateKey, err = nginx.ReadFile(c.SSLCertificateKeyPath)
		if err != nil {
			return err
		}
	}
	blocks, err := config.ParsePEMData(certificate)
	if err != nil || len(blocks) == 0 {
		return ErrCertDecode
	}
	for _, block := range blocks {
		if block.Type != "CERTIFICATE" {
			return ErrCertDecode
		}
		if _, err = x509.ParseCertificate(block.Bytes); err != nil {
			return ErrCertParse
		}
	}
	blocks, err = config.ParsePEMData(privateKey)
	if err != nil || len(blocks) == 0 {
		return cosy.WrapErrorWithParams(ErrInvalidPrivateKey, c.SSLCertificateKeyPath)
	}
	keyCount := 0
	for _, block := range blocks {
		switch block.Type {
		case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
			keyCount++
		case "EC PARAMETERS":
			// OpenSSL may emit the named-curve parameters before an EC key.
		default:
			return cosy.WrapErrorWithParams(ErrInvalidPrivateKey, c.SSLCertificateKeyPath)
		}
	}
	if keyCount != 1 {
		return cosy.WrapErrorWithParams(ErrInvalidPrivateKey, c.SSLCertificateKeyPath)
	}
	if _, err = tls.X509KeyPair(certificate, privateKey); err != nil {
		return cosy.WrapErrorWithParams(ErrCertificateKeyMismatch, err.Error())
	}
	return nil
}

// resolveCertificateWritePath uses the target filesystem, including SFTP, rather
// than resolving a remote symlink against the controller's unrelated local disk.
func resolveCertificateWritePath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", ErrCertPathIsEmpty
	}
	current := filepath.Clean(path)
	var tail []string
	for {
		if _, err := nginx.Lstat(current); err == nil {
			resolved, err := nginx.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return filepath.Clean(resolved), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("cannot resolve certificate path %s", path)
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}

func validateCertificateWritePath(path string) (string, error) {
	resolved, err := resolveCertificateWritePath(path)
	if err != nil {
		return "", err
	}
	root, err := resolveCertificateWritePath(nginx.GetConfPath())
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", ErrCertPathIsNotUnderTheNginxConfDir
	}
	first := strings.ToLower(strings.SplitN(filepath.ToSlash(relative), "/", 2)[0])
	switch first {
	case "sites-available", "sites-enabled", "streams-available", "streams-enabled", "modules-enabled":
		return "", cosy.WrapErrorWithParams(ErrInvalidCertificate, path, "path is reserved for nginx configuration")
	}
	if strings.EqualFold(filepath.Ext(resolved), ".conf") {
		return "", cosy.WrapErrorWithParams(ErrInvalidCertificate, path, "path is reserved for nginx configuration")
	}
	if settings.NginxSettings.ConfigPath != "" {
		entryPath, entryErr := resolveCertificateWritePath(settings.NginxSettings.ConfigPath)
		if entryErr == nil && entryPath == resolved {
			return "", cosy.WrapErrorWithParams(ErrInvalidCertificate, path, "path is reserved for nginx configuration")
		}
	}
	switch strings.ToLower(filepath.Base(resolved)) {
	case "nginx.conf", "mime.types", "fastcgi_params", "fastcgi.conf", "scgi_params", "uwsgi_params", "proxy_params", "koi-utf", "koi-win", "win-utf":
		return "", cosy.WrapErrorWithParams(ErrInvalidCertificate, path, "path is reserved for nginx configuration")
	}
	// Existing custom names remain supported, but unrelated files are never
	// overwritten. Recognized certificate extensions can repair damaged files.
	if existing, readErr := nginx.ReadFile(path); readErr == nil {
		switch strings.ToLower(filepath.Ext(resolved)) {
		case ".pem", ".crt", ".cer", ".key":
		default:
			if !containsCertificateMaterial(existing) {
				return "", cosy.WrapErrorWithParams(ErrInvalidCertificate, path, "existing file is not a certificate or private key")
			}
		}
	} else if !os.IsNotExist(readErr) {
		return "", readErr
	}
	return resolved, nil
}

func containsCertificateMaterial(content []byte) bool {
	blocks, err := config.ParsePEMData(content)
	if err != nil {
		return false
	}
	for _, block := range blocks {
		encoded := string(pem.EncodeToMemory(block))
		if IsCertificate(encoded) || IsPrivateKey(encoded) {
			return true
		}
	}
	return false
}
