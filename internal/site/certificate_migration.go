package site

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy/logger"
)

type CertificateDeploymentState string

const (
	CertificateDeploymentNotApplicable CertificateDeploymentState = "not_applicable"
	CertificateDeploymentConsistent    CertificateDeploymentState = "consistent"
	CertificateDeploymentLegacyDrift   CertificateDeploymentState = "legacy_drift"
	CertificateDeploymentMismatch      CertificateDeploymentState = "mismatch"
	CertificateDeploymentUnreadable    CertificateDeploymentState = "unreadable"
)

type CertificateDeploymentStatus struct {
	State                       CertificateDeploymentState `json:"state"`
	SiteName                    string                     `json:"site_name,omitempty"`
	ManagedCertificatePath      string                     `json:"managed_certificate_path,omitempty"`
	ManagedCertificateKeyPath   string                     `json:"managed_certificate_key_path,omitempty"`
	ConfiguredCertificatePaths  []string                   `json:"configured_certificate_paths,omitempty"`
	ConfiguredCertificateKeys   []string                   `json:"configured_certificate_key_paths,omitempty"`
	AutomaticMigrationAvailable bool                       `json:"automatic_migration_available"`
	Error                       string                     `json:"error,omitempty"`
}

type certificateMigrationTarget struct {
	path        string
	contentHash string
	replace     []nginx.DirectiveValueReplacement
	siteNames   []string
}

type certificateInspection struct {
	status  CertificateDeploymentStatus
	targets []certificateMigrationTarget
}

type CertificateMigrationResult struct {
	MigratedFiles int
	MigratedSites int
	SkippedFiles  int
}

func InspectCertificateDeployment(certModel *model.Cert) CertificateDeploymentStatus {
	return inspectCertificateDeployment(certModel).status
}

func inspectCertificateDeployment(certModel *model.Cert) certificateInspection {
	status := CertificateDeploymentStatus{State: CertificateDeploymentNotApplicable}
	if certModel == nil {
		return certificateInspection{status: status}
	}

	status.SiteName = certModel.Filename
	status.ManagedCertificatePath = certModel.SSLCertificatePath
	status.ManagedCertificateKeyPath = certModel.SSLCertificateKeyPath
	if certModel.AutoCert != model.AutoCertEnabled || certModel.Filename == "" ||
		certModel.SSLCertificatePath == "" || certModel.SSLCertificateKeyPath == "" ||
		certificateSiteIsRemoteDeploy(certModel.Filename) {
		return certificateInspection{status: status}
	}

	if _, err := cert.ValidateCertificateAndKey(certModel.SSLCertificatePath,
		certModel.SSLCertificateKeyPath); err != nil {
		status.State = CertificateDeploymentUnreadable
		status.Error = err.Error()
		return certificateInspection{status: status}
	}

	paths, err := certificateDeploymentConfigPaths(certModel.Filename)
	if err != nil {
		status.State = CertificateDeploymentUnreadable
		status.Error = err.Error()
		return certificateInspection{status: status}
	}
	if len(paths) == 0 {
		return certificateInspection{status: status}
	}

	legacyCertPath, legacyKeyPath, hasLegacyAlias := legacyCertificatePaths(certModel)
	targets := make([]certificateMigrationTarget, 0, len(paths))
	foundDirectives := false
	foundMismatch := false
	foundLegacyDrift := false

	for _, path := range paths {
		content, readErr := nginx.ReadFile(path)
		if readErr != nil {
			status.State = CertificateDeploymentUnreadable
			status.Error = readErr.Error()
			return certificateInspection{status: status}
		}

		certificatePaths, parseErr := nginx.DirectiveValues(string(content), "ssl_certificate")
		if parseErr != nil {
			status.State = CertificateDeploymentUnreadable
			status.Error = parseErr.Error()
			return certificateInspection{status: status}
		}
		keyPaths, parseErr := nginx.DirectiveValues(string(content), "ssl_certificate_key")
		if parseErr != nil {
			status.State = CertificateDeploymentUnreadable
			status.Error = parseErr.Error()
			return certificateInspection{status: status}
		}

		status.ConfiguredCertificatePaths = append(status.ConfiguredCertificatePaths, certificatePaths...)
		status.ConfiguredCertificateKeys = append(status.ConfiguredCertificateKeys, keyPaths...)
		if len(certificatePaths) == 0 && len(keyPaths) == 0 {
			continue
		}
		foundDirectives = true

		usesManagedFiles := containsPathOrSameFile(certificatePaths, certModel.SSLCertificatePath) &&
			containsPathOrSameFile(keyPaths, certModel.SSLCertificateKeyPath)
		usesLegacyPaths := hasLegacyAlias && containsExactPath(certificatePaths, legacyCertPath) &&
			containsExactPath(keyPaths, legacyKeyPath)
		legacyPathsResolveToManagedFiles := usesLegacyPaths &&
			containsPathOrSameFile([]string{legacyCertPath}, certModel.SSLCertificatePath) &&
			containsPathOrSameFile([]string{legacyKeyPath}, certModel.SSLCertificateKeyPath)
		if legacyPathsResolveToManagedFiles {
			continue
		}
		if usesLegacyPaths {
			if coverageErr := validateLegacyDirectiveCoverage(string(content), legacyCertPath,
				legacyKeyPath, certModel.SSLCertificatePath); coverageErr != nil {
				foundMismatch = true
				status.Error = coverageErr.Error()
				continue
			}
			foundLegacyDrift = true
			targets = append(targets, certificateMigrationTarget{
				path:        path,
				contentHash: hashContent(content),
				siteNames:   []string{certModel.Filename},
				replace: []nginx.DirectiveValueReplacement{
					{Directive: "ssl_certificate", OldValue: legacyCertPath, NewValue: certModel.SSLCertificatePath},
					{Directive: "ssl_certificate_key", OldValue: legacyKeyPath, NewValue: certModel.SSLCertificateKeyPath},
				},
			})
			continue
		}

		if usesManagedFiles {
			continue
		}

		foundMismatch = true
	}

	status.ConfiguredCertificatePaths = uniqueSorted(status.ConfiguredCertificatePaths)
	status.ConfiguredCertificateKeys = uniqueSorted(status.ConfiguredCertificateKeys)
	switch {
	case !foundDirectives:
		status.State = CertificateDeploymentUnreadable
		status.Error = "certificate directives are not present in the site configuration"
	case foundMismatch:
		status.State = CertificateDeploymentMismatch
	case foundLegacyDrift:
		status.State = CertificateDeploymentLegacyDrift
		status.AutomaticMigrationAvailable = true
	default:
		status.State = CertificateDeploymentConsistent
	}

	return certificateInspection{status: status, targets: targets}
}

