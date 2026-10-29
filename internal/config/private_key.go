package config

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
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

// secretFileDirectives name the files Nginx reads secrets from. A file one of
// them references is a secret whatever its name or format, which covers DER
// keys, session ticket keys and passphrase files.
var secretFileDirectives = regexp.MustCompile(
	`(?:^|[\s;{}])((?:proxy_|grpc_|uwsgi_)?ssl_certificate_key|(?:proxy_|grpc_|uwsgi_)?ssl_password_file|ssl_session_ticket_key|auth_basic_user_file)\s+("[^"]*"|'[^']*'|[^\s;]+)\s*;`,
)

// maxSecretReferenceScanSize skips files that cannot be ordinary configuration.
const maxSecretReferenceScanSize = 1 << 20

// IsSecretFileContent reports whether a file holds secret material judging by
// its extension or content alone. Certificates, CA bundles and ordinary
// configuration stay readable.
func IsSecretFileContent(path string, content []byte) bool {
	if _, ok := secretFileExtensions[strings.ToLower(filepath.Ext(path))]; ok {
		return true
	}
	return privateKeyPEMHeader.Match(content)
}

// SecretFileReferences collects the absolute paths that Nginx configuration
// under the configuration root names as secret files. Relative paths resolve
// against the configuration root, as Nginx resolves them, and paths built from
// variables or non-file sources such as data: and engine: are skipped.
func SecretFileReferences() map[string]struct{} {
	confRoot := nginx.GetConfPath()
	references := map[string]struct{}{}
	if confRoot == "" {
		return references
	}

	_ = filepath.WalkDir(confRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		if _, isData := dataFileExtensions[strings.ToLower(filepath.Ext(path))]; isData {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > maxSecretReferenceScanSize {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, match := range secretFileDirectives.FindAllSubmatch(stripConfigComments(content), -1) {
			reference := strings.Trim(string(match[2]), `"'`)
			if reference == "" || strings.Contains(reference, "$") || strings.Contains(reference, ":") {
				continue
			}
			if !filepath.IsAbs(reference) {
				reference = filepath.Join(confRoot, reference)
			}
			addSecretReference(references, reference)
		}
		return nil
	})
	return references
}

func addSecretReference(references map[string]struct{}, path string) {
	path = filepath.Clean(path)
	references[path] = struct{}{}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		references[resolved] = struct{}{}
	}
}

// stripConfigComments drops Nginx comments so commented-out directives do not
// mark files as secret.
func stripConfigComments(content []byte) []byte {
	var stripped bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), maxSecretReferenceScanSize)
	for scanner.Scan() {
		line := scanner.Bytes()
		if index := bytes.IndexByte(line, '#'); index >= 0 {
			line = line[:index]
		}
		stripped.Write(line)
		stripped.WriteByte('\n')
	}
	return stripped.Bytes()
}

// IsSecretFile reports whether a file is secret by its extension, its content
// or a secret directive that references it.
func IsSecretFile(path string, content []byte, references map[string]struct{}) bool {
	if IsSecretFileContent(path, content) {
		return true
	}
	path = filepath.Clean(path)
	if _, ok := references[path]; ok {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		_, ok := references[resolved]
		return ok
	}
	return false
}

// SecretBackupRedactor returns a transformer for configuration history entries
// that replaces the content of secret files with the shared redaction
// placeholder. Editing a secret file through the configuration editor records
// its previous content as history, so history reads must follow the same rule
// as file reads.
func SecretBackupRedactor(references map[string]struct{}) func(*model.ConfigBackup) model.ConfigBackup {
	return func(backup *model.ConfigBackup) model.ConfigBackup {
		redacted := *backup
		if IsSecretFile(backup.FilePath, []byte(backup.Content), references) {
			redacted.Content = settings.RedactedSensitiveValue
		}
		return redacted
	}
}
