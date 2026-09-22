package plugin

// Marketplace specific errors. They continue the plugin scope at 55101 so the
// installer codes stay apart from the lifecycle ones.
var (
	ErrMarketplaceDisabled  = e.New(55101, "plugin marketplace is disabled")
	ErrSourceUnavailable    = e.New(55102, "plugin marketplace source is unavailable: {0}")
	ErrReleaseNotFound      = e.New(55103, "plugin release not found")
	ErrReleaseYanked        = e.New(55104, "plugin release has been yanked")
	ErrDigestMismatch       = e.New(55105, "plugin package digest does not match the catalog")
	ErrSignatureMissing     = e.New(55106, "plugin package is not signed")
	ErrSignatureInvalid     = e.New(55107, "plugin package signature is invalid: {0}")
	ErrCommunityNotAllowed  = e.New(55108, "community plugins are not allowed on this node")
	ErrInsecureURL          = e.New(55109, "plugin download url is not https")
	ErrPlatformUnsupported  = e.New(55110, "plugin release has no build for this platform")
	ErrCatalogInvalid       = e.New(55111, "plugin catalog is invalid: {0}")
	ErrMarketplaceNotFound  = e.New(55112, "plugin is not in the marketplace catalog")
	ErrLocalPackageNotFound = e.New(55113, "no local package found for this plugin")
	// ErrPlatformPackageMissing names the platform nothing was found for.
	ErrPlatformPackageMissing = e.New(55114, "no plugin package is available for {0}")
)
