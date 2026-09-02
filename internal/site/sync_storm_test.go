package site

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

type syncRequestCounts struct {
	save   [2]int32
	enable [2]int32
}

func TestSyncSaveEnablesEachSuccessfulNodeOnce(t *testing.T) {
	counts := runSyncSaveRequestCountTest(t, [2]int{http.StatusOK, http.StatusOK})

	for index := range counts.save {
		require.EqualValues(t, 1, counts.save[index], "node %d save requests", index+1)
		require.EqualValues(t, 1, counts.enable[index], "node %d enable requests", index+1)
	}
}

func TestSyncSaveDoesNotEnableNodeWhoseSaveFailed(t *testing.T) {
	counts := runSyncSaveRequestCountTest(t, [2]int{http.StatusOK, http.StatusInternalServerError})

	require.EqualValues(t, [2]int32{1, 1}, counts.save)
	require.EqualValues(t, [2]int32{1, 0}, counts.enable)
}

func runSyncSaveRequestCountTest(t *testing.T, saveStatuses [2]int) syncRequestCounts {
	t.Helper()

	confDir, _ := setupSiteMutationTest(t)
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
	appsettings.CryptoSettings.Secret = "site-sync-storm-test-root"
	appsettings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"

	var saveRequests [2]atomic.Int32
	var enableRequests [2]atomic.Int32
	newNodeServer := func(index int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Content-Type", "application/json")
			switch request.URL.Path {
			case "/api/sites/storm.example.com":
				saveRequests[index].Add(1)
				response.WriteHeader(saveStatuses[index])
			case "/api/sites/storm.example.com/enable":
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

	name := "storm.example.com"
	path := filepath.Join(confDir, "sites-available", name)
	content := "server { return 200; }\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	require.NoError(t, database.Create(&model.Site{
		Path:          path,
		NamespaceID:   namespace.ID,
		RemoteEnabled: true,
	}).Error)

	syncSave(name, content)

	counts := syncRequestCounts{}
	for index := range servers {
		counts.save[index] = saveRequests[index].Load()
		counts.enable[index] = enableRequests[index].Load()
	}
	return counts
}
