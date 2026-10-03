package backup

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

// StorageField is one field of the configuration form of a storage backend.
// Values travel as strings whatever the type.
type StorageField struct {
	Key string `json:"key"`
	// Type is text (default when empty), textarea, number or bool.
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name"`
	HelpText    string `json:"help_text,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// StorageBackend is a place an automatic backup can store its runs.
type StorageBackend struct {
	// Type is the value stored in model.AutoBackup.StorageType.
	Type string `json:"type"`
	Name string `json:"name"`
	// BuiltIn marks the local and S3 storage, whose forms belong to the host.
	BuiltIn bool `json:"builtin,omitempty"`
	// PluginID names the plugin behind the backend, when there is one.
	PluginID string         `json:"plugin_id,omitempty"`
	Fields   []StorageField `json:"fields"`
}

// StoredObject is one object a storage source keeps.
type StoredObject struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
	// ModifiedAt is zero when the source does not know it.
	ModifiedAt time.Time `json:"modified_at,omitzero"`
}

// StorageSource offers storage backends that come and go at runtime, such as
// the backends of storage plugins. The built-in local and S3 storage are not
// sources; they keep their own code paths.
type StorageSource interface {
	// StorageBackends lists the backends the source serves right now.
	StorageBackends() []StorageBackend
	// Validate checks a configuration without storing anything. It returns
	// nil when it has no opinion.
	Validate(ctx context.Context, storageType string, config map[string]string) error
	// Put stores the file at sourcePath under key and returns its size.
	Put(ctx context.Context, storageType string, config map[string]string, key, sourcePath string) (int64, error)
	// Get writes the object stored under key to targetPath, which must not
	// exist yet, and returns its size.
	Get(ctx context.Context, storageType string, config map[string]string, key, targetPath string) (int64, error)
	// List returns the objects whose key starts with prefix.
	List(ctx context.Context, storageType string, config map[string]string, prefix string) ([]StoredObject, error)
	// Delete removes the object stored under key. A missing object is not
	// an error.
	Delete(ctx context.Context, storageType string, config map[string]string, key string) error
}

var (
	storageSources      []StorageSource
	storageSourcesMutex sync.RWMutex
)

// RegisterStorageSource adds a source of storage backends.
func RegisterStorageSource(source StorageSource) {
	storageSourcesMutex.Lock()
	defer storageSourcesMutex.Unlock()
	storageSources = append(storageSources, source)
}

func registeredStorageSources() []StorageSource {
	storageSourcesMutex.RLock()
	defer storageSourcesMutex.RUnlock()
	return append([]StorageSource(nil), storageSources...)
}

// builtinStorageBackends are always offered first, and local is the default.
var builtinStorageBackends = []StorageBackend{
	{Type: string(model.StorageTypeLocal), Name: "Local", BuiltIn: true, Fields: []StorageField{}},
	{Type: string(model.StorageTypeS3), Name: "S3", BuiltIn: true, Fields: []StorageField{}},
}

// IsBuiltinStorageType reports whether storageType is the local or the S3
// storage.
func IsBuiltinStorageType(storageType model.StorageType) bool {
	return storageType == model.StorageTypeLocal || storageType == model.StorageTypeS3
}

// storageTypePattern bounds the storage types a task may store. Sources name
// their backends within it, e.g. "plugin:webdav".
var storageTypePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`)

// IsValidStorageType reports whether storageType is well formed. It does not
// require a source to serve it right now: a task keeps its storage type while
// the plugin behind it is disabled.
func IsValidStorageType(storageType model.StorageType) bool {
	return IsBuiltinStorageType(storageType) || storageTypePattern.MatchString(string(storageType))
}

// StorageBackends lists the built-in storage first and then every backend a
// source offers, ordered by type.
func StorageBackends() []StorageBackend {
	backends := append([]StorageBackend(nil), builtinStorageBackends...)
	seen := map[string]struct{}{}
	var offered []StorageBackend
	for _, source := range registeredStorageSources() {
		for _, backend := range source.StorageBackends() {
			if IsBuiltinStorageType(model.StorageType(backend.Type)) {
				continue
			}
			if _, dup := seen[backend.Type]; dup {
				continue
			}
			seen[backend.Type] = struct{}{}
			backend.BuiltIn = false
			if backend.Fields == nil {
				backend.Fields = []StorageField{}
			}
			offered = append(offered, backend)
		}
	}
	sort.Slice(offered, func(i, j int) bool { return offered[i].Type < offered[j].Type })
	return append(backends, offered...)
}

// storageSourceFor returns the source that serves a storage type right now
// and the backend it describes.
func storageSourceFor(storageType model.StorageType) (StorageSource, StorageBackend, error) {
	for _, source := range registeredStorageSources() {
		for _, backend := range source.StorageBackends() {
			if backend.Type == string(storageType) {
				return source, backend, nil
			}
		}
	}
	return nil, StorageBackend{}, cosy.WrapErrorWithParams(plugin.ErrStorageBackendUnavailable, string(storageType))
}

// maxStorageKeyLength bounds a key, in bytes.
const maxStorageKeyLength = 1024

// validStorageKey reports whether key is a relative path of "/" separated
// segments, none of them empty, "." or "..", without a backslash or a
// control character.
func validStorageKey(key string) bool {
	if key == "" || len(key) > maxStorageKeyLength {
		return false
	}
	for _, r := range key {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return false
		}
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// maxStorageKeyPrefixLength leaves room for the file names in a key.
const maxStorageKeyPrefixLength = 512

// validateStorageKeyPrefix checks the storage path of a task on a plugin
// backend, which is the key prefix of its runs. "" and "/" mean the root.
func validateStorageKeyPrefix(storagePath string) error {
	prefix := strings.Trim(storagePath, "/")
	if prefix == "" {
		return nil
	}
	if len(prefix) > maxStorageKeyPrefixLength || !validStorageKey(prefix) {
		return cosy.WrapErrorWithParams(ErrInvalidStorageKeyPrefix, storagePath)
	}
	return nil
}

// checkRequiredStorageFields rejects a configuration that leaves a required
// field of the backend form empty.
func checkRequiredStorageFields(backend StorageBackend, config map[string]string) error {
	for _, field := range backend.Fields {
		if field.Required && strings.TrimSpace(config[field.Key]) == "" {
			return cosy.WrapErrorWithParams(plugin.ErrStorageConfigInvalid, field.Key, "it is required")
		}
	}
	return nil
}

// ValidatePluginStorage checks the storage of a task on a plugin backend
// before it is stored: the backend is served, the key prefix is well formed,
// every required field is filled in, and the plugin accepts the values.
func ValidatePluginStorage(ctx context.Context, storageType model.StorageType, storagePath string, config map[string]string) error {
	if !IsValidStorageType(storageType) {
		return cosy.WrapErrorWithParams(ErrAutoBackupUnsupportedType, string(storageType))
	}
	source, backend, err := storageSourceFor(storageType)
	if err != nil {
		return err
	}
	if err = validateStorageKeyPrefix(storagePath); err != nil {
		return err
	}
	if err = checkRequiredStorageFields(backend, config); err != nil {
		return err
	}
	return source.Validate(ctx, string(storageType), config)
}
