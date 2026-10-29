package cert

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

func IsCertificate(pemStr string) bool {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return false
	}
	_, err := x509.ParseCertificate(block.Bytes)
	return err == nil
}

func IsPrivateKey(pemStr string) bool {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return false
	}

	_, errRSA := x509.ParsePKCS1PrivateKey(block.Bytes)
	if errRSA == nil {
		return true
	}

	_, errECDSA := x509.ParseECPrivateKey(block.Bytes)
	if errECDSA == nil {
		return true
	}

	_, errPKC := x509.ParsePKCS8PrivateKey(block.Bytes)
	return errPKC == nil
}

// maxCertificateFileSize bounds how much of a certificate or key file is read
// from a stored path. PEM chains and keys are a few KiB, so 1 MiB leaves ample
// room while keeping a request from buffering an arbitrarily large file.
const maxCertificateFileSize = 1 << 20

var (
	// errExternalCertificatePath marks a path outside the nginx configuration
	// directory. Such files are managed by other means and never inspected.
	errExternalCertificatePath = errors.New("certificate path is outside the nginx configuration directory")
	// errUnreadableCertificateFile marks a directory, device, FIFO, socket or
	// a file larger than maxCertificateFileSize.
	errUnreadableCertificateFile = errors.New("certificate path is not a regular file of bounded size")
)

// readManagedCertificateFile reads a certificate or key file that lives in the
// nginx configuration directory.
//
// A path outside that directory is not touched at all and yields
// errExternalCertificatePath: probing it would let a caller learn whether
// arbitrary host paths exist, and reading a device or FIFO there could block
// the request or exhaust memory. Inside the directory only a regular file of
// at most maxCertificateFileSize is read; a missing file yields an error that
// satisfies errors.Is(err, os.ErrNotExist).
func readManagedCertificateFile(path string) ([]byte, error) {
	confDir := nginx.GetConfPath()
	// The lexical check comes first because IsUnderDirectory resolves
	// symlinks, which stats every component of the path, and a path outside
	// the configuration directory must not be touched at all. IsUnderDirectory
	// then catches a link inside the directory that points elsewhere.
	if !isLexicallyUnderDirectory(path, confDir) || !helper.IsUnderDirectory(path, confDir) {
		return nil, errExternalCertificatePath
	}

	info, err := nginx.Stat(path)
	if err != nil {
		return nil, err
	}
	// Reject devices, FIFOs, sockets and directories before opening them:
	// opening a FIFO blocks and a device never reaches EOF.
	if !info.Mode().IsRegular() || info.Size() > maxCertificateFileSize {
		return nil, errUnreadableCertificateFile
	}

	f, err := nginx.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// The file may change between Stat and Open, so the read is bounded too.
	content, err := io.ReadAll(io.LimitReader(f, maxCertificateFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxCertificateFileSize {
		return nil, errUnreadableCertificateFile
	}
	return content, nil
}

// isLexicallyUnderDirectory reports whether the cleaned absolute path lies in
// directory, comparing names only. The directory is also compared in its
// symlink-resolved form (it is the configuration root, so resolving it stays
// inside the boundary), which keeps paths spelled through either form, such
// as /var and /private/var on macOS, recognized as inside.
func isLexicallyUnderDirectory(path, directory string) bool {
	if directory == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(directory)
	if err != nil {
		return false
	}
	roots := []string{absDir}
	if resolved, err := filepath.EvalSymlinks(absDir); err == nil && resolved != absDir {
		roots = append(roots, resolved)
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, absPath)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// isStorableCertificatePath reports whether path may be stored on a
// certificate record: it is outside the nginx configuration directory
// (externally managed, accepted without inspection), does not exist yet (it
// is written on save), or holds content that isPEM accepts.
func isStorableCertificatePath(path string, isPEM func(string) bool) bool {
	if path == "" {
		return false
	}
	content, err := readManagedCertificateFile(path)
	switch {
	case err == nil:
		return isPEM(string(content))
	case errors.Is(err, errExternalCertificatePath), errors.Is(err, os.ErrNotExist):
		return true
	default:
		return false
	}
}

// IsCertificatePath reports whether path may be stored as a certificate path.
func IsCertificatePath(path string) bool {
	return isStorableCertificatePath(path, IsCertificate)
}

// IsPrivateKeyPath reports whether path may be stored as a private key path.
func IsPrivateKeyPath(path string) bool {
	return isStorableCertificatePath(path, IsPrivateKey)
}

// GetKeyType determines the key type from a PEM certificate string.
// Returns "2048", "3072", "4096", "8192", "P256", "P384" or empty string.
func GetKeyType(pemStr string) (string, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return "", ErrCertDecode
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", ErrCertParse
	}

	switch cert.PublicKeyAlgorithm {
	case x509.RSA:
		rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return "", nil
		}
		keySize := rsaKey.Size() * 8 // Size returns size in bytes, convert to bits
		switch keySize {
		case 2048:
			return "2048", nil
		case 3072:
			return "3072", nil
		case 4096:
			return "4096", nil
		case 8192:
			return "8192", nil
		default:
			return "", nil
		}
	case x509.ECDSA:
		ecKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return "", nil
		}
		curve := ecKey.Curve.Params().Name
		switch curve {
		case "P-256":
			return "P256", nil
		case "P-384":
			return "P384", nil
		default:
			return "", nil
		}
	default:
		return "", nil
	}
}

// GetKeyTypeFromPath determines the key type from a certificate file.
// Returns "2048", "3072", "4096", "8192", "P256", "P384" or empty string.
func GetKeyTypeFromPath(path string) (string, error) {
	if path == "" {
		return "", ErrCertPathIsEmpty
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return GetKeyType(string(bytes))
}
