package crypto

import "github.com/uozi-tech/cosy"

var (
	e                     = cosy.NewErrorScope("crypto")
	ErrPlainTextEmpty     = e.New(50001, "plain text is empty")
	ErrCipherTextTooShort = e.New(50002, "cipher text is too short")
)
