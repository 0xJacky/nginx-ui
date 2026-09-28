package sites

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cosyModel "github.com/uozi-tech/cosy/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSiteBatchUpdateTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	confDir := t.TempDir()
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = originalConfigDir })

	cosyModel.ClearCollection()
	cosy.RegisterModels(model.Site{}, model.Namespace{}, model.SiteConfig{})
	db := cosy.InitDB(sqlite.Open(filepath.Join(t.TempDir(), "site-batch.db")))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })

	return db
}

// sitePath resolves name the way the handler does, so stored rows match the
// paths it computes (t.TempDir may sit behind a symlink, e.g. /var on macOS).
func sitePath(t *testing.T, name string) string {
	t.Helper()

	path, err := site.ResolveAvailablePath(name)
	if err != nil {
		t.Fatalf("failed to resolve site path for %s: %v", name, err)
	}
	return path
}

func putSitesBatch(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	router := gin.New()
	router.PUT("/api/sites", BatchUpdateSites)

	request := httptest.NewRequest(http.MethodPut, "/api/sites", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// TestBatchUpdateSitesChangesNamespace guards issue #1970: the site list's
// batch edit sends {"ids": [...], "data": {"namespace_id": N}}, and cosy
// BatchModify only writes fields tagged `cosy:"batch"`. Without that tag the
// request was rejected with 406 "empty payload".
func TestBatchUpdateSitesChangesNamespace(t *testing.T) {
	db := setupSiteBatchUpdateTest(t)

	namespace := &model.Namespace{Name: "production"}
	if err := db.Create(namespace).Error; err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	// "existing.conf" already has a row; "fresh.conf" only exists on disk and
	// gets its row from the handler's BeforeExecute hook.
	existing := &model.Site{Path: sitePath(t, "existing.conf"), Description: "keep me"}
	untouched := &model.Site{Path: sitePath(t, "untouched.conf")}
	if err := db.Create([]*model.Site{existing, untouched}).Error; err != nil {
		t.Fatalf("failed to create sites: %v", err)
	}

	response := putSitesBatch(t, gin.H{
		"ids":  []string{"existing.conf", "fresh.conf"},
		"data": gin.H{"namespace_id": namespace.ID},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var sites []model.Site
	if err := db.Order("path").Find(&sites).Error; err != nil {
		t.Fatalf("failed to load sites: %v", err)
	}
	got := make(map[string]model.Site, len(sites))
	for _, s := range sites {
		got[filepath.Base(s.Path)] = s
	}

	for _, name := range []string{"existing.conf", "fresh.conf"} {
		s, ok := got[name]
		if !ok {
			t.Fatalf("expected a site row for %s", name)
		}
		if s.NamespaceID != namespace.ID {
			t.Errorf("%s: expected namespace_id %d, got %d", name, namespace.ID, s.NamespaceID)
		}
	}
	if got["existing.conf"].Description != "keep me" {
		t.Errorf("batch update must not touch other columns, description is now %q", got["existing.conf"].Description)
	}
	if got["untouched.conf"].NamespaceID != 0 {
		t.Errorf("unselected site must keep its namespace, got %d", got["untouched.conf"].NamespaceID)
	}
}

func TestBatchUpdateSitesRequiresNamespace(t *testing.T) {
	setupSiteBatchUpdateTest(t)

	response := putSitesBatch(t, gin.H{
		"ids":  []string{"existing.conf"},
		"data": gin.H{},
	})
	if response.Code != http.StatusNotAcceptable {
		t.Fatalf("expected 406, got %d: %s", response.Code, response.Body.String())
	}
}
