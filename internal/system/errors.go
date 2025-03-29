package system

import "github.com/uozi-tech/cosy"

// System error definitions
var (
	e                 = cosy.NewErrorScope("system")
	ErrInstallTimeout = e.New(40308, "installation is not allowed after 10 minutes of system startup")
)
