package config

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
)

// A config change another node replicated here is applied locally only. The
// sync functions must return before resolving any target, since the node where
// the change was made already fanned it out.
func TestReplicatedConfigChangesAreNotForwarded(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/configs", nil)
	request.Header.Set(nodeauth.ReplicatedFromHeader, `"origin-instance"`)
	request = nodeauth.WithPrincipal(request, &nodeauth.Principal{AuthMethod: model.NodeAuthMethodLegacy})
	ctx := request.Context()

	// Paths outside the Nginx configuration directory would be rejected, and
	// no database is set up: reaching either means the change was forwarded.
	outside := "/definitely/not/under/the/nginx/conf/dir.conf"
	require.NoError(t, SyncToRemoteServer(ctx, &model.Config{Filepath: outside, SyncNodeIds: []uint64{1}}, ""))
	require.NoError(t, SyncRenameOnRemoteServer(ctx, outside, outside+".new", []uint64{1}))
	require.NoError(t, SyncDeleteOnRemoteServer(ctx, outside, []uint64{1}))
}