func certificateSiteIsRemoteDeploy(name string) bool {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return false
	}
	s := query.Site
	siteModels, err := s.Where(s.Path.Eq(path)).Preload(s.Namespace).Find()
	return err == nil && len(siteModels) > 0 && siteModels[0].Namespace.IsRemoteDeploy()
}

func validateLegacyDirectiveCoverage(content, legacyCertPath, legacyKeyPath,
	managedCertPath string) error {
	parsedConfig, err := nginx.ParseNgxConfigByContent(content)
	if err != nil {
		return err
	}

	foundTargetServer := false
	for _, server := range parsedConfig.Servers {
		certificatePaths := make([]string, 0)
		keyPaths := make([]string, 0)
		serverNames := make([]string, 0)
		for _, directive := range server.Directives {
			switch directive.Directive {
			case "ssl_certificate":
				certificatePaths = append(certificatePaths, trimDirectiveValue(directive.Params))
			case "ssl_certificate_key":
				keyPaths = append(keyPaths, trimDirectiveValue(directive.Params))
			case "server_name":
				serverNames = append(serverNames, strings.Fields(directive.Params)...)
			}
		}
		if !containsExactPath(certificatePaths, legacyCertPath) ||
			!containsExactPath(keyPaths, legacyKeyPath) {
			continue
		}
		foundTargetServer = true
		if len(serverNames) == 0 {
			return fmt.Errorf("legacy certificate directives are in a server block without server_name")
		}
		if err = cert.CertificateCoversNames(managedCertPath, serverNames); err != nil {
			return err
		}
	}

	if !foundTargetServer {
		return fmt.Errorf("legacy certificate and key directives are not paired in the same server block")
	}
	return nil
}

func trimDirectiveValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func MigrateLegacyCertificatePaths() (CertificateMigrationResult, error) {
	result := CertificateMigrationResult{}
	c := query.Cert
	certModels, err := c.Where(c.AutoCert.Eq(model.AutoCertEnabled)).Find()
	if err != nil {
		return result, err
	}

	targetsByPath := make(map[string]certificateMigrationTarget)
	legacyCerts := make(map[uint64]*model.Cert)
	for _, certModel := range certModels {
		inspection := inspectCertificateDeployment(certModel)
		switch inspection.status.State {
		case CertificateDeploymentMismatch:
			notifyCertificateDeploymentMismatch(certModel, inspection.status)
		case CertificateDeploymentConsistent, CertificateDeploymentNotApplicable:
			clearCertificateDeploymentIssue(certModel)
		}
		if inspection.status.State != CertificateDeploymentLegacyDrift {
			continue
		}
		legacyCerts[certModel.ID] = certModel
		for _, target := range inspection.targets {
			existing := targetsByPath[target.path]
			if existing.path == "" {
				existing.path = target.path
				existing.contentHash = target.contentHash
			}
			existing.replace = append(existing.replace, target.replace...)
			existing.siteNames = append(existing.siteNames, target.siteNames...)
			targetsByPath[target.path] = existing
		}
	}
	if len(targetsByPath) == 0 {
		return result, nil
	}

	release := config.LockApply()
	defer release()

	paths := make([]string, 0, len(targetsByPath))
	for path := range targetsByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	tx := &config.FileTransaction{}
	migratedSites := make(map[string]struct{})
	for _, path := range paths {
		target := targetsByPath[path]
		content, readErr := nginx.ReadFile(path)
		if readErr != nil {
			return result, config.RollbackError(readErr, tx.Rollback)
		}
		if hashContent(content) != target.contentHash {
			result.SkippedFiles++
			continue
		}

		rewritten, count, rewriteErr := nginx.RewriteDirectiveValues(string(content), target.replace)
		if rewriteErr != nil {
			return result, config.RollbackError(rewriteErr, tx.Rollback)
		}
		if count == 0 || rewritten == string(content) {
			result.SkippedFiles++
			continue
		}
		if historyErr := config.CheckAndCreateHistory(path, rewritten); historyErr != nil {
			return result, config.RollbackError(historyErr, tx.Rollback)
		}
		info, statErr := nginx.Stat(path)
		if statErr != nil {
			return result, config.RollbackError(statErr, tx.Rollback)
		}
		if writeErr := tx.Write(path, []byte(rewritten), info.Mode().Perm()); writeErr != nil {
			return result, config.RollbackError(writeErr, tx.Rollback)
		}
		for _, siteName := range target.siteNames {
			migratedSites[siteName] = struct{}{}
		}
		result.MigratedFiles++
	}

	if tx.Len() == 0 {
		return result, nil
	}
	if err = tx.TestAndReload(); err != nil {
		for _, certModel := range legacyCerts {
			notifyCertificateMigrationFailure(certModel, err)
		}
		return CertificateMigrationResult{SkippedFiles: result.SkippedFiles}, err
	}

	result.MigratedSites = len(migratedSites)
	for _, certModel := range legacyCerts {
		clearCertificateDeploymentIssue(certModel)
	}
	notification.Success("Certificate Paths Migrated",
		"Automatically migrated certificate paths for %{sites} sites", map[string]any{"sites": result.MigratedSites})
	logger.Infof("Migrated legacy certificate paths in %d files for %d sites", result.MigratedFiles, result.MigratedSites)
	return result, nil
}

func notifyCertificateDeploymentMismatch(certModel *model.Cert, status CertificateDeploymentStatus) {
	hash := deploymentIssueHash(status.State, status.SiteName, status.ManagedCertificatePath,
		status.ManagedCertificateKeyPath, strings.Join(status.ConfiguredCertificatePaths, "\x00"),
		strings.Join(status.ConfiguredCertificateKeys, "\x00"))
	if certModel.LastDeploymentIssueHash == hash {
		return
	}

	details := map[string]any{
		"site":             status.SiteName,
		"managed_path":     status.ManagedCertificatePath,
		"configured_paths": strings.Join(status.ConfiguredCertificatePaths, ", "),
	}
	notification.Warning("Certificate Configuration Mismatch",
		"Site %{site} is not using the certificate path managed by Nginx UI", details)
	persistCertificateDeploymentIssue(certModel, hash)
}

func notifyCertificateMigrationFailure(certModel *model.Cert, migrationErr error) {
	hash := deploymentIssueHash(CertificateDeploymentLegacyDrift, certModel.Filename,
		certModel.SSLCertificatePath, certModel.SSLCertificateKeyPath, migrationErr.Error())
	if certModel.LastDeploymentIssueHash == hash {
		return
	}

	notification.Error("Certificate Path Migration Failed",
		"Automatic certificate path migration failed for site %{site}: %{error}", map[string]any{
			"site": certModel.Filename, "error": migrationErr.Error(),
		})
	persistCertificateDeploymentIssue(certModel, hash)
}

