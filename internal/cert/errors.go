package cert

import "github.com/uozi-tech/cosy"

var (
	e                                    = cosy.NewErrorScope("cert")
	ErrCertModelFilenameEmpty            = e.New(50001, "filename is empty")
	ErrCertPathIsNotUnderTheNginxConfDir = e.New(50002, "cert path is not under the nginx conf dir")
	ErrCertDecode                        = e.New(50003, "certificate decode error")
	ErrCertParse                         = e.New(50004, "certificate parse error")
	ErrPayloadResourceIsNil              = e.New(50005, "payload resource is nil")
	ErrPathIsNotUnderTheNginxConfDir     = e.New(50006, "path: {0} is not under the nginx conf dir: {1}")
	ErrCertPathIsEmpty                   = e.New(50007, "certificate path is empty")
)
