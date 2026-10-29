package config

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
)

// privateKeyPEMHeader matches every PEM private key label, including RSA, EC,
// PKCS#8, encrypted and OpenSSH keys.
var privateKeyPEMHeader = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)

// IsPrivateKeyMaterial reports whether a file under the configuration root
// holds private key material. The .key data-file extension is reserved for
// keys, so it counts even when the content is not PEM; any other file counts
// when it carries a PEM private key block, such as a combined .pem bundle.
// Certificates, CA bundles and ordinary configuration stay readable.
func IsPrivateKeyMaterial(path string, content []byte) bool {
	if strings.EqualFold(filepath.Ext(path), ".key") {
		return true
	}
	return privateKeyPEMHeader.Match(content)
}

// RedactPrivateKeyBackup returns a copy of a configuration history entry whose
// content is replaced by the shared redaction placeholder when it holds
// private key material. Editing a key file through the configuration editor
// records its previous content as history, so history reads must follow the
// same rule as file reads.
func RedactPrivateKeyBackup(backup *model.ConfigBackup) model.ConfigBackup {
	redacted := *backup
	if IsPrivateKeyMaterial(backup.FilePath, []byte(backup.Content)) {
		redacted.Content = settings.RedactedSensitiveValue
	}
	return redacted
}
