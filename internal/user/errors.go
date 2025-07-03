package user

import "github.com/uozi-tech/cosy"

var (
	e                          = cosy.NewErrorScope("user")
	ErrPasswordIncorrect       = e.New(40301, "password incorrect")
	ErrUserBanned              = e.New(40303, "user banned")
	ErrOTPCode                 = e.New(40304, "invalid otp code")
	ErrRecoveryCode            = e.New(40305, "invalid recovery code")
	ErrTOTPNotEnabled          = e.New(40306, "legacy recovery code not allowed since totp is not enabled")
	ErrWebAuthnNotConfigured   = e.New(50000, "WebAuthn settings are not configured")
	ErrUserNotEnabledOTPAs2FA  = e.New(50001, "user not enabled otp as 2fa")
	ErrOTPOrRecoveryCodeEmpty  = e.New(50002, "otp or recovery code empty")
	ErrCannotRemoveInitUser    = e.New(50003, "cannot remove initial user")
	ErrChangeInitUserPwdInDemo = e.New(50004, "cannot change initial user password in demo mode")
	ErrSessionNotFound         = e.New(40401, "session not found")
	ErrTokenIsEmpty            = e.New(40402, "token is empty")
	ErrInvalidClaimsType       = e.New(50005, "invalid claims type")
)
