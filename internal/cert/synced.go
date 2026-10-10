package cert

import (
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

// SyncPathsFor tells where this node keeps the certificate of one of its
// configurations: the files a record of the configuration already owns, so
// the configuration keeps loading them, or else the default location.
func SyncPathsFor(request SyncPathsRequest) SyncPaths {
	keyType := helper.GetKeyType(request.KeyType)
	if db := model.UseDB(); db != nil {
		var existing model.Cert
		err := db.Where("filename = ? AND key_type IN ? AND delegated_node_id = 0", request.Name,
			helper.GetKeyTypeAliasStrings(keyType)).First(&existing).Error
		confDir := nginx.GetConfPath()
		if err == nil && existing.SSLCertificatePath != "" && existing.SSLCertificateKeyPath != "" &&
			helper.IsUnderDirectory(existing.SSLCertificatePath, confDir) &&
			helper.IsUnderDirectory(existing.SSLCertificateKeyPath, confDir) {
			return SyncPaths{
				SSLCertificatePath:    existing.SSLCertificatePath,
				SSLCertificateKeyPath: existing.SSLCertificateKeyPath,
			}
		}
	}
	payload := &ConfigPayload{ServerName: request.ServerName, KeyType: keyType}
	return SyncPaths{
		SSLCertificatePath:    payload.GetCertificatePath(),
		SSLCertificateKeyPath: payload.GetCertificateKeyPath(),
	}
}

// StopRenewingDelegated hands the files of a certificate another instance
// issued for this node over to that instance: records of this node that renew
// the same files stop doing so.
func StopRenewingDelegated(certPath, keyPath string) error {
	db := model.UseDB()
	if db == nil {
		return nil
	}
	return db.Model(&model.Cert{}).
		Where("ssl_certificate_path = ? AND ssl_certificate_key_path = ? AND auto_cert = ?",
			certPath, keyPath, model.AutoCertEnabled).
		Update("auto_cert", model.AutoCertSync).Error
}

// RemoveSynced deletes certificate files another instance sent to this node,
// along with their records. Nginx is tested without them first, and they are
// put back when the configuration still loads them.
func RemoveSynced(paths SyncPaths) error {
	confDir := nginx.GetConfPath()
	files := []string{paths.SSLCertificatePath, paths.SSLCertificateKeyPath}
	for _, path := range files {
		if path == "" || !helper.IsUnderDirectory(path, confDir) {
			return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), path, confDir)
		}
	}

	type saved struct {
		path    string
		content []byte
		mode    os.FileMode
	}
	var removed []saved
	for _, path := range files {
		content, err := nginx.ReadFile(path)
		if err != nil {
			continue
		}
		mode := os.FileMode(0o644)
		if info, statErr := nginx.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		if err = nginx.Remove(path); err != nil {
			return err
		}
		removed = append(removed, saved{path: path, content: content, mode: mode})
	}

	if len(removed) > 0 {
		if output, err := nginx.TestConfig(); err != nil {
			for _, file := range removed {
				_ = nginx.WriteFile(file.path, file.content, file.mode)
			}
			message := strings.TrimSpace(output)
			if message == "" {
				message = err.Error()
			}
			return cosy.WrapErrorWithParams(ErrCertificateStillLoaded, message)
		}
	}

	if db := model.UseDB(); db != nil {
		return db.Where("ssl_certificate_path = ? AND ssl_certificate_key_path = ? AND auto_cert = ?",
			paths.SSLCertificatePath, paths.SSLCertificateKeyPath, model.AutoCertSync).
			Delete(&model.Cert{}).Error
	}
	return nil
}
