package middleware

import "github.com/uozi-tech/cosy"

var (
	e                       = cosy.NewErrorScope("middleware")
	ErrInvalidRequestFormat = e.New(40000, "invalid request format")
	ErrDecryptionFailed     = e.New(40001, "decryption failed")
	ErrFormParseFailed      = e.New(40002, "form parse failed")
	ErrDisabledInDemo       = e.New(40300, "this action is disabled in demo mode")
	ErrRemoteFeedHasSecrets = e.New(40301, "this remote node feed carries stored secrets and requires a token with write scope")
)
