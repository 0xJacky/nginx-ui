package sign

import "github.com/uozi-tech/cosy"

var (
	e                        = cosy.NewErrorScope("sign")
	ErrTimeout               = e.New(40401, "request timeout")
	ErrInvalidNonce          = e.New(50000, "invalid nonce")
	ErrDecodePrivateKey      = e.New(50001, "failed to decode private key")
	ErrInvalidSign           = e.New(50002, "invalid signature")
	ErrEncryptedDataTooShort = e.New(50003, "encrypted data too short")
)
