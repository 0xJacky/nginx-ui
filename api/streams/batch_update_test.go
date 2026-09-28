package streams

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupStreamBatchUpdateTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = t.TempDir()
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = originalConfigDir })

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Stream{}, model.Namespace{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "stream-batch.db")))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })

	return db
}

// streamPath resolves name the way the handler does, so stored rows match the
// paths it computes (t.TempDir may sit behind a symlink, e.g. /var on macOS).
func streamPath(t *testing.T, name string) string {
	t.Helper()

	path, err := stream.ResolveAvailablePath(name)
	if err != nil {
		t.Fatalf("failed to resolve stream path for %s: %v", name, err)
	}
	return path
}

func putStreamsBatch(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	router := gin.New()
	router.PUT("/api/streams", BatchUpdateStreams)

	request := httptest.NewRequest(http.MethodPut, "/api/streams", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// TestBatchUpdateStreamsChangesNamespace guards issue #1970 for the stream
// list, which shares the site list's batch edit: the namespace must actually
// be written for the selected streams, and nothing else may change.
func TestBatchUpdateStreamsChangesNamespace(t *testing.T) {
	db := setupStreamBatchUpdateTest(t)

	namespace := &model.Namespace{Name: "production"}
	if err := db.Create(namespace).Error; err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	// "existing" already has a row; "fresh" only exists on disk and gets its
	// row from the handler's BeforeExecute hook.
	existing := &model.Stream{Path: streamPath(t, "existing"), Advanced: true}
	untouched := &model.Stream{Path: streamPath(t, "untouched")}
	if err := db.Create([]*model.Stream{existing, untouched}).Error; err != nil {
		t.Fatalf("failed to create streams: %v", err)
	}

	response := putStreamsBatch(t, gin.H{
		"ids":  []string{"existing", "fresh"},
		"data": gin.H{"namespace_id": namespace.ID},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var streams []model.Stream
	if err := db.Find(&streams).Error; err != nil {
		t.Fatalf("failed to load streams: %v", err)
	}
	got := make(map[string]model.Stream, len(streams))
	for _, s := range streams {
		got[filepath.Base(s.Path)] = s
	}

	for _, name := range []string{"existing", "fresh"} {
		s, ok := got[name]
		if !ok {
			t.Fatalf("expected a stream row for %s", name)
		}
		if s.NamespaceID != namespace.ID {
			t.Errorf("%s: expected namespace_id %d, got %d", name, namespace.ID, s.NamespaceID)
		}
	}
	if !got["existing"].Advanced {
		t.Error("batch update must not touch other columns, advanced was reset")
	}
	if got["untouched"].NamespaceID != 0 {
		t.Errorf("unselected stream must keep its namespace, got %d", got["untouched"].NamespaceID)
	}
}
