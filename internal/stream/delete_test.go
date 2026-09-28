package stream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type deleteTestEnv struct {
	db           *gorm.DB
	syncedNames  []string
	availableDir string
	enabledDir   string
}

func setupDeleteTest(t *testing.T) *deleteTestEnv {
	t.Helper()

	confDir := t.TempDir()
	env := &deleteTestEnv{
		availableDir: filepath.Join(confDir, "streams-available"),
		enabledDir:   filepath.Join(confDir, "streams-enabled"),
	}
	for _, dir := range []string{env.availableDir, env.enabledDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	originalConfigDir := appsettings.NginxSettings.ConfigDir
	appsettings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		appsettings.NginxSettings.ConfigDir = originalConfigDir
	})

	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := database.AutoMigrate(&model.Namespace{}, &model.Stream{}, &model.Cert{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	model.Use(database)
	query.SetDefault(database)
	env.db = database

	originalSyncDelete := syncDelete
	syncDelete = func(_ context.Context, name string) {
		env.syncedNames = append(env.syncedNames, name)
	}
	t.Cleanup(func() {
		syncDelete = originalSyncDelete
	})

	return env
}

// addStream writes the stream file and its database record, returning the record.
func (env *deleteTestEnv) addStream(t *testing.T, name string, namespaceID uint64, remoteEnabled bool) *model.Stream {
	t.Helper()

	availablePath := filepath.Join(env.availableDir, name)
	if err := os.WriteFile(availablePath, []byte("server {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write stream file: %v", err)
	}

	streamModel := &model.Stream{
		Path:          availablePath,
		NamespaceID:   namespaceID,
		SyncNodeIDs:   []uint64{1, 2},
		RemoteEnabled: remoteEnabled,
	}
	if err := env.db.Create(streamModel).Error; err != nil {
		t.Fatalf("failed to create stream record: %v", err)
	}
	return streamModel
}

func (env *deleteTestEnv) assertRefusedWithoutSideEffects(t *testing.T, name string, streamModel *model.Stream, err, want error) {
	t.Helper()

	if !errors.Is(err, want) {
		t.Fatalf("Delete(context.Background(), %q) error = %v, want %v", name, err, want)
	}
	if len(env.syncedNames) != 0 {
		t.Fatalf("refused delete dispatched sync delete for %v", env.syncedNames)
	}

	var got model.Stream
	if err := env.db.Unscoped().First(&got, streamModel.ID).Error; err != nil {
		t.Fatalf("refused delete removed the stream record: %v", err)
	}
	if got.NamespaceID != streamModel.NamespaceID || len(got.SyncNodeIDs) != len(streamModel.SyncNodeIDs) {
		t.Fatalf("refused delete changed the stream record: got %+v, want %+v", got, streamModel)
	}
	if _, err := os.Stat(streamModel.Path); err != nil {
		t.Fatalf("refused delete removed the stream file: %v", err)
	}
}

func TestDeleteEnabledStreamKeepsRecordAndSkipsSync(t *testing.T) {
	env := setupDeleteTest(t)

	const name = "enabled.conf"
	streamModel := env.addStream(t, name, 0, false)
	if err := os.Symlink(streamModel.Path, filepath.Join(env.enabledDir, name)); err != nil {
		t.Fatalf("failed to enable stream: %v", err)
	}

	err := Delete(context.Background(), name)
	env.assertRefusedWithoutSideEffects(t, name, streamModel, err, ErrStreamIsEnabled)
}

func TestDeleteRemoteEnabledStreamKeepsRecordAndSkipsSync(t *testing.T) {
	env := setupDeleteTest(t)

	namespace := &model.Namespace{DeployMode: model.DeployModeRemote}
	if err := env.db.Create(namespace).Error; err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	const name = "remote.conf"
	streamModel := env.addStream(t, name, namespace.ID, true)

	err := Delete(context.Background(), name)
	env.assertRefusedWithoutSideEffects(t, name, streamModel, err, ErrStreamIsEnabled)
}

func TestDeleteMissingStreamKeepsRecordAndSkipsSync(t *testing.T) {
	env := setupDeleteTest(t)

	const name = "missing.conf"
	streamModel := env.addStream(t, name, 0, false)
	if err := os.Remove(streamModel.Path); err != nil {
		t.Fatalf("failed to remove stream file: %v", err)
	}

	err := Delete(context.Background(), name)
	if !errors.Is(err, ErrStreamNotFound) {
		t.Fatalf("Delete(context.Background(), %q) error = %v, want %v", name, err, ErrStreamNotFound)
	}
	if len(env.syncedNames) != 0 {
		t.Fatalf("refused delete dispatched sync delete for %v", env.syncedNames)
	}
	if err := env.db.Unscoped().First(&model.Stream{}, streamModel.ID).Error; err != nil {
		t.Fatalf("refused delete removed the stream record: %v", err)
	}
}

func TestDeleteDisabledStreamRemovesEverything(t *testing.T) {
	env := setupDeleteTest(t)

	const name = "disabled.conf"
	streamModel := env.addStream(t, name, 0, false)

	if err := Delete(context.Background(), name); err != nil {
		t.Fatalf("Delete(context.Background(), %q) error = %v", name, err)
	}
	if len(env.syncedNames) != 1 || env.syncedNames[0] != name {
		t.Fatalf("sync delete calls = %v, want [%s]", env.syncedNames, name)
	}
	err := env.db.Unscoped().First(&model.Stream{}, streamModel.ID).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("stream record lookup error = %v, want record not found", err)
	}
	if _, err := os.Stat(streamModel.Path); !os.IsNotExist(err) {
		t.Fatalf("stream file stat error = %v, want not exist", err)
	}
}
