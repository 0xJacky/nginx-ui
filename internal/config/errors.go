package config

import "github.com/uozi-tech/cosy"

var (
	e                                = cosy.NewErrorScope("config")
	ErrPathIsNotUnderTheNginxConfDir = e.New(50006, "path: {0} is not under the nginx conf dir: {1}")
	ErrDstFileExists                 = e.New(50007, "destination file: {0} already exists")
	ErrNginxTestFailed               = e.New(50008, "nginx test failed: {0}")
	ErrNginxReloadFailed             = e.New(50009, "nginx reload failed: {0}")
)
