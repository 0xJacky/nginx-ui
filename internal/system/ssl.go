package system

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy"
)

// ValidateSSLCertificates checks if SSL certificate and key files exist and are under Nginx config directory
// Returns nil if valid, or a CosyError if invalid
func ValidateSSLCertificates(sslCert, sslKey string) error {
	// Check if both paths are provided
	if sslCert == "" {
		return ErrSSLCertRequired
	}

	if sslKey == "" {
		return ErrSSLKeyRequired
	}

	// Get Nginx configuration directory
	nginxConfPath := nginx.GetConfPath()

	// Check if certificate file exists and is under Nginx config directory
	if !helper.IsUnderDirectory(sslCert, nginxConfPath) {
		return cosy.WrapErrorWithParams(ErrSSLCertNotUnderConf, nginxConfPath)
	}

	// Check if certificate file exists
	if _, err := os.Stat(sslCert); os.IsNotExist(err) {
		return ErrSSLCertNotFound
	}

	// Check if key file is under Nginx config directory
	if !helper.IsUnderDirectory(sslKey, nginxConfPath) {
		return cosy.WrapErrorWithParams(ErrSSLKeyNotUnderConf, nginxConfPath)
	}

	// Check if key file exists
	if _, err := os.Stat(sslKey); os.IsNotExist(err) {
		return ErrSSLKeyNotFound
	}

	return nil
}
