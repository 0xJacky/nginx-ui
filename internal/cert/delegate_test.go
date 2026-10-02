package cert

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func withDelegationDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalModelDB := model.UseDB()
	originalQueryDB := query.Q.UnderlyingDB()
	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err = db.AutoMigrate(&model.Cert{}, &model.Namespace{}, &model.Site{}, &model.Stream{},
		&model.Node{}, &model.NodeCredential{}, &model.Notification{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() {
		model.Use(originalModelDB)
		if originalQueryDB != nil {
			query.SetDefault(originalQueryDB)
		}
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func withTestConfigCmd(t *testing.T, cmd string) {
	t.Helper()
	original := settings.NginxSettings.TestConfigCmd
	settings.NginxSettings.TestConfigCmd = cmd
	t.Cleanup(func() { settings.NginxSettings.TestConfigCmd = original })
}

// fakeNode records the requests a node receives and answers them.
type fakeNode struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   [][]byte
	answer   func(w http.ResponseWriter, r *http.Request)
}

func newFakeNode(t *testing.T, db *gorm.DB, answer func(w http.ResponseWriter, r *http.Request)) (*fakeNode, *model.Node) {
	t.Helper()
	fake := &fakeNode{answer: answer}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fake.mu.Lock()
		fake.requests = append(fake.requests, r)
		fake.bodies = append(fake.bodies, body)
		fake.mu.Unlock()
		if fake.answer != nil {
			fake.answer(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	t.Cleanup(server.Close)

	originalSecret, originalInstance := settings.CryptoSettings.Secret, settings.NodeSettings.InstanceID
	settings.CryptoSettings.Secret = "delegation-test-root"
	settings.NodeSettings.InstanceID = "22222222-2222-4222-8222-222222222222"
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = originalSecret
		settings.NodeSettings.InstanceID = originalInstance
	})

	var count int64
	db.Model(&model.Node{}).Count(&count)
	id := uint64(count + 1)
	secret, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(id), []byte("node-secret"))
	if err != nil {
		t.Fatalf("encrypt node secret: %v", err)
	}
	node := &model.Node{
		Model:                 model.Model{ID: id},
		Name:                  "edge",
		URL:                   server.URL,
		Enabled:               true,
		AuthMethod:            model.NodeAuthMethodLegacy,
		EncryptedLegacySecret: secret,
	}
	if err = db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return fake, node
}

func cosyCode(t *testing.T, err error) int32 {
	t.Helper()
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("expected a cosy error, got %v", err)
	}
	return cosyErr.Code
}

func TestDelegatedFilenameCannotNameAConfiguration(t *testing.T) {
	name := DelegatedFilename(3, "example.com")
	if name != "node:3/example.com" {
		t.Fatalf("unexpected name %q", name)
	}
	// A configuration name is a file name, so it never holds a slash.
	if filepath.Base(name) == name {
		t.Fatal("the key must not be a valid configuration file name")
	}
}

func TestSyncPathsForKeepsTheFilesOfAnExistingRecord(t *testing.T) {
	db := withDelegationDB(t)
	confDir := withSyncConfDir(t, nil)
	existing := &model.Cert{
		Filename:              "example.com",
		KeyType:               certcrypto.EC256,
		SSLCertificatePath:    filepath.Join(confDir, "ssl/custom/fullchain.cer"),
		SSLCertificateKeyPath: filepath.Join(confDir, "ssl/custom/private.key"),
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create cert: %v", err)
	}

	paths := SyncPathsFor(SyncPathsRequest{Name: "example.com", ServerName: []string{"example.com"}, KeyType: certcrypto.EC256})
	if paths.SSLCertificatePath != existing.SSLCertificatePath || paths.SSLCertificateKeyPath != existing.SSLCertificateKeyPath {
		t.Fatalf("expected the files of the record, got %+v", paths)
	}

	paths = SyncPathsFor(SyncPathsRequest{Name: "other.com", ServerName: []string{"other.com", "www.other.com"}, KeyType: certcrypto.RSA2048})
	want := filepath.Join(confDir, "ssl", "other.com_www.other.com_"+string(certcrypto.RSA2048))
	if paths.SSLCertificatePath != filepath.Join(want, "fullchain.cer") || paths.SSLCertificateKeyPath != filepath.Join(want, "private.key") {
		t.Fatalf("expected the default location, got %+v", paths)
	}
}

