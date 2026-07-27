package cluster

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteNodeReloadsStatusAfterSuccessfulDeletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Node{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "node-delete.db")))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	query.SetDefault(db)

	node := &model.Node{
		Name:    "deleted-node",
		URL:     "http://127.0.0.1:1",
		Token:   "test-token",
		Enabled: true,
	}
	require.NoError(t, db.Create(node).Error)

	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		analytic.RetrieveNodesStatus(monitorCtx)
	}()
	t.Cleanup(func() {
		cancelMonitor()
		select {
		case <-monitorDone:
		case <-time.After(time.Second):
			t.Error("node status monitor did not stop")
		}
	})

	require.Eventually(t, func() bool {
		_, exists := analytic.SnapshotNodeMap()[node.ID]
		return exists
	}, time.Second, 10*time.Millisecond)

	router := gin.New()
	router.DELETE("/nodes/:id", DeleteNode)
	request := httptest.NewRequest(http.MethodDelete, "/nodes/"+strconv.FormatUint(node.ID, 10), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.ErrorIs(t, db.First(&model.Node{}, node.ID).Error, gorm.ErrRecordNotFound)
	require.Eventually(t, func() bool {
		_, exists := analytic.SnapshotNodeMap()[node.ID]
		return !exists
	}, time.Second, 10*time.Millisecond, "deleted node remained in the analytic snapshot")
}
