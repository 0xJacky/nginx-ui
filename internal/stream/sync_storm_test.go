package stream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

func TestSyncSaveEnablesEachSuccessfulNodeOnce(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)
	database := model.UseDB()
	require.NoError(t, database.AutoMigrate(
		&model.Namespace{},
		&model.Node{},
		&model.Notification{},
		&model.ExternalNotify{},
	))

	originalCryptoSecret := appsettings.CryptoSettings.Secret
	originalInstanceID := appsettings.NodeSettings.InstanceID
	t.Cleanup(func() {
		appsettings.CryptoSettings.Secret = originalCryptoSecret
		appsettings.NodeSettings.InstanceID = originalInstanceID
	})
	appsettings.CryptoSettings.Secret = "stream-sync-storm-test-root"
	appsettings.NodeSettings.InstanceID = "22222222-2222-4222-8222-222222222222"

	var saveRequests [2]atomic.Int32
	var enableRequests [2]atomic.Int32
	newNodeServer := func(index int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Content-Type", "application/json")
			switch request.URL.Path {
			case "/api/streams/storm-stream":
				saveRequests[index].Add(1)
			case "/api/streams/storm-stream/enable":
				enableRequests[index].Add(1)
			default:
				http.NotFound(response, request)
				return
			}
			_, _ = response.Write([]byte(`{"message":"ok"}`))
		}))
	}

	servers := []*httptest.Server{newNodeServer(0), newNodeServer(1)}
	for _, server := range servers {
		t.Cleanup(server.Close)
	}

	nodes := make([]*model.Node, 0, len(servers))
	for index, server := range servers {
		node := &model.Node{
			Name:             "node-" + string(rune('1'+index)),
			URL:              server.URL,
			AuthMethod:       model.NodeAuthMethodLegacy,
			CredentialStatus: model.NodeCredentialStatusActive,
			Enabled:          true,
		}
		require.NoError(t, database.Create(node).Error)
		encryptedSecret, err := nodeauth.EncryptPrivateCredential(
			nodeauth.LegacyCredentialPurpose(node.ID),
			[]byte("node-secret"),
		)
		require.NoError(t, err)
		require.NoError(t, database.Model(node).
			Update("encrypted_legacy_secret", encryptedSecret).Error)
		nodes = append(nodes, node)
	}

	namespace := &model.Namespace{
		Name:           "all-node",
		SyncNodeIds:    []uint64{nodes[0].ID, nodes[1].ID},
		PostSyncAction: model.PostSyncActionReloadNginx,
		DeployMode:     model.DeployModeRemote,
	}
	require.NoError(t, database.Create(namespace).Error)

	name := "storm-stream"
	path := filepath.Join(confDir, "streams-available", name)
	content := "server { listen 12345; }\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	require.NoError(t, database.Create(&model.Stream{
		Path:          path,
		NamespaceID:   namespace.ID,
		RemoteEnabled: true,
	}).Error)

	syncSave(name, content)

	for index := range servers {
		require.EqualValues(t, 1, saveRequests[index].Load(), "node %d save requests", index+1)
		require.EqualValues(t, 1, enableRequests[index].Load(), "node %d enable requests", index+1)
	}
}
