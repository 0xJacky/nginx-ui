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

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func withSiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalQueryDB := query.Q.UnderlyingDB()
	originalModelDB := model.UseDB()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err = db.AutoMigrate(&model.Namespace{}, &model.Site{}, &model.Cert{}); err != nil {
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
	return db
}

func TestBuildItemsPushesLoadedCertificatesBeforeSites(t *testing.T) {
	db := withSiteDB(t)
	confDir := withConfDir(t, map[string]string{
		"nginx.conf":                          "events {}\n",
		"ssl/example.com_EC256/fullchain.cer": "cert",
		"ssl/example.com_EC256/private.key":   "key",
	})
	siteContent := fmt.Sprintf("server { listen 443 ssl; ssl_certificate %[1]s/ssl/example.com_EC256/fullchain.cer; ssl_certificate_key %[1]s/ssl/example.com_EC256/private.key; }\n", confDir)
	writeTestFile(t, filepath.Join(confDir, "sites-available", "example"), siteContent)

	namespace := &model.Namespace{Name: "edge", PostSyncAction: model.PostSyncActionReloadNginx}
	if err := db.Create(namespace).Error; err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	if err := db.Create(&model.Site{
		Path:        filepath.Join(confDir, "sites-available", "example"),
		NamespaceID: namespace.ID,
	}).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}

	items, err := buildItems(Scope{Sites: true, Overwrite: true}, namespace)
	if err != nil {
		t.Fatalf("build items: %v", err)
	}
	if len(items) != 3 || items[0].kind != KindCertificate || !items[0].blocking {
		t.Fatalf("expected a blocking certificate push first, got %+v", items)
	}

	var mutex sync.Mutex
	var requests []string
	var received cert.SyncCertificatePayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()

		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/api/cert_sync" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode certificate: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"written":1,"failures":[]}`))
	}))
	defer server.Close()

	client := resty.New()
	client.SetBaseURL(server.URL)
	summary := run(context.Background(), []nodeRef{{id: 1, name: "remote", client: client}}, items)
	if summary.Failed != 0 {
		t.Fatalf("sync failed: %+v", summary)
	}

	mutex.Lock()
	defer mutex.Unlock()
	if len(requests) == 0 || requests[0] != "PUT /api/cert_sync" {
		t.Fatalf("the certificate must reach the node before the site, got %v", requests)
	}
	if received.SSLCertificate != "cert" || received.SSLCertificateKey != "key" ||
		received.SSLCertificatePath != filepath.Join(confDir, "ssl/example.com_EC256/fullchain.cer") {
		t.Fatalf("unexpected certificate payload: %+v", received)
	}
}

func TestCertificateItemFailureBlocksSites(t *testing.T) {
	certItem, ok := certificateItem([]*cert.SyncCertificatePayload{{
		Name:                  "example.com",
		SSLCertificatePath:    "/etc/nginx/ssl/example/fullchain.cer",
		SSLCertificateKeyPath: "/etc/nginx/ssl/example/private.key",
	}})
	if !ok {
		t.Fatal("expected a certificate item")
	}

	var mutex sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requests = append(requests, r.URL.Path)
		mutex.Unlock()
		if r.URL.Path == "/api/cert_sync" {
			http.Error(w, "disk full", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := resty.New()
	client.SetBaseURL(server.URL)
	items := []item{certItem, siteItem("example", "server {}", "", model.PostSyncActionReloadNginx, true, true)}
	summary := run(context.Background(), []nodeRef{{id: 1, name: "remote", client: client}}, items)
	if summary.Failed != 2 {
		t.Fatalf("a failed certificate must fail the site that loads it, got %+v", summary)
	}

	mutex.Lock()
	defer mutex.Unlock()
	for _, path := range requests {
		if path == "/api/sites/example" {
			t.Fatalf("the site must not be pushed without its certificate, got %v", requests)
		}
	}
}

func TestCertificateItemSkipsEmptySet(t *testing.T) {
	if _, ok := certificateItem(nil); ok {
		t.Fatal("no certificate means no item")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
