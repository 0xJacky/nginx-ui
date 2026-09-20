package clustersync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildItemsStagesAllSitesBeforeApplyingEitherSite(t *testing.T) {
	confDir := withConfDir(t, map[string]string{
		"nginx.conf":                    "events {}\n",
		"sites-available/dependent":     "server { proxy_pass http://shared_upstream; }\n",
		"sites-available/upstream-only": "upstream shared_upstream { server 127.0.0.1:8080; }\n",
	})

	for _, name := range []string{"dependent", "upstream-only"} {
		if err := os.MkdirAll(filepath.Join(confDir, "sites-enabled"), 0o755); err != nil {
			t.Fatalf("create enabled directory: %v", err)
		}
		if err := os.Symlink(filepath.Join(confDir, "sites-available", name), filepath.Join(confDir, "sites-enabled", name)); err != nil {
			t.Fatalf("enable %s: %v", name, err)
		}
	}

	originalQueryDB := query.Q.UnderlyingDB()
	originalModelDB := model.UseDB()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err = db.AutoMigrate(&model.Namespace{}, &model.Site{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() {
		model.Use(originalModelDB)
		if originalQueryDB != nil {
			query.SetDefault(originalQueryDB)
		}
	})

	namespace := &model.Namespace{Name: "production", PostSyncAction: model.PostSyncActionReloadNginx}
	if err = db.Create(namespace).Error; err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	for _, name := range []string{"dependent", "upstream-only"} {
		if err = db.Create(&model.Site{
			Path:        filepath.Join(confDir, "sites-available", name),
			NamespaceID: namespace.ID,
		}).Error; err != nil {
			t.Fatalf("create site %s: %v", name, err)
		}
	}

	items, err := buildItems(Scope{Sites: true, Overwrite: true}, namespace)
	if err != nil {
		t.Fatalf("build items: %v", err)
	}
	if len(items) != 3 || items[0].kind != KindConfig || !items[0].blocking {
		t.Fatalf("expected one blocking batch before two sites, got %+v", items)
	}

	var requestMutex sync.Mutex
	var requestPaths []string
	var stagedFiles []ConfigFile
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMutex.Lock()
		defer requestMutex.Unlock()

		requestPaths = append(requestPaths, r.URL.Path)
		if r.URL.Path == "/api/config_sync_batch" {
			var payload configBatchPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode batch: %v", err)
			}
			stagedFiles = payload.Files
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"written":2,"failures":[]}`))
	}))
	defer server.Close()

	client := resty.New()
	client.SetBaseURL(server.URL)
	summary := run(context.Background(), []nodeRef{{id: 1, name: "remote", client: client}}, items)
	if summary.Failed != 0 {
		t.Fatalf("sync failed: %+v", summary)
	}

	requestMutex.Lock()
	defer requestMutex.Unlock()
	if len(requestPaths) == 0 || requestPaths[0] != "/api/config_sync_batch" {
		t.Fatalf("all files must be staged before a site is applied, got %v", requestPaths)
	}
	if len(stagedFiles) != 2 {
		t.Fatalf("expected both sites in the staging batch, got %+v", stagedFiles)
	}
	stagedNames := map[string]bool{}
	for _, file := range stagedFiles {
		if file.BaseDir != "sites-available" {
			t.Fatalf("unexpected staging directory: %+v", file)
		}
		stagedNames[file.Name] = true
	}
	if !stagedNames["dependent"] || !stagedNames["upstream-only"] {
		t.Fatalf("staging batch did not contain the complete dependency set: %+v", stagedFiles)
	}
}

func TestBuildItemsDoesNotStageNonOverwriteSites(t *testing.T) {
	originalQueryDB := query.Q.UnderlyingDB()
	originalModelDB := model.UseDB()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err = db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() {
		model.Use(originalModelDB)
		if originalQueryDB != nil {
			query.SetDefault(originalQueryDB)
		}
	})

	confDir := withConfDir(t, map[string]string{
		"nginx.conf":             "events {}\n",
		"sites-available/site-a": "server {}\n",
	})
	if err = db.Create(&model.Site{Path: filepath.Join(confDir, "sites-available", "site-a")}).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}

	items, err := buildItems(Scope{Sites: true, Overwrite: false}, nil)
	if err != nil {
		t.Fatalf("build items: %v", err)
	}
	if len(items) != 1 || items[0].kind != KindSite {
		t.Fatalf("a non-overwrite sync must preserve per-site create semantics, got %+v", items)
	}
}
