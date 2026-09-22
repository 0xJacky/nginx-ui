package capability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	// storageTransferTimeout bounds storage.put and storage.get, which move
	// whole archives.
	storageTransferTimeout = 10 * time.Minute
	// storageCallTimeout bounds the other storage methods.
	storageCallTimeout = 30 * time.Second
	// exchangeDirName is the directory of the plugin's data directory files
	// are exchanged through (spec STORAGE-5).
	exchangeDirName = "exchange"
	// exchangeLeftoverAge is when an exchange directory is considered left
	// behind by an earlier run of the host.
	exchangeLeftoverAge = 24 * time.Hour
)

// StorageHost is the part of the plugin manager the storage capability needs.
type StorageHost interface {
	// OwnerOf returns the plugin that serves a backend code.
	OwnerOf(capability, code string) (pluginID string, ok bool)
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// StorageBackends lists what every enabled storage plugin offers.
	StorageBackends() []plugin.StorageBackendEntry
	// DataDir is the private writable directory of a plugin.
	DataDir(pluginID string) string
}

// RegisterStorage offers the backends of every enabled storage plugin as
// automatic backup storage, next to the built-in local and S3 storage.
func RegisterStorage(h StorageHost) {
	backup.RegisterStorageSource(NewStorageSource(h))
}

// NewStorageSource exposes the storage capability of the plugins of h as a
// storage source. A backend code is published as PluginType(code).
func NewStorageSource(h StorageHost) backup.StorageSource {
	return &storageSource{host: h}
}

type storageSource struct {
	host StorageHost
}

// StorageBackends lists every backend once, served by the plugin that owns
// its code.
func (s *storageSource) StorageBackends() []backup.StorageBackend {
	entries := s.host.StorageBackends()
	backends := make([]backup.StorageBackend, 0, len(entries))
	for _, entry := range entries {
		if owner, ok := s.host.OwnerOf(protocol.CapabilityStorage, entry.Backend.Code); !ok || owner != entry.PluginID {
			continue
		}
		fields := configurationFields(entry.Backend.Configuration)
		backend := backup.StorageBackend{
			Type:     PluginType(entry.Backend.Code),
			Name:     entry.Backend.Name,
			PluginID: entry.PluginID,
			Fields:   make([]backup.StorageField, 0, len(fields)),
		}
		for _, field := range fields {
			backend.Fields = append(backend.Fields, backup.StorageField{
				Key:         field.Key,
				Type:        field.Type,
				DisplayName: field.DisplayName,
				HelpText:    field.HelpText,
				Required:    field.Required,
				Secret:      field.Secret,
			})
		}
		backends = append(backends, backend)
	}
	return backends
}

// storageCall is one call to the plugin that owns a backend.
type storageCall struct {
	pluginID string
	code     string
	caller   jsonrpc.Caller
	release  func()
	cancel   context.CancelFunc
	ctx      context.Context
}

func (c *storageCall) close() {
	c.cancel()
	c.release()
}

