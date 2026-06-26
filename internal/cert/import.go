package cert

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
)

type ImportCertificateOptions struct {
	Name     string             `json:"name"`
	CertPath string             `json:"ssl_certificate_path"`
	KeyPath  string             `json:"ssl_certificate_key_path"`
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
		return nil, ErrCertificateNameRequired
	}

	db := model.UseDB()
	if db == nil {
		return nil, ErrDatabaseNotInitialized
	}

	certModel, err := findImportedCertificate(name, pair)
	if err != nil {
		return nil, err
	}
	if certModel == nil {
		certModel = &model.Cert{Name: name}
	}

	updates := map[string]interface{}{
		"name":                     name,
		"domains":                  domainsFromInfo(pair.CertificateInfo),
		"ssl_certificate_path":     pair.SSLCertificatePath,
		"ssl_certificate_key_path": pair.SSLCertificateKeyPath,
		"fingerprint":              pair.Fingerprint,
		"key_type":                 pair.KeyType,
		"auto_cert":                model.AutoCertDisabled,
	}

	if certModel.ID > 0 {
		if err := db.Model(certModel).Updates(updates).Error; err != nil {
			return nil, err
		}
		if err := db.First(certModel, certModel.ID).Error; err != nil {
			return nil, err
		}
		return certModel, nil
	}

	if err := db.Assign(updates).FirstOrCreate(certModel, model.Cert{Name: name}).Error; err != nil {
		return nil, err
	}
	if err := db.First(certModel, certModel.ID).Error; err != nil {
		return nil, err
	}

	return certModel, nil
}

func ResolveExistingCertificate(opts ImportCertificateOptions) (*DiscoveredCertificatePair, error) {
	pair := &DiscoveredCertificatePair{
		SSLCertificatePath:    opts.CertPath,
		SSLCertificateKeyPath: opts.KeyPath,
	}

	pair.Name = strings.TrimSpace(opts.Name)
	var err error
	pair.SSLCertificatePath, err = absPath(pair.SSLCertificatePath)
	if err != nil {
		return nil, err
	}
	pair.SSLCertificateKeyPath, err = absPath(pair.SSLCertificateKeyPath)
	if err != nil {
		return nil, err
	}

	parsedCert, err := validateCertificateAndKey(pair.SSLCertificatePath, pair.SSLCertificateKeyPath)
	if err != nil {
		return nil, err
	}
	pair.CertificateInfo = infoFromCertificate(parsedCert)
	pair.Fingerprint = certificateFingerprint(parsedCert)

	if opts.KeyType != "" {
		if !helper.IsValidKeyType(opts.KeyType) {
			return nil, NewInvalidKeyTypeError(string(opts.KeyType))
		}
		pair.KeyType = helper.GetKeyType(opts.KeyType)
		return pair, nil
	}

	keyType := keyTypeFromCertificate(parsedCert)
	if keyType != "" {
		pair.KeyType = helper.GetKeyType(keyType)
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
		return nil, e.NewWithParams(50040, ErrReadCertificateDirectory.Error(), dirPath, err.Error())
	}
	if !info.IsDir() {
		return nil, e.NewWithParams(50041, ErrCertificateDirectoryNotDirectory.Error(), dirPath)
	}

	certCandidates, keyCandidates, err := findCertificateCandidates(dirPath)
	if err != nil {
		return nil, err
	}

	if len(certCandidates) == 0 && len(keyCandidates) == 0 {
		return nil, e.NewWithParams(50042, ErrNoCertificateOrKeyCandidates.Error(), dirPath)
	}
	if len(certCandidates) == 0 {
		return nil, e.NewWithParams(50043, ErrNoValidCertificateCandidates.Error(), dirPath)
	}
	if len(keyCandidates) == 0 {
		return nil, e.NewWithParams(50044, ErrNoValidPrivateKeyCandidates.Error(), dirPath)
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

func ScanCertificateSSLDirectoryResults() ([]ScanCertificateResult, error) {
	root := nginx.GetConfPath("ssl")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []ScanCertificateResult{}, nil
		}
		return nil, err
	}
	return ScanCertificateDirectoryResults(root)
}

func ScanCertificateSSLDirectory(newOnly bool) ([]DiscoveredCertificatePair, error) {
	pairs, err := ScanCertificateDirectories(nginx.GetConfPath("ssl"))
	if err != nil {
		if os.IsNotExist(err) {
			return []DiscoveredCertificatePair{}, nil
		}
		return nil, err
	}

	if newOnly {
		return FilterNewCertificatePairs(pairs)
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

func FilterNewCertificatePairs(pairs []DiscoveredCertificatePair) ([]DiscoveredCertificatePair, error) {
	db := model.UseDB()
	if db == nil {
		return nil, ErrDatabaseNotInitialized
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
	certPath, err := validatedCertificateFilePath(certPath, "certificate path")
	if err != nil {
		return "", err
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return "", e.NewWithParams(50045, ErrReadCertificate.Error(), certPath, err.Error())
	}

	parsedCert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return "", e.NewWithParams(50046, ErrInvalidCertificate.Error(), certPath, err.Error())
	}

	return certificateFingerprint(parsedCert), nil
}

func ValidateCertificateAndKey(certPath, keyPath string) (*Info, error) {
	parsedCert, err := validateCertificateAndKey(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return infoFromCertificate(parsedCert), nil
}

func validateCertificateAndKey(certPath, keyPath string) (*x509.Certificate, error) {
	certPath, err := validatedCertificateFilePath(certPath, "certificate path")
	if err != nil {
		return nil, err
	}
	keyPath, err = validatedCertificateFilePath(keyPath, "private key path")
	if err != nil {
		return nil, err
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, e.NewWithParams(50045, ErrReadCertificate.Error(), certPath, err.Error())
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, e.NewWithParams(50047, ErrReadPrivateKey.Error(), keyPath, err.Error())
	}

	parsedCert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return nil, e.NewWithParams(50046, ErrInvalidCertificate.Error(), certPath, err.Error())
	}
	if !IsPrivateKey(string(keyPEM)) {
		return nil, e.NewWithParams(50048, ErrInvalidPrivateKey.Error(), keyPath)
	}
	if _, err = tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, e.NewWithParams(50049, ErrCertificateKeyMismatch.Error(), err.Error())
	}

	return parsedCert, nil
}

func validatedCertificateFilePath(path, label string) (string, error) {
	path, err := absPath(path)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", e.NewWithParams(50050, ErrCertificateFieldRequired.Error(), label)
	}
	if !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		return "", ErrCertPathIsNotUnderTheNginxConfDir
	}
	return path, nil
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

