package cert

import (
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

// ReferencedSyncPayloads returns the certificate pairs a site or stream file
// loads through ssl_certificate and ssl_certificate_key, read from disk and
// ready for the /api/cert_sync endpoint of a node. A node rejects the file in
// its Nginx test until these pairs exist there, so they are pushed first.
//
// Pairs that cannot travel through that endpoint are left out: paths built
// from variables, "data:" or "engine:" values, paths outside the Nginx
// configuration directory and files that cannot be read.
func ReferencedSyncPayloads(content string) []*SyncCertificatePayload {
	certPaths, keyPaths := certificateDirectivePaths(content)
	if len(certPaths) == 0 {
		return nil
	}

	confPath := nginx.GetConfPath()
	seen := map[string]bool{}
	var payloads []*SyncCertificatePayload
	for i, certPath := range certPaths {
		if certPath == "" || seen[certPath] {
			continue
		}
		seen[certPath] = true

		name, keyPath, keyType := describeCertificatePair(certPath)
		if keyPath == "" {
			// Nginx pairs the directives in order, so without a record the key
			// at the same position belongs to this certificate.
			if i >= len(keyPaths) {
				continue
			}
			keyPath = keyPaths[i]
		}

		if !syncablePath(certPath, confPath) || !syncablePath(keyPath, confPath) {
			logger.Debugf("Skipping certificate sync for a path outside the Nginx conf dir: cert=%q key=%q",
				certPath, keyPath)
			continue
		}

		payload, err := newSyncPayload(name, certPath, keyPath, keyType)
		if err != nil {
			logger.Warnf("Skipping certificate sync for an unreadable pair: cert=%q key=%q error=%v",
				certPath, keyPath, err)
			continue
		}
		payloads = append(payloads, payload)
	}

	return payloads
}

// describeCertificatePair returns the name, key path and key type recorded for
// the certificate file. Without a record the name falls back to the directory
// holding the file and the key path is empty.
func describeCertificatePair(certPath string) (name, keyPath string, keyType certcrypto.KeyType) {
	name = filepath.Base(filepath.Dir(certPath))

	db := model.UseDB()
	if db == nil {
		return
	}

	var record model.Cert
	if err := db.Where("ssl_certificate_path = ?", certPath).
		Order("id desc").First(&record).Error; err != nil {
		return
	}
	if record.Name != "" {
		name = record.Name
	}

	return name, record.SSLCertificateKeyPath, record.GetKeyType()
}

// certificateDirectivePaths returns the files a configuration loads through
// ssl_certificate and ssl_certificate_key, in directive order and resolved
// against the Nginx configuration directory like Nginx resolves them.
func certificateDirectivePaths(content string) (certPaths, keyPaths []string) {
	certValues, err := nginx.DirectiveValues(content, "ssl_certificate")
	if err != nil {
		return nil, nil
	}
	keyValues, err := nginx.DirectiveValues(content, "ssl_certificate_key")
	if err != nil {
		return nil, nil
	}

	return resolveDirectivePaths(certValues), resolveDirectivePaths(keyValues)
}

func resolveDirectivePaths(values []string) []string {
	paths := make([]string, 0, len(values))
	for _, value := range values {
		value = trimQuotes(strings.TrimSpace(value))
		// Keep the position so certificate and key directives stay paired.
		if value == "" || strings.Contains(value, "$") ||
			strings.HasPrefix(value, "data:") || strings.HasPrefix(value, "engine:") {
			paths = append(paths, "")
			continue
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(nginx.GetConfPath(), value)
		}
		paths = append(paths, filepath.Clean(value))
	}
	return paths
}

func trimQuotes(value string) string {
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func syncablePath(path, confPath string) bool {
	return path != "" && helper.IsUnderDirectory(path, confPath)
}

// loadsCertificate reports whether a configuration loads the certificate file.
func loadsCertificate(content, certPath string) bool {
	certPaths, _ := certificateDirectivePaths(content)
	return lo.Contains(certPaths, filepath.Clean(certPath))
}

// referencingNodeIDs returns the sync nodes of every site and stream that
// loads the certificate, including the nodes inherited from its namespace.
// Those nodes serve the certificate, so they need every renewal even when the
// certificate record itself has no sync nodes configured.
func referencingNodeIDs(c *model.Cert) []uint64 {
	db := model.UseDB()
	if db == nil || c.SSLCertificatePath == "" {
		return nil
	}

	type syncedFile struct {
		path    string
		nodeIDs []uint64
	}
	var files []syncedFile

	var sites []*model.Site
	if err := db.Preload("Namespace").Find(&sites).Error; err != nil {
		logger.Errorf("Loading sites for certificate sync failed: %v", err)
	}
	for _, s := range sites {
		files = append(files, syncedFile{path: s.Path, nodeIDs: effectiveSyncNodeIDs(s.SyncNodeIDs, s.Namespace)})
	}

	var streams []*model.Stream
	if err := db.Preload("Namespace").Find(&streams).Error; err != nil {
		logger.Errorf("Loading streams for certificate sync failed: %v", err)
	}
	for _, s := range streams {
		files = append(files, syncedFile{path: s.Path, nodeIDs: effectiveSyncNodeIDs(s.SyncNodeIDs, s.Namespace)})
	}

	var nodeIDs []uint64
	for _, file := range files {
		if len(file.nodeIDs) == 0 {
			continue
		}
		content, err := nginx.ReadFile(file.path)
		if err != nil {
			continue
		}
		if loadsCertificate(string(content), c.SSLCertificatePath) {
			nodeIDs = append(nodeIDs, file.nodeIDs...)
		}
	}

	return lo.Uniq(nodeIDs)
}

func effectiveSyncNodeIDs(own []uint64, namespace *model.Namespace) []uint64 {
	nodeIDs := append([]uint64{}, own...)
	if namespace != nil {
		nodeIDs = append(nodeIDs, namespace.SyncNodeIds...)
	}
	return nodeIDs
}
