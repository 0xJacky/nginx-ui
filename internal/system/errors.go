package system

import "github.com/uozi-tech/cosy"

// System error definitions
var (
	e                 = cosy.NewErrorScope("system")
	ErrInstalled      = e.New(40301, "Nginx UI already installed")
	ErrInstallTimeout = e.New(40302, "installation is not allowed after 10 minutes of system startup")

	ErrSSLCertRequired     = e.New(40303, "SSL certificate path is required when HTTPS is enabled")
	ErrSSLKeyRequired      = e.New(40304, "SSL key path is required when HTTPS is enabled")
	ErrSSLCertNotFound     = e.New(40305, "SSL certificate file not found")
	ErrSSLKeyNotFound      = e.New(40306, "SSL key file not found")
	ErrSSLCertNotUnderConf = e.New(40307, "SSL certificate file must be under Nginx configuration directory: {0}")
	ErrSSLKeyNotUnderConf  = e.New(40308, "SSL key file must be under Nginx configuration directory: {0}")
)