func findImportedCertificate(name string, pair *DiscoveredCertificatePair) (*model.Cert, error) {
	db := model.UseDB()
	if db == nil {
		return nil, ErrDatabaseNotInitialized
	}

	var existing []model.Cert
	if err := db.Find(&existing).Error; err != nil {
		return nil, err
	}

	for i := range existing {
		certModel := existing[i]
		if name != "" && strings.EqualFold(name, strings.TrimSpace(certModel.Name)) {
			return &certModel, nil
		}
	}

	pairCertPath := normalizePathForCompare(pair.SSLCertificatePath)
	pairKeyPath := normalizePathForCompare(pair.SSLCertificateKeyPath)
	pairFingerprint := strings.TrimSpace(pair.Fingerprint)

	for i := range existing {
		certModel := existing[i]
		if pairCertPath != "" && pairCertPath == normalizePathForCompare(certModel.SSLCertificatePath) {
			return &certModel, nil
		}
		if pairKeyPath != "" && pairKeyPath == normalizePathForCompare(certModel.SSLCertificateKeyPath) {
			return &certModel, nil
		}
		if pairFingerprint != "" && pairFingerprint == strings.TrimSpace(certModel.Fingerprint) {
			return &certModel, nil
		}
	}

	return nil, nil
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

	sortCertificateCandidates(dir, certCandidates)
	sortPrivateKeyCandidates(dir, keyCandidates)

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

func certificateFingerprint(c *x509.Certificate) string {
	sum := sha256.Sum256(c.Raw)
	return hex.EncodeToString(sum[:])
}

func keyTypeFromCertificate(c *x509.Certificate) certcrypto.KeyType {
	switch c.PublicKeyAlgorithm {
	case x509.RSA:
		rsaKey, ok := c.PublicKey.(*rsa.PublicKey)
		if !ok {
			return ""
		}
		switch rsaKey.Size() * 8 {
		case 2048:
			return certcrypto.RSA2048
		case 3072:
			return certcrypto.RSA3072
		case 4096:
			return certcrypto.RSA4096
		case 8192:
			return certcrypto.RSA8192
		}
	case x509.ECDSA:
		ecKey, ok := c.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return ""
		}
		switch ecKey.Curve.Params().Name {
		case "P-256":
			return certcrypto.EC256
		case "P-384":
			return certcrypto.EC384
		}
	}
	return ""
}

func domainsFromInfo(info *Info) []string {
	if info == nil || info.SubjectName == "" {
		return []string{}
	}
	return []string{info.SubjectName}
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

func sortCertificateCandidates(dir string, candidates []string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := certificateCandidateRank(dir, filepath.Base(candidates[i]))
		right := certificateCandidateRank(dir, filepath.Base(candidates[j]))
		if left == right {
			return candidates[i] < candidates[j]
		}
		return left < right
	})
}

func sortPrivateKeyCandidates(dir string, candidates []string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := privateKeyCandidateRank(dir, filepath.Base(candidates[i]))
		right := privateKeyCandidateRank(dir, filepath.Base(candidates[j]))
		if left == right {
			return candidates[i] < candidates[j]
		}
		return left < right
	})
}

func certificateCandidateRank(dir, name string) int {
	lowerName := strings.ToLower(name)
	baseName := strings.ToLower(filepath.Base(dir))
	switch lowerName {
	case "fullchain.pem":
		return 0
	case "cert.pem":
		return 1
	case "certificate.pem":
		return 2
	case "tls.crt":
		return 3
	case baseName + ".pem":
		return 4
	case baseName + ".crt":
		return 5
	case baseName + ".cer":
		return 6
	case baseName + ".cert":
		return 7
	}
	switch strings.ToLower(filepath.Ext(lowerName)) {
	case ".crt":
		return 8
	case ".cer":
		return 9
	case ".cert":
		return 10
	}
	return 100
}

func privateKeyCandidateRank(dir, name string) int {
	lowerName := strings.ToLower(name)
	baseName := strings.ToLower(filepath.Base(dir))
	switch lowerName {
	case "privkey.pem":
		return 0
	case "key.pem":
		return 1
	case "private.key":
		return 2
	case "tls.key":
		return 3
	case baseName + ".key":
		return 4
	case baseName + ".priv":
		return 5
	case baseName + ".pem":
		return 6
	}
	if strings.EqualFold(filepath.Ext(lowerName), ".key") {
		return 7
	}
	return 100
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
