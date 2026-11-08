package middleware

import "github.com/uozi-tech/cosy"

var (
	e                        = cosy.NewErrorScope("middleware")
	ErrInvalidRequestFormat  = e.New(40000, "invalid request format")
	ErrDecryptionFailed      = e.New(40001, "decryption failed")
	ErrFormParseFailed       = e.New(40002, "form parse failed")
	ErrProxyPathNotCanonical = e.New(40003, "this request path cannot be forwarded to a remote node because it is not in canonical form")
	ErrDisabledInDemo        = e.New(40300, "this action is disabled in demo mode")
	ErrRemoteFeedHasSecrets  = e.New(40301, "this remote node feed carries stored secrets and requires a token with write scope")
	ErrProxyGuardNotEnforced = e.New(40302, "this route cannot be forwarded to a remote node because the node proxy does not enforce its authorization guard")
	ErrProxyNodeDisabled     = e.New(40303, "this request cannot be forwarded to the remote node because the node is disabled")
)
