package config

import "github.com/uozi-tech/cosy"

var (
	e                               = cosy.NewErrorScope("config")
	ErrConfigFilenameNotAllowed     = e.New(40014, "file name is not allowed: {0}")
	ErrConfigContentMustBeUTF8Text  = e.New(40015, "file content must be valid UTF-8 text")
	ErrConfigContentHasControlChars = e.New(40016, "file content contains invalid control characters")
	ErrConfigDirectiveNotAllowed    = e.New(40017, "file content contains restricted nginx directive: {0}")
	ErrConfigDataInvalid            = e.New(40018, "file content must be valid PEM or htpasswd data")
	ErrSecretFileHiddenFromReadOnly = e.New(40019, "secret files are hidden from read-only service tokens")
	ErrConfigIncludeGlobUnsupported = e.New(40020, "include pattern uses glob syntax that cannot be validated: {0}")
	// ErrConfigRestrictedDirectiveConfirmRequired asks an interactive
	// administrator to confirm a save that loads code into the Nginx worker.
	// The request repeats with allow_restricted_directives set to apply it.
	ErrConfigRestrictedDirectiveConfirmRequired = e.New(40021, "restricted nginx directives need confirmation: {0} in {1}")
	// ErrConfigRestrictedDirectiveNeedsTwoFactor is returned when the account
	// has no second factor to step up with, so there is nothing to confirm.
	ErrConfigRestrictedDirectiveNeedsTwoFactor = e.New(40022, "saving restricted nginx directives requires two-factor authentication on your account: {0} in {1}")
	ErrPathIsNotUnderTheNginxConfDir           = e.New(50006, "path: {0} is not under the nginx conf dir: {1}")
	ErrDstFileExists                           = e.New(50007, "destination file: {0} already exists")
	ErrNginxTestFailed                         = e.New(50008, "nginx test failed: {0}")
	ErrNginxReloadFailed                       = e.New(50009, "nginx reload failed: {0}")
	ErrCannotDeleteProtectedPath               = e.New(50010, "cannot delete protected path")
	ErrFileNotFound                            = e.New(50011, "file or directory not found: {0}")
	ErrDeletePathNotUnderNginxConfDir          = e.New(50012, "you are not allowed to delete a file outside of the nginx config path")
	ErrCannotDeleteNginxConfDir                = e.New(50013, "cannot delete the nginx config directory")
)
