package plugin

import "github.com/uozi-tech/cosy"

// marketplaceErrors shares the plugin scope. It is declared here too so the
// error code generator, which reads one file at a time, finds these codes.
var marketplaceErrors = cosy.NewErrorScope("plugin")

// Marketplace specific errors. They continue the plugin scope at 55101 so the
// installer codes stay apart from the lifecycle ones.
var (
	ErrMarketplaceDisabled  = marketplaceErrors.New(55101, "plugin marketplace is disabled")
	ErrSourceUnavailable    = marketplaceErrors.New(55102, "plugin marketplace source is unavailable: {0}")
	ErrReleaseNotFound      = marketplaceErrors.New(55103, "plugin release not found")
	ErrReleaseYanked        = marketplaceErrors.New(55104, "plugin release has been yanked")
	ErrDigestMismatch       = marketplaceErrors.New(55105, "plugin package digest does not match the catalog")
	ErrSignatureInvalid     = marketplaceErrors.New(55107, "plugin package signature is invalid: {0}")
	ErrCommunityNotAllowed  = marketplaceErrors.New(55108, "community plugins are not allowed on this node")
	ErrInsecureURL          = marketplaceErrors.New(55109, "plugin download url is not https")
	ErrPlatformUnsupported  = marketplaceErrors.New(55110, "plugin release has no build for this platform")
	ErrCatalogInvalid       = marketplaceErrors.New(55111, "plugin catalog is invalid: {0}")
	ErrMarketplaceNotFound  = marketplaceErrors.New(55112, "plugin is not in the marketplace catalog")
	ErrLocalPackageNotFound = marketplaceErrors.New(55113, "no local package found for this plugin")
	// ErrPlatformPackageMissing names the platform nothing was found for.
	ErrPlatformPackageMissing = marketplaceErrors.New(55114, "no plugin package is available for {0}")
	// ErrReplaceUnavailable refuses a replacement that would not raise the trust.
	ErrReplaceUnavailable = marketplaceErrors.New(55115, "the marketplace has no more trusted package of this plugin")
)
