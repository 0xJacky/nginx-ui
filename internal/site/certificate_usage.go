package site

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

type CertificateUsageKind string

const (
	CertificateUsageSite   CertificateUsageKind = "site"
	CertificateUsageStream CertificateUsageKind = "stream"
)

// CertificateUsage is a site or stream whose configuration loads a certificate.
type CertificateUsage struct {
	Kind   CertificateUsageKind `json:"kind"`
	Name   string               `json:"name"`
	Status config.Status        `json:"status"`
}

// CertificateUsageIndex maps certificate files to the sites and streams that
// load them through ssl_certificate. Build it once and look up every
// certificate of a list against it, so the configuration files are read once.
type CertificateUsageIndex struct {
	byPath map[string][]CertificateUsage
	// fileInfos caches the stat result of every configured certificate path,
	// filled on the first lookup that needs to compare files.
	fileInfos map[string]os.FileInfo
}

type certificateUsageSource struct {
	kind          CertificateUsageKind
	availableDir  string
	enabledDir    string
	statusBuilder config.StatusMapBuilder
	remoteEnabled map[string]bool
}

// BuildCertificateUsageIndex reads every site and stream configuration and
// records the certificate files each one loads.
func BuildCertificateUsageIndex() *CertificateUsageIndex {
	index := &CertificateUsageIndex{byPath: make(map[string][]CertificateUsage)}
	remoteSites, remoteStreams := remoteDeployStates()

	sources := []certificateUsageSource{
		{
			kind:          CertificateUsageSite,
			availableDir:  "sites-available",
			enabledDir:    "sites-enabled",
			statusBuilder: config.SiteStatusMapBuilder(MaintenanceSuffix),
			remoteEnabled: remoteSites,
		},
		{
			kind:          CertificateUsageStream,
			availableDir:  "streams-available",
			enabledDir:    "streams-enabled",
			statusBuilder: config.DefaultStatusMapBuilder,
			remoteEnabled: remoteStreams,
		},
	}
	for _, source := range sources {
		index.scan(source)
	}

	return index
}

func (index *CertificateUsageIndex) scan(source certificateUsageSource) {
	availablePath := nginx.GetConfPath(source.availableDir)
	entries, err := nginx.ReadDir(availablePath)
	if err != nil {
		logger.Debugf("Skipping certificate usage scan of %s: %v", availablePath, err)
		return
	}
	enabledEntries, err := nginx.ReadDir(nginx.GetConfPath(source.enabledDir))
	if err != nil {
		enabledEntries = nil
	}
	statusMap := source.statusBuilder(entries, enabledEntries)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		content, readErr := nginx.ReadFile(filepath.Join(availablePath, name))
		if readErr != nil {
			logger.Debugf("Skipping certificate usage scan of %s: %v", name, readErr)
			continue
		}

		status := statusMap[name]
		if enabled, ok := source.remoteEnabled[name]; ok {
			status = config.Status(remoteStatus(enabled))
		}
		usage := CertificateUsage{Kind: source.kind, Name: name, Status: status}
		for _, certPath := range cert.ConfiguredCertificatePaths(string(content)) {
			index.byPath[certPath] = append(index.byPath[certPath], usage)
		}
	}
}

// remoteDeployStates returns the deployment intent of every site and stream
// in a remote namespace, keyed by file name. Those never get a local symlink,
// so their status lives in the database.
func remoteDeployStates() (sites, streams map[string]bool) {
	sites = make(map[string]bool)
	streams = make(map[string]bool)
	db := model.UseDB()
	if db == nil {
		return
	}

	var siteModels []*model.Site
	if err := db.Preload("Namespace").Find(&siteModels).Error; err != nil {
		logger.Errorf("Loading sites for certificate usage failed: %v", err)
	}
	for _, siteModel := range siteModels {
		if siteModel.Namespace.IsRemoteDeploy() {
			sites[filepath.Base(siteModel.Path)] = siteModel.RemoteEnabled
		}
	}

	var streamModels []*model.Stream
	if err := db.Preload("Namespace").Find(&streamModels).Error; err != nil {
		logger.Errorf("Loading streams for certificate usage failed: %v", err)
	}
	for _, streamModel := range streamModels {
		if streamModel.Namespace.IsRemoteDeploy() {
			streams[filepath.Base(streamModel.Path)] = streamModel.RemoteEnabled
		}
	}
	return
}

// Lookup returns the sites and streams that load the certificate file, either
// by its path or by another path pointing at the same file.
func (index *CertificateUsageIndex) Lookup(certPath string) []CertificateUsage {
	usages := make([]CertificateUsage, 0)
	if index == nil || certPath == "" {
		return usages
	}
	certPath = filepath.Clean(certPath)

	seen := make(map[CertificateUsage]struct{})
	add := func(items []CertificateUsage) {
		for _, item := range items {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			usages = append(usages, item)
		}
	}
	add(index.byPath[certPath])

	if certInfo, err := nginx.Stat(certPath); err == nil {
		for path, info := range index.statConfiguredPaths() {
			if path != certPath && os.SameFile(info, certInfo) {
				add(index.byPath[path])
			}
		}
	}

	// Files that serve traffic come first, since the list shows only a few.
	sort.Slice(usages, func(i, j int) bool {
		if rank, other := usageStatusRank(usages[i].Status), usageStatusRank(usages[j].Status); rank != other {
			return rank < other
		}
		if usages[i].Kind != usages[j].Kind {
			return usages[i].Kind == CertificateUsageSite
		}
		return usages[i].Name < usages[j].Name
	})
	return usages
}

func usageStatusRank(status config.Status) int {
	switch status {
	case config.StatusEnabled:
		return 0
	case config.StatusMaintenance:
		return 1
	default:
		return 2
	}
}

func (index *CertificateUsageIndex) statConfiguredPaths() map[string]os.FileInfo {
	if index.fileInfos != nil {
		return index.fileInfos
	}
	index.fileInfos = make(map[string]os.FileInfo, len(index.byPath))
	for path := range index.byPath {
		if info, err := nginx.Stat(path); err == nil {
			index.fileInfos[path] = info
		}
	}
	return index.fileInfos
}
