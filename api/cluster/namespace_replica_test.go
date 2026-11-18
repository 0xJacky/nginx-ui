package cluster

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNamespaceRenameAcrossMemberUpgrade(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Namespace{}))
	// Name-only replica left by the old protocol with member-local settings.
	member := model.Namespace{Name: "A", SyncNodeIds: []uint64{8}, DeployMode: model.DeployModeRemote}
	require.NoError(t, db.Create(&member).Error)
	payload := namespaceSyncRequest{Name: "B", OriginalName: "A", PostSyncAction: model.PostSyncActionNone}
	const source = "controller/42"
	for _, name := range []string{"B", "B", "C"} {
		payload.Name = name
		got, err := upsertNamespace(db, payload, source)
		require.NoError(t, err)
		require.Equal(t, member.ID, got.ID)
		require.Equal(t, name, got.Name)
	}
	var rows []model.Namespace
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "C", rows[0].Name)
	require.Equal(t, member.SyncNodeIds, rows[0].SyncNodeIds)
	require.Equal(t, member.DeployMode, rows[0].DeployMode)
	require.Equal(t, model.PostSyncActionNone, rows[0].PostSyncAction)
	require.NoError(t, db.Create(&model.Namespace{Name: "D"}).Error)
	payload.Name = "D"
	_, err = upsertNamespace(db, payload, source)
	require.ErrorIs(t, err, errNamespaceConflict)
	payload.Name = "C"
	_, err = upsertNamespace(db, payload, "other-controller/42")
	require.ErrorIs(t, err, errNamespaceConflict)
	require.NoError(t, db.First(&member, member.ID).Error)
	require.Equal(t, "C", member.Name)
}

func TestNamespaceLegacyNamesFollowOnlyExistingMembers(t *testing.T) {
	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Namespace{})
	db := cosy.InitDB(sqlite.Open(testdb.DSN(t)))
	router := gin.New()
	router.POST("/namespaces/:id", ModifyNamespace)
	a := model.Namespace{Name: "A", SyncNodeIds: []uint64{1, 2}}
	require.NoError(t, db.Create(&a).Error)
	rename := func(body string) model.Namespace {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/namespaces/%d", a.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var saved model.Namespace
		require.NoError(t, db.First(&saved, a.ID).Error)
		return saved
	}
	b := rename(`{"name":"B","sync_node_ids":[2,3]}`)
	require.Equal(t, map[uint64]string{2: "A"}, b.LegacyMemberNames)
	c := rename(`{"name":"C"}`)
	require.Equal(t, map[uint64]string{2: "A", 3: "B"}, c.LegacyMemberNames)
}
