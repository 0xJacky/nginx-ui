package upgrader

import "github.com/uozi-tech/cosy"

var (
	e                              = cosy.NewErrorScope("upgrader")
	ErrDownloadUrlEmpty            = e.New(52001, "upgrader core downloadUrl is empty")
	ErrDigestEmpty                 = e.New(52002, "upgrader core digest is empty")
	ErrDigestFileEmpty             = e.New(52003, "digest file content is empty")
	ErrExecutableBinaryEmpty       = e.New(52004, "executable binary file is empty")
	ErrUpdateInProgress            = e.New(52005, "update already in progress")
	ErrSignatureEmpty              = e.New(52006, "release signature is missing")
	ErrSignatureInvalid            = e.New(52007, "release signature is invalid")
	ErrSignatureKeyUnknown         = e.New(52008, "release signature key is not trusted")
	ErrTrustedSignatureKeysEmpty   = e.New(52009, "no trusted release signing keys are embedded")
	ErrTrustedSignatureKeysInvalid = e.New(52010, "embedded release signing keys are invalid")
	ErrDigestMismatch              = e.New(52011, "release archive digest does not match")
	ErrReleaseAssetEmpty           = e.New(52012, "upgrader core release asset is missing")
	ErrUnsupportedPlatform         = e.New(52013, "upgrader core platform is unsupported")
)
