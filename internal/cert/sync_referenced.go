package cert

import (
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

// ReferencedSyncPayloads returns the certificate pairs a site or stream file
// loads through ssl_certificate and ssl_certificate_key, including the ones
// its included files load, read from disk and
// ready for the /api/cert_sync endpoint of a node. A node rejects the file in
// its Nginx test until these pairs exist there, so they are pushed first.
//
// Pairs that cannot travel through that endpoint are left out: paths built
// from variables, "data:" or "engine:" values, paths outside the Nginx
// configuration directory and files that cannot be read.
func ReferencedSyncPayloads(content string) []*SyncCertificatePayload {
	directives := scanCertificateDirectives(content)
	certPaths, keyPaths := directives.certPaths, directives.keyPaths
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

// certificateDirectives holds the files a configuration loads through its
// certificate directives, with included files spliced in where they are
// included, so a certificate in a snippet still pairs with a key next to the
// include. Paths are resolved against the Nginx configuration directory like
// Nginx resolves them.
type certificateDirectives struct {
	// certPaths and keyPaths follow ssl_certificate and ssl_certificate_key in
	// directive order, so equal positions belong together.
	certPaths []string
	keyPaths  []string
	// caPaths are the CA bundles of ssl_client_certificate and
	// ssl_trusted_certificate.
	caPaths []string
}

func scanCertificateDirectives(content string) certificateDirectives {
	var certValues, keyValues, caValues []string
	config.VisitStatementsWithIncludes("", []byte(content), func(statement []string, _ int) {
		if len(statement) != 2 {
			return
		}
		switch strings.ToLower(statement[0]) {
		case "ssl_certificate":
			certValues = append(certValues, statement[1])
		case "ssl_certificate_key":
			keyValues = append(keyValues, statement[1])
		case "ssl_client_certificate", "ssl_trusted_certificate":
			caValues = append(caValues, statement[1])
		}
	})

	return certificateDirectives{
		certPaths: resolveDirectivePaths(certValues),
		keyPaths:  resolveDirectivePaths(keyValues),
		caPaths:   resolveDirectivePaths(caValues),
	}
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

// ConfiguredCertificatePaths returns the certificate files a configuration
// loads through ssl_certificate, resolved like Nginx resolves them. Values
// that are not plain files, such as variables or "data:" values, are left out.
func ConfiguredCertificatePaths(content string) []string {
	return lo.Uniq(lo.Compact(scanCertificateDirectives(content).certPaths))
}

// UsedCertificatePaths returns every certificate file a configuration loads:
// the server certificates of ssl_certificate and the CA bundles of
// ssl_client_certificate and ssl_trusted_certificate. It answers which files
// use a certificate; renewal and sync stay on ConfiguredCertificatePaths, since
// a CA bundle is never served with its private key.
func UsedCertificatePaths(content string) []string {
	directives := scanCertificateDirectives(content)
	return lo.Uniq(lo.Compact(append(directives.certPaths, directives.caPaths...)))
}

// loadsCertificate reports whether a configuration loads the certificate file.
func loadsCertificate(content, certPath string) bool {
	return lo.Contains(ConfiguredCertificatePaths(content), filepath.Clean(certPath))
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