func TestRemoveSyncedPutsTheFilesBackWhileNginxLoadsThem(t *testing.T) {
	db := withDelegationDB(t)
	confDir := withSyncConfDir(t, map[string]string{
		"ssl/example.com_EC256/fullchain.cer": "cert",
		"ssl/example.com_EC256/private.key":   "key",
	})
	paths := SyncPaths{
		SSLCertificatePath:    filepath.Join(confDir, "ssl/example.com_EC256/fullchain.cer"),
		SSLCertificateKeyPath: filepath.Join(confDir, "ssl/example.com_EC256/private.key"),
	}
	synced := &model.Cert{Name: "example.com", SSLCertificatePath: paths.SSLCertificatePath,
		SSLCertificateKeyPath: paths.SSLCertificateKeyPath, AutoCert: model.AutoCertSync}
	if err := db.Create(synced).Error; err != nil {
		t.Fatalf("create cert: %v", err)
	}

	withTestConfigCmd(t, "echo 'cannot load certificate' >&2; exit 1")
	err := RemoveSynced(paths)
	if code := cosyCode(t, err); code != 50071 {
		t.Fatalf("expected the still loaded error, got %d", code)
	}
	for _, path := range []string{paths.SSLCertificatePath, paths.SSLCertificateKeyPath} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("%s must be put back: %v", path, statErr)
		}
	}

	withTestConfigCmd(t, "true")
	if err = RemoveSynced(paths); err != nil {
		t.Fatalf("remove synced: %v", err)
	}
	if _, statErr := os.Stat(paths.SSLCertificatePath); !os.IsNotExist(statErr) {
		t.Fatalf("the certificate must be gone, got %v", statErr)
	}
	var count int64
	db.Model(&model.Cert{}).Where("id = ?", synced.ID).Count(&count)
	if count != 0 {
		t.Fatal("the synced record must be deleted")
	}
}

func TestRemoveSyncedRefusesPathsOutsideTheConfiguration(t *testing.T) {
	withDelegationDB(t)
	withSyncConfDir(t, nil)
	err := RemoveSynced(SyncPaths{SSLCertificatePath: "/etc/passwd", SSLCertificateKeyPath: "/etc/shadow"})
	if code := cosyCode(t, err); code != 50006 {
		t.Fatalf("expected the outside path error, got %d", code)
	}
}

func TestStopRenewingDelegatedHandsTheFilesOver(t *testing.T) {
	db := withDelegationDB(t)
	renewing := &model.Cert{SSLCertificatePath: "/c", SSLCertificateKeyPath: "/k", AutoCert: model.AutoCertEnabled}
	other := &model.Cert{SSLCertificatePath: "/other", SSLCertificateKeyPath: "/k", AutoCert: model.AutoCertEnabled}
	db.Create(renewing)
	db.Create(other)

	if err := StopRenewingDelegated("/c", "/k"); err != nil {
		t.Fatalf("stop renewing: %v", err)
	}
	var got, kept model.Cert
	db.First(&got, renewing.ID)
	if got.AutoCert != model.AutoCertSync {
		t.Fatalf("expected the record to stop renewing, got %d", got.AutoCert)
	}
	db.First(&kept, other.ID)
	if kept.AutoCert != model.AutoCertEnabled {
		t.Fatal("a record of other files must keep renewing")
	}
}

