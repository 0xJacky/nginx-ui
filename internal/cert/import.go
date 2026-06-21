package cert

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
)

type ImportCertificateOptions struct {
	Name     string             `json:"name"`
	CertPath string             `json:"ssl_certificate_path"`
	KeyPath  string             `json:"ssl_certificate_key_path"`
	Dir      string             `json:"dir"`
	KeyType  certcrypto.KeyType `json:"key_type"`
}

type DiscoveredCertificatePair struct {
	Name                  string             `json:"name,omitempty"`
	Dir                   string             `json:"dir,omitempty"`
	SSLCertificatePath    string             `json:"ssl_certificate_path"`
	SSLCertificateKeyPath string             `json:"ssl_certificate_key_path"`
	Fingerprint           string             `json:"fingerprint"`
	KeyType               certcrypto.KeyType `json:"key_type"`
	CertificateInfo       *Info              `json:"certificate_info,omitempty"`
}

type ScanCertificateResult struct {
	Dir    string
	Name   string
	Pair   *DiscoveredCertificatePair
	Cert   *model.Cert
	Error  error
	Reason string
}

func ImportExistingCertificate(opts ImportCertificateOptions) (*model.Cert, error) {
	pair, err := ResolveExistingCertificate(opts)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = pair.Name
	}
	if name == "" {
		name = filepath.Base(filepath.Dir(pair.SSLCertificatePath))
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		return nil, fmt.Errorf("certificate name is required")
	}

	db := model.UseDB()
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	certModel := &model.Cert{Name: name}
	updates := &model.Cert{
		Name:                  name,
		Domains:               domainsFromInfo(pair.CertificateInfo),
		SSLCertificatePath:    pair.SSLCertificatePath,
		SSLCertificateKeyPath: pair.SSLCertificateKeyPath,
		Fingerprint:           pair.Fingerprint,
		KeyType:               pair.KeyType,
		AutoCert:              model.AutoCertDisabled,
	}

	if err := db.Where("name = ?", name).
		Assign(updates).
		FirstOrCreate(certModel).Error; err != nil {
		return nil, err
	}

	return certModel, nil
}

func ResolveExistingCertificate(opts ImportCertificateOptions) (*DiscoveredCertificatePair, error) {
	var pair *DiscoveredCertificatePair
	var err error

	if strings.TrimSpace(opts.Dir) != "" {
		pair, err = DiscoverCertificatePair(opts.Dir)
		if err != nil {
			return nil, err
		}
	} else {
		pair = &DiscoveredCertificatePair{
			SSLCertificatePath:    opts.CertPath,
			SSLCertificateKeyPath: opts.KeyPath,
		}
	}

	pair.Name = strings.TrimSpace(opts.Name)
	pair.SSLCertificatePath, err = absPath(pair.SSLCertificatePath)
	if err != nil {
		return nil, err
	}
	pair.SSLCertificateKeyPath, err = absPath(pair.SSLCertificateKeyPath)
	if err != nil {
		return nil, err
	}

	info, err := ValidateCertificateAndKey(pair.SSLCertificatePath, pair.SSLCertificateKeyPath)
	if err != nil {
		return nil, err
	}
	pair.CertificateInfo = info
	pair.Fingerprint, err = CertificateFingerprintFromPath(pair.SSLCertificatePath)
	if err != nil {
		return nil, err
	}

	if opts.KeyType != "" {
		if !helper.IsValidKeyType(opts.KeyType) {
			return nil, fmt.Errorf("invalid key type: %s", opts.KeyType)
		}
		pair.KeyType = helper.GetKeyType(opts.KeyType)
		return pair, nil
	}

	keyType, err := GetKeyTypeFromPath(pair.SSLCertificatePath)
	if err != nil {
		return nil, err
	}
	if keyType != "" {
		pair.KeyType = helper.GetKeyType(certcrypto.KeyType(keyType))
	}
	if pair.KeyType == "" {
		pair.KeyType = certcrypto.RSA2048
	}

	return pair, nil
}

