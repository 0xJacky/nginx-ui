package config

import "github.com/uozi-tech/cosy"

var (
	e                                 = cosy.NewErrorScope("config")
	ErrPathIsNotUnderTheNginxConfDir  = e.New(50006, "path: {0} is not under the nginx conf dir: {1}")
	ErrDstFileExists                  = e.New(50007, "destination file: {0} already exists")
	ErrNginxTestFailed                = e.New(50008, "nginx test failed: {0}")
	ErrNginxReloadFailed              = e.New(50009, "nginx reload failed: {0}")
	ErrCannotDeleteProtectedPath      = e.New(50010, "cannot delete protected path")
	ErrFileNotFound                   = e.New(50011, "file or directory not found: {0}")
	ErrDeletePathNotUnderNginxConfDir = e.New(50012, "you are not allowed to delete a file outside of the nginx config path")
)