func TestSyncToRemoteServerSendsADelegatedCertificateToItsNode(t *testing.T) {
	db := withDelegationDB(t)
	confDir := withSyncConfDir(t, map[string]string{
		"ssl/nodes/1/example.com_EC256/fullchain.cer": "renewed cert",
		"ssl/nodes/1/example.com_EC256/private.key":   "renewed key",
	})
	fake, node := newFakeNode(t, db, nil)

	certModel := &model.Cert{
		Name:                        "example.com",
		Filename:                    DelegatedFilename(node.ID, "example.com"),
		KeyType:                     certcrypto.EC256,
		SSLCertificatePath:          filepath.Join(confDir, "ssl/nodes/1/example.com_EC256/fullchain.cer"),
		SSLCertificateKeyPath:       filepath.Join(confDir, "ssl/nodes/1/example.com_EC256/private.key"),
		DelegatedNodeID:             node.ID,
		DelegatedConfigName:         "example.com",
		RemoteSSLCertificatePath:    "/srv/nginx/ssl/example.com/fullchain.cer",
		RemoteSSLCertificateKeyPath: "/srv/nginx/ssl/example.com/private.key",
	}
	if err := SyncToRemoteServer(certModel); err != nil {
		t.Fatalf("sync: %v", err)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.requests) != 1 || fake.requests[0].Method != http.MethodPut || fake.requests[0].URL.Path != "/api/cert_sync" {
		t.Fatalf("expected one certificate push, got %d", len(fake.requests))
	}
	var payload SyncCertificatePayload
	if err := json.Unmarshal(fake.bodies[0], &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.SSLCertificatePath != certModel.RemoteSSLCertificatePath ||
		payload.SSLCertificateKeyPath != certModel.RemoteSSLCertificateKeyPath ||
		payload.SSLCertificate != "renewed cert" || payload.SSLCertificateKey != "renewed key" || !payload.Delegated {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestRemoveDelegatedKeepsTheFilesOfTheNodeUnlessAsked(t *testing.T) {
	db := withDelegationDB(t)
	confDir := withSyncConfDir(t, map[string]string{
		"ssl/nodes/1/example.com_EC256/fullchain.cer": "cert",
		"ssl/nodes/1/example.com_EC256/private.key":   "key",
	})
	fake, node := newFakeNode(t, db, nil)

	newRecord := func() *model.Cert {
		c := &model.Cert{
			Filename:                    DelegatedFilename(node.ID, "example.com"),
			SSLCertificatePath:          filepath.Join(confDir, "ssl/nodes/1/example.com_EC256/fullchain.cer"),
			SSLCertificateKeyPath:       filepath.Join(confDir, "ssl/nodes/1/example.com_EC256/private.key"),
			DelegatedNodeID:             node.ID,
			RemoteSSLCertificatePath:    "/srv/nginx/ssl/example.com/fullchain.cer",
			RemoteSSLCertificateKeyPath: "/srv/nginx/ssl/example.com/private.key",
		}
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("create cert: %v", err)
		}
		return c
	}

	kept := newRecord()
	if err := RemoveDelegated(context.Background(), kept, false); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(fake.requests) != 0 {
		t.Fatal("the node must not be asked to remove its files")
	}
	if _, err := os.Stat(kept.SSLCertificatePath); !os.IsNotExist(err) {
		t.Fatal("the copy of this instance must be removed")
	}
	var count int64
	db.Model(&model.Cert{}).Count(&count)
	if count != 0 {
		t.Fatal("the record must be deleted")
	}

	removed := newRecord()
	if err := RemoveDelegated(context.Background(), removed, true); err != nil {
		t.Fatalf("remove with the node files: %v", err)
	}
	if len(fake.requests) != 1 || fake.requests[0].Method != http.MethodDelete || fake.requests[0].URL.Path != "/api/cert_sync" {
		t.Fatalf("expected the node to be asked to remove its files, got %d requests", len(fake.requests))
	}
	var paths SyncPaths
	if err := json.Unmarshal(fake.bodies[0], &paths); err != nil || paths.SSLCertificatePath != removed.RemoteSSLCertificatePath {
		t.Fatalf("unexpected removal request %s: %v", fake.bodies[0], err)
	}
}

func TestRemoveDelegatedStopsWhenTheNodeRefuses(t *testing.T) {
	db := withDelegationDB(t)
	_, node := newFakeNode(t, db, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotAcceptable)
		_, _ = w.Write([]byte(`{"message":"the Nginx configuration still loads the certificate"}`))
	})
	c := &model.Cert{Filename: DelegatedFilename(node.ID, "a"), DelegatedNodeID: node.ID,
		RemoteSSLCertificatePath: "/c", RemoteSSLCertificateKeyPath: "/k"}
	db.Create(c)

	err := RemoveDelegated(context.Background(), c, true)
	if code := cosyCode(t, err); code != 50070 || !strings.Contains(err.Error(), "still loads") {
		t.Fatalf("expected the refusal of the node, got %v", err)
	}
	var count int64
	db.Model(&model.Cert{}).Count(&count)
	if count != 1 {
		t.Fatal("the record must stay when the node refuses")
	}
}

