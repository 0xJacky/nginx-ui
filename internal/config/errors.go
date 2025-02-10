package config

import "github.com/uozi-tech/cosy"

var (
	e                                = cosy.NewErrorScope("config")
	ErrPathIsNotUnderTheNginxConfDir = e.New(50006, "path: {0} is not under the nginx conf dir: {1}")
)
