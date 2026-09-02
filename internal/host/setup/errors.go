package setup

import (
	"errors"

	"github.com/uozi-tech/cosy"
)

var ErrInvalidPrivateKey = errors.New("invalid private key")

var ErrInvalidHostUser = errors.New("ssh user must match ^[a-z_][a-z0-9_-]*$ and be at most 32 characters")

var e = cosy.NewErrorScope("host_setup")

var (
	ErrUnsafeSnippetValue = e.New(520007, "{0} contains characters that are unsafe to paste into the generated host instructions: {1}")
	ErrInvalidPublicKey   = e.New(520008, "public key must be a single valid OpenSSH key line")
	ErrInvalidAccessMode  = e.New(520009, "access mode must be either sftp or mounted")
	ErrInvalidHostAddress = e.New(520010, "host address must be a hostname or IP with an optional port: {0}")
)

var (
	ErrTemplateRender  = e.New(520001, "failed to render template {0}: {1}")
	ErrKeygenFailed    = e.New(520002, "failed to generate keypair: {0}")
	ErrKeyfileWrite    = e.New(520003, "failed to write key file {0}: {1}")
	ErrKeyfileRead     = e.New(520004, "failed to read key file {0}: {1}")
	ErrVerifyStep      = e.New(520005, "verify step {0} failed: {1}")
	ErrDiscoveryFailed = e.New(520006, "failed to discover host nginx: {0}")
)
