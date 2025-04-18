package site

import "github.com/uozi-tech/cosy"

var (
	e                      = cosy.NewErrorScope("site")
	ErrSiteNotFound        = e.New(40401, "site not found")
	ErrDstFileExists       = e.New(50001, "destination file already exists")
	ErrSiteIsEnabled       = e.New(50002, "site is enabled")
	ErrSiteIsInMaintenance = e.New(50003, "site is in maintenance mode")
	ErrNginxTestFailed     = e.New(50004, "nginx test failed: {0}")
	ErrNginxReloadFailed   = e.New(50005, "nginx reload failed: {0}")
)
