package upgrader

import "github.com/uozi-tech/cosy"

var (
	e                        = cosy.NewErrorScope("upgrader")
	ErrDownloadUrlEmpty      = e.New(52001, "upgrader core downloadUrl is empty")
	ErrDigestEmpty           = e.New(52002, "upgrader core digest is empty")
	ErrDigestFileEmpty       = e.New(52003, "digest file content is empty")
	ErrExecutableBinaryEmpty = e.New(52004, "executable binary file is empty")
	ErrUpdateInProgress      = e.New(52005, "update already in progress")
)