func DiscoverCertificatePair(dir string) (*DiscoveredCertificatePair, error) {
	dirPath, err := absPath(dir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read certificate directory %s: %w", dirPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("certificate directory %s is not a directory", dirPath)
	}

	certCandidates, keyCandidates, err := findCertificateCandidates(dirPath)
	if err != nil {
		return nil, err
	}

	if len(certCandidates) == 0 && len(keyCandidates) == 0 {
		return nil, fmt.Errorf("no certificate or private key candidates found in %s", dirPath)
	}
	if len(certCandidates) == 0 {
		return nil, fmt.Errorf("no valid certificate candidates found in %s", dirPath)
	}
	if len(keyCandidates) == 0 {
		return nil, fmt.Errorf("no valid private key candidates found in %s", dirPath)
	}
	if len(certCandidates) > 1 {
		return nil, fmt.Errorf("multiple certificate candidates found in %s: %s", dirPath, strings.Join(certCandidates, ", "))
	}
	if len(keyCandidates) > 1 {
		return nil, fmt.Errorf("multiple private key candidates found in %s: %s", dirPath, strings.Join(keyCandidates, ", "))
	}

	pair := &DiscoveredCertificatePair{
		Name:                  filepath.Base(dirPath),
		Dir:                   dirPath,
		SSLCertificatePath:    certCandidates[0],
		SSLCertificateKeyPath: keyCandidates[0],
	}

	resolved, err := ResolveExistingCertificate(ImportCertificateOptions{
		Name:     pair.Name,
		CertPath: pair.SSLCertificatePath,
		KeyPath:  pair.SSLCertificateKeyPath,
	})
	if err != nil {
		return nil, err
	}
	resolved.Dir = dirPath

	return resolved, nil
}

func ScanCertificateDirectories(root string) ([]DiscoveredCertificatePair, error) {
	results, err := ScanCertificateDirectoryResults(root)
	if err != nil {
		return nil, err
	}

	pairs := make([]DiscoveredCertificatePair, 0, len(results))
	for _, result := range results {
		if result.Error == nil && result.Pair != nil {
			pairs = append(pairs, *result.Pair)
		}
	}

	return pairs, nil
}

func ScanCertificateDirectoryResults(root string) ([]ScanCertificateResult, error) {
	rootPath, err := absPath(root)
	if err != nil {
		return nil, err
	}

	results := make([]ScanCertificateResult, 0)
	err = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			results = append(results, ScanCertificateResult{
				Dir:    path,
				Name:   filepath.Base(path),
				Error:  walkErr,
				Reason: walkErr.Error(),
			})
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		hasCandidates, err := directoryHasImportCandidates(path)
		if err != nil {
			results = append(results, ScanCertificateResult{
				Dir:    path,
				Name:   filepath.Base(path),
				Error:  err,
				Reason: err.Error(),
			})
			return nil
		}
		if !hasCandidates {
			return nil
		}

		pair, err := DiscoverCertificatePair(path)
		result := ScanCertificateResult{
			Dir:  path,
			Name: filepath.Base(path),
			Pair: pair,
		}
		if err != nil {
			result.Error = err
			result.Reason = err.Error()
		}
		results = append(results, result)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func ScanCertificateDiscoveryPatterns(patterns []string, newOnly bool) ([]DiscoveredCertificatePair, error) {
	pairs := make([]DiscoveredCertificatePair, 0)
	seenDirs := make(map[string]struct{})

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		if hasGlobMeta(pattern) {
			matches, err := filepath.Glob(filepath.Clean(pattern))
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				dir := match
				info, err := os.Stat(match)
				if err != nil {
					continue
				}
				if !info.IsDir() {
					dir = filepath.Dir(match)
				}
				dirKey := normalizePathForCompare(dir)
				if _, ok := seenDirs[dirKey]; ok {
					continue
				}
				seenDirs[dirKey] = struct{}{}

				pair, err := DiscoverCertificatePair(dir)
				if err == nil {
					pairs = append(pairs, *pair)
				}
			}
			continue
		}

		results, err := ScanCertificateDirectoryResults(pattern)
		if err != nil {
			return nil, err
		}
		for _, result := range results {
			if result.Error != nil || result.Pair == nil {
				continue
			}
			dirKey := normalizePathForCompare(result.Dir)
			if _, ok := seenDirs[dirKey]; ok {
				continue
			}
			seenDirs[dirKey] = struct{}{}
			pairs = append(pairs, *result.Pair)
		}
	}

	if newOnly {
		return FilterNewCertificatePairs(pairs)
	}

	return pairs, nil
}

func FilterNewCertificatePairs(pairs []DiscoveredCertificatePair) ([]DiscoveredCertificatePair, error) {
	db := model.UseDB()
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	var existing []model.Cert
	if err := db.Find(&existing).Error; err != nil {
		return nil, err
	}

	filtered := make([]DiscoveredCertificatePair, 0, len(pairs))
	for _, pair := range pairs {
		if certificatePairIsImported(pair, existing) {
			continue
		}
		filtered = append(filtered, pair)
	}

	return filtered, nil
}

