package capability

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/uozi-tech/cosy"
)

const storagePluginID = "io.github.example.webdav"

// storageHost serves one backend whose plugin keeps objects in memory and
// reads and writes the exchange files like a real plugin.
func storageHost(t *testing.T) (*capabilityHost, map[string][]byte, *[]string) {
	t.Helper()
	h := newCapabilityHost()
	h.dataDirs[storagePluginID] = t.TempDir()
	h.backends = []plugin.StorageBackendEntry{
		{PluginID: storagePluginID, Backend: protocol.StorageBackend{
			Code: "webdav",
			Name: "WebDAV",
			Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
				{Key: "url", DisplayName: "Server URL", Required: true},
				{Key: "password", DisplayName: "Password", Secret: true},
			}},
		}},
		// A second plugin declaring the same code does not own it.
		{PluginID: "io.github.other.webdav", Backend: protocol.StorageBackend{Code: "webdav", Name: "Shadowed"}},
	}
	h.own(protocol.CapabilityStorage, "webdav", storagePluginID)

	objects := map[string][]byte{}
	var seenPaths []string
	exchange := filepath.Join(h.dataDirs[storagePluginID], exchangeDirName)
	h.handlers[storagePluginID] = handlerCaller(func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case protocol.MethodStorageValidate:
			var p protocol.StorageValidateParams
			_ = json.Unmarshal(raw, &p)
			if !strings.HasPrefix(p.Config["url"], "https://") {
				return nil, &protocol.Error{Code: protocol.CodeInvalidConfig, Message: "url must be https", Data: map[string]any{"field": "url"}}
			}
			return protocol.EmptyResult{}, nil
		case protocol.MethodStoragePut:
			var p protocol.StoragePutParams
			_ = json.Unmarshal(raw, &p)
			if !strings.HasPrefix(p.SourcePath, exchange+string(filepath.Separator)) {
				t.Errorf("source_path %s is outside the exchange directory %s", p.SourcePath, exchange)
			}
			seenPaths = append(seenPaths, p.SourcePath)
			data, err := os.ReadFile(p.SourcePath)
			if err != nil {
				return nil, err
			}
			objects[p.Key] = data
			return protocol.StorageSizeResult{Size: protocol.ByteSize(len(data))}, nil
		case protocol.MethodStorageGet:
			var p protocol.StorageGetParams
			_ = json.Unmarshal(raw, &p)
			if !strings.HasPrefix(p.TargetPath, exchange+string(filepath.Separator)) {
				t.Errorf("target_path %s is outside the exchange directory %s", p.TargetPath, exchange)
			}
			if _, err := os.Stat(p.TargetPath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("target_path %s exists before the call", p.TargetPath)
			}
			seenPaths = append(seenPaths, p.TargetPath)
			data, ok := objects[p.Key]
			if !ok {
				return nil, &protocol.Error{Code: protocol.CodeInternalError, Message: "no object " + p.Key}
			}
			return protocol.StorageSizeResult{Size: protocol.ByteSize(len(data))}, os.WriteFile(p.TargetPath, data, 0o600)
		case protocol.MethodStorageList:
			var p protocol.StorageListParams
			_ = json.Unmarshal(raw, &p)
			var list []map[string]any
			for key, data := range objects {
				if strings.HasPrefix(key, p.Prefix) {
					list = append(list, map[string]any{"key": key, "size": float64(len(data)), "modified_at": "2026-09-21T03:00:05Z"})
				}
			}
			return map[string]any{"objects": list}, nil
		case protocol.MethodStorageDelete:
			var p protocol.StorageDeleteParams
			_ = json.Unmarshal(raw, &p)
			delete(objects, p.Key)
			return protocol.EmptyResult{}, nil
		}
		return nil, &protocol.Error{Code: protocol.CodeMethodNotFound, Message: method}
	})
	return h, objects, &seenPaths
}

func TestStorageBackendsListTheOwnedCodes(t *testing.T) {
	h, _, _ := storageHost(t)
	backends := NewStorageSource(h).StorageBackends()
	if len(backends) != 1 {
		t.Fatalf("backends = %+v, want one", backends)
	}
	backend := backends[0]
	if backend.Type != "plugin:webdav" || backend.Name != "WebDAV" || backend.PluginID != storagePluginID {
		t.Fatalf("backend = %+v", backend)
	}
	if len(backend.Fields) != 2 || !backend.Fields[0].Required || !backend.Fields[1].Secret {
		t.Fatalf("fields = %+v", backend.Fields)
	}
}

