package backup

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

const (
	ManifestFile          = "manifest.json"
	ManifestSignatureFile = "manifest.sig"
	manifestSchemaVersion = 1
	manifestKeyContext    = "nginx-ui-backup-signing-v1:"
)

var requiredManifestFiles = []string{NginxUIZipName, NginxZipName}

type Manifest struct {
	Schema    int             `json:"schema"`
	CreatedAt string          `json:"created_at"`
	Version   string          `json:"version"`
	Files     []ManifestEntry `json:"files"`
}

type ManifestEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func newManifest(createdAt, version string, files []ManifestEntry) Manifest {
	sortedFiles := slices.Clone(files)
	slices.SortFunc(sortedFiles, func(a, b ManifestEntry) int {
		return strings.Compare(a.Name, b.Name)
	})

	return Manifest{
		Schema:    manifestSchemaVersion,
		CreatedAt: createdAt,
		Version:   version,
		Files:     sortedFiles,
	}
}

func writeManifestFiles(baseDir string, manifest Manifest) error {
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateManifest, err.Error())
	}

	signingKey, err := deriveBackupSigningKey()
	if err != nil {
		return err
	}

	signature := signManifest(manifestBytes, signingKey)

	if err := os.WriteFile(filepath.Join(baseDir, ManifestFile), manifestBytes, 0644); err != nil {
		return cosy.WrapErrorWithParams(ErrCreateManifest, err.Error())
	}

	if err := os.WriteFile(filepath.Join(baseDir, ManifestSignatureFile), []byte(signature), 0644); err != nil {
		return cosy.WrapErrorWithParams(ErrCreateManifestSig, err.Error())
	}

	return nil
}

func verifyBackupManifest(baseDir string) error {
	manifest, manifestBytes, signature, err := loadManifest(baseDir)
	if err != nil {
		return err
	}

	signingKey, err := deriveBackupSigningKey()
	if err != nil {
		return err
	}

	if err := verifyManifestSignature(manifestBytes, signature, signingKey); err != nil {
		return err
	}

	filesByName := make(map[string]ManifestEntry, len(manifest.Files))
	for _, file := range manifest.Files {
		filesByName[file.Name] = file
	}

	for _, fileName := range requiredManifestFiles {
		entry, ok := filesByName[fileName]
		if !ok {
			return cosy.WrapErrorWithParams(ErrMissingManifest, fileName)
		}

		filePath := filepath.Join(baseDir, fileName)
		stat, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return cosy.WrapErrorWithParams(ErrMissingManifest, fileName)
			}
			return cosy.WrapErrorWithParams(ErrBackupIntegrity, err.Error())
		}

		if stat.Size() != entry.Size {
			return ErrBackupIntegrity
		}

		fileHash, err := calculateFileHash(filePath)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrBackupIntegrity, err.Error())
		}

		if fileHash != entry.SHA256 {
			return ErrBackupIntegrity
		}
	}

	return nil
}

func loadManifest(baseDir string) (Manifest, []byte, string, error) {
	manifestPath := filepath.Join(baseDir, ManifestFile)
	signaturePath := filepath.Join(baseDir, ManifestSignatureFile)

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, nil, "", ErrUnsupportedFormat
		}
		return Manifest{}, nil, "", cosy.WrapErrorWithParams(ErrReadManifest, err.Error())
	}

	signatureBytes, err := os.ReadFile(signaturePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, nil, "", ErrUnsupportedFormat
		}
		return Manifest{}, nil, "", cosy.WrapErrorWithParams(ErrReadManifestSig, err.Error())
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Manifest{}, nil, "", cosy.WrapErrorWithParams(ErrInvalidManifest, err.Error())
	}

	if manifest.Schema != manifestSchemaVersion {
		return Manifest{}, nil, "", cosy.WrapErrorWithParams(ErrInvalidManifest, "unsupported schema version")
	}

	return manifest, manifestBytes, strings.TrimSpace(string(signatureBytes)), nil
}

func deriveBackupSigningKey() ([]byte, error) {
	secret := strings.TrimSpace(settings.CryptoSettings.Secret)
	if secret == "" {
		return nil, ErrSigningKeyMissing
	}

	sum := sha256.Sum256([]byte(manifestKeyContext + secret))
	return sum[:], nil
}

func signManifest(manifestBytes []byte, signingKey []byte) string {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write(manifestBytes)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyManifestSignature(manifestBytes []byte, signature string, signingKey []byte) error {
	decodedSignature, err := hex.DecodeString(signature)
	if err != nil {
		return ErrInvalidManifestSig
	}

	mac := hmac.New(sha256.New, signingKey)
	mac.Write(manifestBytes)
	if !hmac.Equal(mac.Sum(nil), decodedSignature) {
		return ErrInvalidManifestSig
	}

	return nil
}
