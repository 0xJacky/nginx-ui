package system

import "github.com/uozi-tech/cosy"

// System error definitions
var (
	e                 = cosy.NewErrorScope("system")
	ErrInstalled      = e.New(40301, "Nginx UI already installed")
	ErrInstallTimeout = e.New(40302, "installation is not allowed after 10 minutes of system startup")
)
