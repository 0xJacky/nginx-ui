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
	manifestSchemaVersion = 2
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

type ManifestTrust string

const (
	ManifestTrustCurrentServer ManifestTrust = "current_server"
	ManifestTrustPortable      ManifestTrust = "portable"
)

type manifestSignatures struct {
	ServerHMACSHA256   string `json:"server_hmac_sha256"`
	PortableHMACSHA256 string `json:"portable_hmac_sha256"`
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

func writeManifestFiles(baseDir string, manifest Manifest, aesKey []byte) error {
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateManifest, err.Error())
	}

	serverSigningKey, err := deriveBackupSigningKey()
	if err != nil {
		return err
	}
	portableSigningKey, err := deriveBackupSigningKeyFromAESKey(aesKey)
	if err != nil {
		return err
	}
	signatureBytes, err := json.Marshal(manifestSignatures{
		ServerHMACSHA256:   signManifest(manifestBytes, serverSigningKey),
		PortableHMACSHA256: signManifest(manifestBytes, portableSigningKey),
	})
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateManifestSig, err.Error())
	}

	if err := os.WriteFile(filepath.Join(baseDir, ManifestFile), manifestBytes, 0644); err != nil {
		return cosy.WrapErrorWithParams(ErrCreateManifest, err.Error())
	}

	if err := os.WriteFile(filepath.Join(baseDir, ManifestSignatureFile), signatureBytes, 0644); err != nil {
		return cosy.WrapErrorWithParams(ErrCreateManifestSig, err.Error())
	}

	return nil
}

func verifyBackupManifest(baseDir string, aesKey []byte) (ManifestTrust, error) {
	manifest, manifestBytes, signature, err := loadManifest(baseDir)
	if err != nil {
		return "", err
	}

	trust, err := verifyManifestSignatureWithFallback(manifest, manifestBytes, signature, aesKey)
	if err != nil {
		return "", err
	}

	filesByName := make(map[string]ManifestEntry, len(manifest.Files))
	for _, file := range manifest.Files {
		if file.Name != NginxUIZipName && file.Name != NginxZipName {
			return "", cosy.WrapErrorWithParams(ErrInvalidManifest, "unexpected file entry: "+file.Name)
		}
		if _, exists := filesByName[file.Name]; exists {
			return "", cosy.WrapErrorWithParams(ErrInvalidManifest, "duplicate file entry: "+file.Name)
		}
		filesByName[file.Name] = file
	}

	for _, fileName := range requiredManifestFiles {
		entry, ok := filesByName[fileName]
		if !ok {
			return "", cosy.WrapErrorWithParams(ErrMissingManifest, fileName)
		}

		filePath := filepath.Join(baseDir, fileName)
		stat, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", cosy.WrapErrorWithParams(ErrMissingManifest, fileName)
			}
			return "", cosy.WrapErrorWithParams(ErrBackupIntegrity, err.Error())
		}

		if !stat.Mode().IsRegular() || stat.Size() != entry.Size {
			return "", ErrBackupIntegrity
		}

		fileHash, err := calculateFileHash(filePath)
		if err != nil {
			return "", cosy.WrapErrorWithParams(ErrBackupIntegrity, err.Error())
		}

		if fileHash != entry.SHA256 {
			return "", ErrBackupIntegrity
		}
	}

	return trust, nil
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

	if manifest.Schema != 1 && manifest.Schema != manifestSchemaVersion {
		return Manifest{}, nil, "", cosy.WrapErrorWithParams(ErrInvalidManifest, "unsupported schema version")
	}

	return manifest, manifestBytes, strings.TrimSpace(string(signatureBytes)), nil
}

func deriveBackupSigningKeyFromAESKey(aesKey []byte) ([]byte, error) {
	if len(aesKey) == 0 {
		return nil, ErrInvalidAESKey
	}

	sum := sha256.Sum256(append([]byte(manifestKeyContext), aesKey...))
	return sum[:], nil
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

func verifyManifestSignatureWithFallback(manifest Manifest, manifestBytes []byte, signature string, aesKey []byte) (ManifestTrust, error) {
	serverSigningKey, serverKeyErr := deriveBackupSigningKey()
	portableSigningKey, portableKeyErr := deriveBackupSigningKeyFromAESKey(aesKey)

	if manifest.Schema == manifestSchemaVersion {
		var signatures manifestSignatures
		if err := json.Unmarshal([]byte(signature), &signatures); err != nil {
			return "", ErrInvalidManifestSig
		}
		if serverKeyErr == nil && verifyManifestSignature(manifestBytes, signatures.ServerHMACSHA256, serverSigningKey) == nil {
			return ManifestTrustCurrentServer, nil
		}
		if portableKeyErr == nil && verifyManifestSignature(manifestBytes, signatures.PortableHMACSHA256, portableSigningKey) == nil {
			return ManifestTrustPortable, nil
		}
		return "", ErrInvalidManifestSig
	}

	// Schema v1 used a single raw HMAC. AES-derived signatures are portable;
	// server-secret signatures are authoritative for the current installation.
	if portableKeyErr == nil && verifyManifestSignature(manifestBytes, signature, portableSigningKey) == nil {
		return ManifestTrustPortable, nil
	}
	if serverKeyErr == nil && verifyManifestSignature(manifestBytes, signature, serverSigningKey) == nil {
		return ManifestTrustCurrentServer, nil
	}
	return "", ErrInvalidManifestSig
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