func persistCertificateDeploymentIssue(certModel *model.Cert, hash string) {
	if certModel == nil || certModel.ID == 0 || model.UseDB() == nil {
		return
	}
	now := time.Now()
	certModel.LastDeploymentIssueHash = hash
	certModel.LastDeploymentIssueNotifyAt = &now
	if err := model.UseDB().Model(&model.Cert{}).Where("id = ?", certModel.ID).Updates(map[string]any{
		"last_deployment_issue_hash":      hash,
		"last_deployment_issue_notify_at": now,
	}).Error; err != nil {
		logger.Errorf("Persist certificate deployment issue state: %v", err)
	}
}

func clearCertificateDeploymentIssue(certModel *model.Cert) {
	if certModel == nil || certModel.LastDeploymentIssueHash == "" {
		return
	}
	certModel.LastDeploymentIssueHash = ""
	certModel.LastDeploymentIssueNotifyAt = nil
	if certModel.ID == 0 || model.UseDB() == nil {
		return
	}
	if err := model.UseDB().Model(&model.Cert{}).Where("id = ?", certModel.ID).Updates(map[string]any{
		"last_deployment_issue_hash":      "",
		"last_deployment_issue_notify_at": nil,
	}).Error; err != nil {
		logger.Errorf("Clear certificate deployment issue state: %v", err)
	}
}

func deploymentIssueHash(values ...any) string {
	hasher := sha256.New()
	for _, value := range values {
		_, _ = fmt.Fprintln(hasher, value)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func certificateDeploymentConfigPaths(name string) ([]string, error) {
	status := GetSiteStatus(name)
	if status == StatusDisabled {
		return nil, nil
	}

	availablePath, err := ResolveAvailablePath(name)
	if err != nil {
		return nil, err
	}
	paths := []string{availablePath}
	if status == StatusMaintenance {
		maintenancePath, resolveErr := ResolveEnabledMaintenancePath(name)
		if resolveErr != nil {
			return nil, resolveErr
		}
		paths = append(paths, maintenancePath)
	}
	return paths, nil
}

func legacyCertificatePaths(certModel *model.Cert) (string, string, bool) {
	if certModel == nil {
		return "", "", false
	}
	certDir := filepath.Dir(certModel.SSLCertificatePath)
	keyDir := filepath.Dir(certModel.SSLCertificateKeyPath)
	if certDir != keyDir || !helper.IsUnderDirectory(certDir, nginx.GetConfPath("ssl")) {
		return "", "", false
	}

	aliases := helper.GetKeyTypeAliases(certcrypto.KeyType(certModel.KeyType))
	if len(aliases) < 2 {
		return "", "", false
	}
	canonical := string(helper.GetKeyType(certcrypto.KeyType(certModel.KeyType)))
	legacy := ""
	for _, alias := range aliases {
		if string(alias) != canonical {
			legacy = string(alias)
			break
		}
	}
	if legacy == "" {
		return "", "", false
	}

	dirName := filepath.Base(certDir)
	canonicalSuffix := "_" + canonical
	if !strings.HasSuffix(dirName, canonicalSuffix) {
		return "", "", false
	}
	legacyDir := filepath.Join(filepath.Dir(certDir), strings.TrimSuffix(dirName, canonicalSuffix)+"_"+legacy)
	return filepath.Join(legacyDir, filepath.Base(certModel.SSLCertificatePath)),
		filepath.Join(legacyDir, filepath.Base(certModel.SSLCertificateKeyPath)), true
}

func containsExactPath(paths []string, target string) bool {
	target = filepath.Clean(target)
	for _, path := range paths {
		if filepath.Clean(path) == target {
			return true
		}
	}
	return false
}

func containsPathOrSameFile(paths []string, target string) bool {
	if containsExactPath(paths, target) {
		return true
	}
	targetInfo, err := nginx.Stat(target)
	if err != nil {
		return false
	}
	for _, path := range paths {
		info, statErr := nginx.Stat(path)
		if statErr == nil && os.SameFile(info, targetInfo) {
			return true
		}
	}
	return false
}

func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func uniqueSorted(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		unique[value] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
