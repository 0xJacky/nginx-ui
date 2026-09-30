package plugin

import (
	"errors"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/uozi-tech/cosy"
)

var (
	e = cosy.NewErrorScope("plugin")

	ErrManifestInvalid            = e.New(55001, "plugin manifest is invalid")
	ErrPluginNotFound             = e.New(55002, "plugin not found")
	ErrPluginAlreadyExists        = e.New(55003, "plugin already exists")
	ErrPluginNotRunning           = e.New(55004, "plugin is not running")
	ErrPluginHandshake            = e.New(55005, "plugin handshake failed")
	ErrNoExecutableForPlatform    = e.New(55006, "plugin has no executable for this platform")
	ErrPackageInvalid             = e.New(55007, "plugin package is invalid")
	ErrPackageTooLarge            = e.New(55008, "plugin package is too large")
	ErrIncompatibleAPIVersion     = e.New(55009, "plugin api version is incompatible")
	ErrPermissionApprovalRequired = e.New(55010, "plugin permissions need approval")
	ErrDependencyMissing          = e.New(55011, "plugin dependency is missing: {0}")
	ErrDependencyCycle            = e.New(55012, "plugin dependencies form a cycle")
	ErrPluginInUse                = e.New(55013, "plugin is in use by {0}")
	ErrPluginsDisabled            = e.New(55014, "plugin system is disabled")
	ErrUploadsDisabled            = e.New(55015, "plugin uploads are disabled")
	ErrCallTimeout                = e.New(55016, "plugin call timed out")
	ErrRPC                        = e.New(55017, "plugin rpc error: {0}")
	ErrSettingsInvalid            = e.New(55018, "plugin setting {0} is invalid")
	ErrHostVersionTooOld          = e.New(55019, "plugin needs nginx-ui {0} or newer, this node runs {1}")
	ErrPluginIDMismatch           = e.New(55020, "plugin package declares another id")
	ErrContentInvalid             = e.New(55021, "plugin content {0} is invalid: {1}")
	ErrPluginVersionMismatch      = e.New(55022, "plugin package declares version {0}, {1} was expected")
	ErrUnsignedPackage            = e.New(55023, "plugin package is not signed and developer mode is off")
	ErrTrustDowngrade             = e.New(55024, "plugin package trust {0} is below the installed {1}")
	ErrPluginConflict             = e.New(55025, "plugin conflicts with the enabled plugin(s): {0}")

	// Capability errors continue at 55201, apart from the lifecycle and the
	// marketplace codes.
	ErrStorageBackendUnavailable = e.New(55201, "storage backend {0} is not available")
	ErrStorageConfigInvalid      = e.New(55202, "storage config field {0} is invalid: {1}")
	ErrDeployKindUnavailable     = e.New(55203, "deploy target kind {0} is not available")
	ErrDeployConfigInvalid       = e.New(55204, "deploy target config field {0} is invalid: {1}")
	ErrBlocklistKindUnavailable  = e.New(55205, "blocklist source kind {0} is not available")
	ErrBlocklistConfigInvalid    = e.New(55206, "blocklist source config field {0} is invalid: {1}")
	ErrDiscoveryKindUnavailable  = e.New(55207, "discovery provider {0} is not available")
	ErrDiscoveryConfigInvalid    = e.New(55208, "discovery provider config field {0} is invalid: {1}")
	ErrRefreshIntervalInvalid    = e.New(55209, "refresh interval must be at least {0} seconds")
	ErrUpstreamNameInvalid       = e.New(55210, "upstream name {0} is invalid")
	ErrUpstreamNameTaken         = e.New(55211, "upstream name {0} is already bound to another service")
	ErrExtraDirectivesInvalid    = e.New(55212, "extra directives must not contain braces")
	ErrGeneratedFileIncluded     = e.New(55213, "{0} is still included by the nginx configuration: {1}")
)

// rpcError carries both the cosy error the API layer reports and the original
// JSON-RPC error so callers can still inspect the code.
type rpcError struct {
	cosyErr error
	rpcErr  *protocol.Error
}

func (r *rpcError) Error() string { return r.cosyErr.Error() }

// Unwrap exposes the parameterised cosy error, the ErrRPC sentinel and the
// original JSON-RPC error so errors.Is and errors.As reach all of them.
func (r *rpcError) Unwrap() []error { return []error{r.cosyErr, ErrRPC, r.rpcErr} }

// WrapRPCError converts a JSON-RPC error into a cosy error carrying the peer
// message. Errors that do not come from the wire are returned untouched.
func WrapRPCError(err error) error {
	if err == nil {
		return nil
	}
	var perr *protocol.Error
	if !errors.As(err, &perr) {
		return err
	}
	return &rpcError{
		cosyErr: e.NewWithParams(55017, ErrRPC.Error(), perr.Message),
		rpcErr:  perr,
	}
}