func TestRequestSyncPathsReportsANodeThatCannotReceiveCertificates(t *testing.T) {
	db := withDelegationDB(t)
	_, node := newFakeNode(t, db, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := RequestSyncPaths(context.Background(), node, SyncPathsRequest{Name: "a", ServerName: []string{"a.com"}})
	if code := cosyCode(t, err); code != 50069 {
		t.Fatalf("expected the old node error, got %v", err)
	}
}

func TestIssueForNodeOnlyRunsDNS01(t *testing.T) {
	withDelegationDB(t)
	_, err := IssueForNode(context.Background(), 1, "a", &ConfigPayload{ChallengeMethod: HTTP01}, NewLogger())
	if !errors.Is(err, ErrDelegationRequiresDNS01) {
		t.Fatalf("expected the DNS-01 error, got %v", err)
	}
}

func TestExistingRemotePathsKeepTheFilesOfTheNode(t *testing.T) {
	db := withDelegationDB(t)
	if got := existingRemotePaths(1, "example.com", certcrypto.EC256); got != nil {
		t.Fatalf("expected nothing before the first issuance, got %+v", got)
	}
	db.Create(&model.Cert{
		Filename:                    DelegatedFilename(1, "example.com"),
		KeyType:                     certcrypto.EC256,
		RemoteSSLCertificatePath:    "/srv/ssl/a/fullchain.cer",
		RemoteSSLCertificateKeyPath: "/srv/ssl/a/private.key",
	})
	got := existingRemotePaths(1, "example.com", certcrypto.EC256)
	if got == nil || got.SSLCertificatePath != "/srv/ssl/a/fullchain.cer" {
		t.Fatalf("expected the recorded paths, got %+v", got)
	}
	if existingRemotePaths(2, "example.com", certcrypto.EC256) != nil {
		t.Fatal("another node must not reuse the paths")
	}
}

func TestPushDelegatedReturnsTheRecordOfTheNode(t *testing.T) {
	db := withDelegationDB(t)
	confDir := withSyncConfDir(t, map[string]string{
		"ssl/nodes/1/a_EC256/fullchain.cer": "cert",
		"ssl/nodes/1/a_EC256/private.key":   "key",
	})
	_, node := newFakeNode(t, db, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok","id":7}`))
	})
	id, err := pushDelegated(node, &model.Cert{
		Name:                        "a",
		SSLCertificatePath:          filepath.Join(confDir, "ssl/nodes/1/a_EC256/fullchain.cer"),
		SSLCertificateKeyPath:       filepath.Join(confDir, "ssl/nodes/1/a_EC256/private.key"),
		RemoteSSLCertificatePath:    "/srv/a/fullchain.cer",
		RemoteSSLCertificateKeyPath: "/srv/a/private.key",
	})
	if err != nil || id != 7 {
		t.Fatalf("expected the record 7 of the node, got %d, %v", id, err)
	}
}