func CertificateFingerprintFromPath(certPath string) (string, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("read certificate %s: %w", certPath, err)
	}

	parsedCert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return "", fmt.Errorf("invalid certificate %s: %w", certPath, err)
	}

	sum := sha256.Sum256(parsedCert.Raw)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateCertificateAndKey(certPath, keyPath string) (*Info, error) {
	if strings.TrimSpace(certPath) == "" {
		return nil, fmt.Errorf("certificate path is required")
	}
	if strings.TrimSpace(keyPath) == "" {
		return nil, fmt.Errorf("private key path is required")
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read certificate %s: %w", certPath, err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key %s: %w", keyPath, err)
	}

	parsedCert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate %s: %w", certPath, err)
	}
	if !IsPrivateKey(string(keyPEM)) {
		return nil, fmt.Errorf("invalid private key %s", keyPath)
	}
	if _, err = tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, fmt.Errorf("certificate and private key do not match: %w", err)
	}

	return infoFromCertificate(parsedCert), nil
}

func certificatePairIsImported(pair DiscoveredCertificatePair, existing []model.Cert) bool {
	name := strings.TrimSpace(pair.Name)
	certPath := normalizePathForCompare(pair.SSLCertificatePath)
	keyPath := normalizePathForCompare(pair.SSLCertificateKeyPath)
	fingerprint := strings.TrimSpace(pair.Fingerprint)

	for _, certModel := range existing {
		if name != "" && strings.EqualFold(name, strings.TrimSpace(certModel.Name)) {
			return true
		}
		if certPath != "" && certPath == normalizePathForCompare(certModel.SSLCertificatePath) {
			return true
		}
		if keyPath != "" && keyPath == normalizePathForCompare(certModel.SSLCertificateKeyPath) {
			return true
		}
		if fingerprint != "" && fingerprint == strings.TrimSpace(certModel.Fingerprint) {
			return true
		}
	}

	return false
}

func findCertificateCandidates(dir string) (certCandidates []string, keyCandidates []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if isCertificateCandidate(dir, entry.Name()) {
			content, readErr := os.ReadFile(path)
			if readErr == nil && IsCertificate(string(content)) {
				certCandidates = append(certCandidates, path)
			}
		}
		if isPrivateKeyCandidate(dir, entry.Name()) {
			content, readErr := os.ReadFile(path)
			if readErr == nil && IsPrivateKey(string(content)) {
				keyCandidates = append(keyCandidates, path)
			}
		}
	}

	sort.Strings(certCandidates)
	sort.Strings(keyCandidates)

	return certCandidates, keyCandidates, nil
}

func directoryHasImportCandidates(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isCertificateCandidate(dir, entry.Name()) || isPrivateKeyCandidate(dir, entry.Name()) {
			return true, nil
		}
	}

	return false, nil
}

func isCertificateCandidate(dir, name string) bool {
	lowerName := strings.ToLower(name)
	baseName := strings.ToLower(filepath.Base(dir))
	exactNames := map[string]struct{}{
		"fullchain.pem":    {},
		"cert.pem":         {},
		"certificate.pem":  {},
		"tls.crt":          {},
		baseName + ".pem":  {},
		baseName + ".crt":  {},
		baseName + ".cer":  {},
		baseName + ".cert": {},
	}
	if _, ok := exactNames[lowerName]; ok {
		return true
	}

	switch strings.ToLower(filepath.Ext(lowerName)) {
	case ".crt", ".cer", ".cert":
		return true
	}

	return false
}

func isPrivateKeyCandidate(dir, name string) bool {
	lowerName := strings.ToLower(name)
	baseName := strings.ToLower(filepath.Base(dir))
	exactNames := map[string]struct{}{
		"privkey.pem":      {},
		"key.pem":          {},
		"private.key":      {},
		"tls.key":          {},
		baseName + ".pem":  {},
		baseName + ".key":  {},
		baseName + ".priv": {},
	}
	if _, ok := exactNames[lowerName]; ok {
		return true
	}

	return strings.EqualFold(filepath.Ext(lowerName), ".key")
}

func parseCertificatePEM(certPEM []byte) (*x509.Certificate, error) {
	for {
		block, rest := pem.Decode(certPEM)
		if block == nil {
			return nil, ErrCertDecode
		}
		if block.Type == "CERTIFICATE" {
			parsed, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, ErrCertParse
			}
			return parsed, nil
		}
		certPEM = rest
	}
}

func infoFromCertificate(c *x509.Certificate) *Info {
	subjectName := c.Subject.CommonName
	if subjectName == "" {
		for _, name := range c.DNSNames {
			if name != "" {
				subjectName = name
				break
			}
		}
	}

	return &Info{
		SubjectName: subjectName,
		IssuerName:  c.Issuer.CommonName,
		NotAfter:    c.NotAfter,
		NotBefore:   c.NotBefore,
	}
}

func domainsFromInfo(info *Info) []string {
	if info == nil || info.SubjectName == "" {
		return nil
	}
	return []string{info.SubjectName}
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func normalizePathForCompare(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

func absPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return filepath.Clean(abs), nil
}
