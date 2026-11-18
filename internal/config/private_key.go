package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
)

// privateKeyPEMHeader matches every PEM private key label, including RSA, EC,
// PKCS#8, encrypted and OpenSSH keys, as well as armored PGP private keys.
var privateKeyPEMHeader = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----`)

// secretFileExtensions are data-file extensions reserved for secrets: private
// keys and basic auth password files.
var secretFileExtensions = map[string]struct{}{
	".key":      {},
	".htpasswd": {},
}

// IsSecretFileContent reports whether a file holds secret material judging by
// its extension or content alone. Certificates, CA bundles and ordinary
// configuration stay readable.
func IsSecretFileContent(path string, content []byte) bool {
	if _, ok := secretFileExtensions[strings.ToLower(filepath.Ext(path))]; ok {
		return true
	}
	return privateKeyPEMHeader.Match(content)
}

// IsSecretFile reports whether a file is secret by its extension, its content
// or a secret directive that references it.
func IsSecretFile(path string, content []byte, references SecretReferences) bool {
	if IsSecretFileContent(path, content) {
		return true
	}
	path = filepath.Clean(path)
	if _, ok := references[path]; ok {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		if _, ok := references[resolved]; ok {
			return true
		}
	}
	return opensReferencedFile(path, references)
}

// opensReferencedFile reports whether path opens a referenced file under
// another name. Which names open the same file depends on the file system:
// letter case and Unicode normalization, case folds that change a name's
// length, platform-specific aliases and hard links all do. No rule over the
// names covers every file system, so the file path opens is compared by
// identity with every referenced file, each of which the scan has already
// looked up once.
func opensReferencedFile(path string, references SecretReferences) bool {
	if len(references) == 0 {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	for _, referenced := range references {
		if referenced != nil && os.SameFile(info, referenced) {
			return true
		}
	}
	return false
}

// SecretBackupRedactor returns a transformer for configuration history entries
// that replaces the content of secret files with the shared redaction
// placeholder. Editing a secret file through the configuration editor records
// its previous content as history, so history reads must follow the same rule
// as file reads.
func SecretBackupRedactor(references SecretReferences) func(*model.ConfigBackup) model.ConfigBackup {
	return func(backup *model.ConfigBackup) model.ConfigBackup {
		redacted := *backup
		if IsSecretFile(backup.FilePath, []byte(backup.Content), references) {
			redacted.Content = settings.RedactedSensitiveValue
		}
		return redacted
	}
}