// begin resolves the owner of a backend, bounds the call and starts the
// plugin when it is on_demand.
func (s *storageSource) begin(ctx context.Context, storageType string, timeout time.Duration) (*storageCall, error) {
	code, ok := pluginCode(storageType)
	if !ok {
		return nil, cosy.WrapErrorWithParams(plugin.ErrStorageBackendUnavailable, storageType)
	}
	pluginID, ok := s.host.OwnerOf(protocol.CapabilityStorage, code)
	if !ok {
		return nil, cosy.WrapErrorWithParams(plugin.ErrStorageBackendUnavailable, storageType)
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	caller, release, err := s.host.Acquire(callCtx, pluginID)
	if err != nil {
		if release != nil {
			release()
		}
		cancel()
		return nil, plugin.WrapRPCError(err)
	}
	return &storageCall{pluginID: pluginID, code: code, caller: caller, release: release, cancel: cancel, ctx: callCtx}, nil
}

// storageError maps a failed call: CodeInvalidConfig names the field of the
// backend form, anything else is a plugin rpc error.
func storageError(err error) error {
	if field, message, invalid := invalidConfigField(err); invalid {
		return cosy.WrapErrorWithParams(plugin.ErrStorageConfigInvalid, field, message)
	}
	return plugin.WrapRPCError(err)
}

// Validate asks the owning plugin to check a configuration. Only a
// CodeInvalidConfig answer rejects it: a plugin that does not implement
// storage.validate or cannot be reached right now has no opinion.
func (s *storageSource) Validate(ctx context.Context, storageType string, config map[string]string) error {
	call, err := s.begin(ctx, storageType, storageCallTimeout)
	if err != nil {
		if sameCosyError(err, plugin.ErrStorageBackendUnavailable) {
			return err
		}
		logger.Warnf("cannot validate storage backend %s: %v", storageType, err)
		return nil
	}
	defer call.close()

	err = call.caller.Call(call.ctx, protocol.MethodStorageValidate, protocol.StorageValidateParams{
		Backend: call.code,
		Config:  config,
	}, nil)
	switch {
	case err == nil, isUnimplemented(err):
		return nil
	}
	if field, message, invalid := invalidConfigField(err); invalid {
		return cosy.WrapErrorWithParams(plugin.ErrStorageConfigInvalid, field, message)
	}
	logger.Warnf("[plugin:%s] validate storage backend %s: %s", call.pluginID, call.code, rpcMessage(err))
	return nil
}

// Put places the file in a fresh exchange directory of the plugin and asks it
// to store the file under key.
func (s *storageSource) Put(ctx context.Context, storageType string, config map[string]string, key, sourcePath string) (int64, error) {
	call, err := s.begin(ctx, storageType, storageTransferTimeout)
	if err != nil {
		return 0, err
	}
	defer call.close()

	dir, cleanup, err := newExchangeDir(s.host.DataDir(call.pluginID))
	if err != nil {
		return 0, err
	}
	defer cleanup()

	staged := filepath.Join(dir, exchangeFileName(key))
	if err = linkOrCopy(sourcePath, staged); err != nil {
		return 0, fmt.Errorf("stage %s for the plugin: %w", filepath.Base(sourcePath), err)
	}

	var result protocol.StorageSizeResult
	if err = call.caller.Call(call.ctx, protocol.MethodStoragePut, protocol.StoragePutParams{
		Backend:    call.code,
		Config:     config,
		Key:        key,
		SourcePath: staged,
	}, &result); err != nil {
		return 0, storageError(err)
	}
	return int64(result.Size), nil
}

// Get asks the plugin to write the object to a fresh exchange directory and
// moves the file to targetPath.
func (s *storageSource) Get(ctx context.Context, storageType string, config map[string]string, key, targetPath string) (int64, error) {
	call, err := s.begin(ctx, storageType, storageTransferTimeout)
	if err != nil {
		return 0, err
	}
	defer call.close()

	dir, cleanup, err := newExchangeDir(s.host.DataDir(call.pluginID))
	if err != nil {
		return 0, err
	}
	defer cleanup()

	fetched := filepath.Join(dir, exchangeFileName(key))
	var result protocol.StorageSizeResult
	if err = call.caller.Call(call.ctx, protocol.MethodStorageGet, protocol.StorageGetParams{
		Backend:    call.code,
		Config:     config,
		Key:        key,
		TargetPath: fetched,
	}, &result); err != nil {
		return 0, storageError(err)
	}

	info, err := os.Lstat(fetched)
	if err != nil || !info.Mode().IsRegular() {
		return 0, fmt.Errorf("plugin %s did not write %s as a regular file", call.pluginID, key)
	}
	if err = moveFile(fetched, targetPath); err != nil {
		return 0, fmt.Errorf("take over %s from the plugin: %w", key, err)
	}
	return info.Size(), nil
}

// List asks the plugin for the objects whose key starts with prefix.
func (s *storageSource) List(ctx context.Context, storageType string, config map[string]string, prefix string) ([]backup.StoredObject, error) {
	call, err := s.begin(ctx, storageType, storageCallTimeout)
	if err != nil {
		return nil, err
	}
	defer call.close()

	var result protocol.StorageListResult
	if err = call.caller.Call(call.ctx, protocol.MethodStorageList, protocol.StorageListParams{
		Backend: call.code,
		Config:  config,
		Prefix:  prefix,
	}, &result); err != nil {
		return nil, storageError(err)
	}

	objects := make([]backup.StoredObject, 0, len(result.Objects))
	for _, object := range result.Objects {
		stored := backup.StoredObject{Key: object.Key, Size: int64(object.Size)}
		if object.ModifiedAt != "" {
			if modified, parseErr := time.Parse(time.RFC3339, object.ModifiedAt); parseErr == nil {
				stored.ModifiedAt = modified
			}
		}
		objects = append(objects, stored)
	}
	return objects, nil
}

// Delete asks the plugin to remove the object stored under key.
func (s *storageSource) Delete(ctx context.Context, storageType string, config map[string]string, key string) error {
	call, err := s.begin(ctx, storageType, storageCallTimeout)
	if err != nil {
		return err
	}
	defer call.close()

	if err = call.caller.Call(call.ctx, protocol.MethodStorageDelete, protocol.StorageDeleteParams{
		Backend: call.code,
		Config:  config,
		Key:     key,
	}, nil); err != nil {
		return storageError(err)
	}
	return nil
}

// exchangeFileName names the exchanged file after the last key segment, so
// the plugin sees a meaningful name.
func exchangeFileName(key string) string {
	name := path.Base(key)
	if name == "." || name == "/" || name == ".." || name == "" {
		return "object"
	}
	return name
}

// newExchangeDir creates <dataDir>/exchange/<random> with mode 0700 and
// returns a func that removes it. It also removes directories an earlier
// run of the host left behind.
func newExchangeDir(dataDir string) (string, func(), error) {
	if dataDir == "" {
		return "", nil, errors.New("the plugin has no data directory")
	}
	root := filepath.Join(dataDir, exchangeDirName)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", nil, fmt.Errorf("create exchange directory: %w", err)
	}
	sweepExchange(root)

	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", nil, err
	}
	dir := filepath.Join(root, hex.EncodeToString(suffix[:]))
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", nil, fmt.Errorf("create exchange directory: %w", err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// sweepExchange removes exchange directories older than exchangeLeftoverAge.
func sweepExchange(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-exchangeLeftoverAge)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
}

// linkOrCopy makes src available at dst without touching src: a hard link
// when both are on one file system, a copy otherwise.
func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

// moveFile renames src to dst, copying across file systems.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}