func TestStorageMovesFilesThroughTheExchangeDirectory(t *testing.T) {
	h, objects, seen := storageHost(t)
	source := NewStorageSource(h)
	config := map[string]string{"url": "https://dav.example"}

	local := filepath.Join(t.TempDir(), "daily_1790000000.zip")
	if err := os.WriteFile(local, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, err := source.Put(t.Context(), "plugin:webdav", config, "nginx-ui/daily_1790000000.zip", local)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if size != 7 || string(objects["nginx-ui/daily_1790000000.zip"]) != "archive" {
		t.Fatalf("size = %d, objects = %v", size, objects)
	}
	if data, _ := os.ReadFile(local); string(data) != "archive" {
		t.Fatal("the source file must stay untouched")
	}

	target := filepath.Join(t.TempDir(), "fetched.zip")
	if size, err = source.Get(t.Context(), "plugin:webdav", config, "nginx-ui/daily_1790000000.zip", target); err != nil {
		t.Fatalf("get: %v", err)
	}
	if data, _ := os.ReadFile(target); string(data) != "archive" || size != 7 {
		t.Fatalf("fetched %q (%d bytes)", data, size)
	}

	// Every exchange directory is gone once its call returned.
	for _, path := range *seen {
		if _, statErr := os.Stat(filepath.Dir(path)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("exchange directory %s was not removed", filepath.Dir(path))
		}
		if filepath.Base(path) != "daily_1790000000.zip" {
			t.Fatalf("exchanged file is named %s", filepath.Base(path))
		}
	}

	listed, err := source.List(t.Context(), "plugin:webdav", config, "nginx-ui/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 || listed[0].Size != 7 || !listed[0].ModifiedAt.Equal(time.Date(2026, 9, 21, 3, 0, 5, 0, time.UTC)) {
		t.Fatalf("listed = %+v", listed)
	}

	if err = source.Delete(t.Context(), "plugin:webdav", config, "nginx-ui/daily_1790000000.zip"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(objects) != 0 {
		t.Fatalf("objects = %v after delete", objects)
	}
	if h.releaseCount() != 4 {
		t.Fatalf("releases = %d, want 4", h.releaseCount())
	}
}

func TestStorageMapsTheErrors(t *testing.T) {
	h, _, _ := storageHost(t)
	source := NewStorageSource(h)

	target := filepath.Join(t.TempDir(), "missing.zip")
	_, err := source.Get(t.Context(), "plugin:webdav", map[string]string{"url": "https://dav.example"}, "nginx-ui/missing.zip", target)
	if !errors.Is(err, plugin.ErrRPC) {
		t.Fatalf("err = %v, want the plugin rpc error", err)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("a failed get must not leave a target file")
	}

	for _, storageType := range []string{"plugin:unknown", "s3"} {
		_, err = source.List(t.Context(), storageType, nil, "")
		assertCosyCode(t, err, plugin.ErrStorageBackendUnavailable)
	}

	err = source.Validate(t.Context(), "plugin:webdav", map[string]string{"url": "ftp://dav.example"})
	assertCosyCode(t, err, plugin.ErrStorageConfigInvalid)
	if err = source.Validate(t.Context(), "plugin:webdav", map[string]string{"url": "https://dav.example"}); err != nil {
		t.Fatalf("validate: %v", err)
	}

	// A plugin that cannot be reached has no opinion on a configuration.
	h.acquireErr = errors.New("cannot start")
	if err = source.Validate(t.Context(), "plugin:webdav", map[string]string{}); err != nil {
		t.Fatalf("validate of an unreachable plugin = %v, want nil", err)
	}
}

func TestStorageSourceThroughTheBackupRegistry(t *testing.T) {
	h, objects, _ := storageHost(t)
	backup.RegisterStorageSource(NewStorageSource(h))

	var found bool
	for _, backend := range backup.StorageBackends() {
		if backend.Type == "plugin:webdav" {
			found = true
		}
	}
	if !found {
		t.Fatal("the plugin backend is not offered")
	}

	// Required fields are checked before the plugin is asked.
	err := backup.ValidatePluginStorage(t.Context(), "plugin:webdav", "nginx-ui", map[string]string{})
	assertCosyCode(t, err, plugin.ErrStorageConfigInvalid)
	if err = backup.ValidatePluginStorage(t.Context(), "plugin:webdav", "nginx-ui", map[string]string{"url": "https://dav.example"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(objects) != 0 {
		t.Fatal("validation must not store anything")
	}
}

// assertCosyCode checks that err carries the code of want.
func assertCosyCode(t *testing.T, err, want error) {
	t.Helper()
	got, ok := errors.AsType[*cosy.Error](err)
	if !ok {
		t.Fatalf("err = %v, want a cosy error", err)
	}
	wantErr, _ := errors.AsType[*cosy.Error](want)
	if got.Code != wantErr.Code {
		t.Fatalf("err = %+v, want code %d", got, wantErr.Code)
	}
}
