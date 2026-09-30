package plugin

import (
	"errors"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

func TestWrapRPCError(t *testing.T) {
	assert.NoError(t, WrapRPCError(nil))

	plain := errors.New("not from the wire")
	assert.Equal(t, plain, WrapRPCError(plain))

	wire := &protocol.Error{Code: protocol.CodePermissionDenied, Message: "permission kv is not granted"}
	wrapped := WrapRPCError(wire)
	require.Error(t, wrapped)
	assert.Equal(t, "plugin rpc error: permission kv is not granted", wrapped.Error())
	assert.ErrorIs(t, wrapped, ErrRPC)

	// The API layer reports the cosy error.
	var cosyErr *cosy.Error
	require.True(t, errors.As(wrapped, &cosyErr))
	assert.Equal(t, int32(55017), cosyErr.Code)
	assert.Equal(t, "plugin", cosyErr.Scope)

	// The original code stays reachable for capability specific handling.
	var wireErr *protocol.Error
	require.True(t, errors.As(wrapped, &wireErr))
	assert.Equal(t, protocol.CodePermissionDenied, wireErr.Code)
}

func TestErrorScopeCodesAreUnique(t *testing.T) {
	scoped := []error{
		ErrManifestInvalid, ErrPluginNotFound, ErrPluginAlreadyExists, ErrPluginNotRunning,
		ErrPluginHandshake, ErrNoExecutableForPlatform, ErrPackageInvalid, ErrPackageTooLarge,
		ErrIncompatibleAPIVersion, ErrPermissionApprovalRequired, ErrDependencyMissing,
		ErrDependencyCycle, ErrPluginInUse, ErrPluginsDisabled, ErrUploadsDisabled,
		ErrCallTimeout, ErrRPC, ErrSettingsInvalid, ErrHostVersionTooOld, ErrPluginIDMismatch,
		ErrContentInvalid, ErrPluginVersionMismatch, ErrUnsignedPackage, ErrTrustDowngrade, ErrPluginConflict,
	}

	seen := make(map[int32]struct{}, len(scoped))
	for _, err := range scoped {
		var cosyErr *cosy.Error
		require.True(t, errors.As(err, &cosyErr))
		assert.Equal(t, "plugin", cosyErr.Scope)
		// The 52000 range belongs to the upgrader, plugins start at 55001.
		assert.GreaterOrEqual(t, cosyErr.Code, int32(55001))
		_, duplicate := seen[cosyErr.Code]
		assert.False(t, duplicate, "duplicate error code %d", cosyErr.Code)
		seen[cosyErr.Code] = struct{}{}
	}
}
