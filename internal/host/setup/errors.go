package setup

import "github.com/uozi-tech/cosy"

var e = cosy.NewErrorScope("host_setup")

var (
	ErrTemplateRender = e.New(520001, "failed to render template {0}: {1}")
	ErrKeygenFailed   = e.New(520002, "failed to generate keypair: {0}")
	ErrKeyfileWrite   = e.New(520003, "failed to write key file {0}: {1}")
	ErrKeyfileRead    = e.New(520004, "failed to read key file {0}: {1}")
	ErrVerifyStep     = e.New(520005, "verify step {0} failed: {1}")
)
