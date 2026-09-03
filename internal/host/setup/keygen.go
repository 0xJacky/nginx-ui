package setup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"

	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/uozi-tech/cosy"
	gossh "golang.org/x/crypto/ssh"
)

// GenerateKeypair creates a new ed25519 keypair, writes the private key
// (OpenSSH format, mode 0600) to privateKeyPath, and returns the public
// key in single-line OpenSSH format suitable for authorized_keys.
//
// Any existing private key file at privateKeyPath is overwritten.
// Parent directories are created with mode 0700.
func GenerateKeypair(privateKeyPath string) (publicKeyOpenSSH string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeygenFailed, err.Error())
	}

	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0o700); err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}

	pemBlock, err := gossh.MarshalPrivateKey(priv, "nginx-ui-host-key")
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeygenFailed, err.Error())
	}
	pemBytes := pem.EncodeToMemory(pemBlock)

	if err := writePrivateKeyFile(privateKeyPath, pemBytes); err != nil {
		return "", err
	}

	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeygenFailed, err.Error())
	}
	line := string(gossh.MarshalAuthorizedKey(sshPub))
	return strings.TrimSpace(line) + " nginx-ui@generated", nil
}

// writePrivateKeyFile replaces the key atomically. os.WriteFile would leave an
// existing file's permissions untouched, so a rotated key could inherit a
// looser mode from whatever was there before.
func writePrivateKeyFile(privateKeyPath string, contents []byte) error {
	dir := filepath.Dir(privateKeyPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}
	tmp, err := os.CreateTemp(dir, ".host-key-*")
	if err != nil {
		return cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}
	if _, err := tmp.Write(contents); err != nil {
		_ = tmp.Close()
		return cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}
	if err := tmp.Close(); err != nil {
		return cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}
	if err := os.Rename(tmpPath, privateKeyPath); err != nil {
		return cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}
	return nil
}

// MaxPrivateKeyFileSize bounds every private key read; see hostssh.MaxPrivateKeyFileSize.
const MaxPrivateKeyFileSize = hostssh.MaxPrivateKeyFileSize

// ReadPrivateKeyFile reads a private key from disk, rejecting anything that is
// not a regular file of a plausible size. It delegates to the SSH client's
// reader and only re-wraps the failure in this package's error scope.
func ReadPrivateKeyFile(privateKeyPath string) ([]byte, error) {
	raw, err := hostssh.ReadPrivateKeyFile(privateKeyPath)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrKeyfileRead, privateKeyPath, err.Error())
	}
	return raw, nil
}

// LoadPublicKey reads an OpenSSH private key file and returns its public key
// in OpenSSH single-line form. Useful for "show current public key" flows.
func LoadPublicKey(privateKeyPath string) (string, error) {
	raw, err := ReadPrivateKeyFile(privateKeyPath)
	if err != nil {
		return "", err
	}
	signer, err := gossh.ParsePrivateKey(raw)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeyfileRead, privateKeyPath, err.Error())
	}
	line := string(gossh.MarshalAuthorizedKey(signer.PublicKey()))
	return strings.TrimSpace(line) + " nginx-ui@generated", nil
}

// SavePrivateKey validates and atomically stores an unencrypted SSH private key.
// The private key is never returned; only its derived public key is exposed.
func SavePrivateKey(privateKeyPath string, raw []byte) (string, error) {
	signer, err := gossh.ParsePrivateKey(raw)
	if err != nil {
		return "", errors.Join(ErrInvalidPrivateKey, err)
	}

	if err := writePrivateKeyFile(privateKeyPath, raw); err != nil {
		return "", err
	}

	line := string(gossh.MarshalAuthorizedKey(signer.PublicKey()))
	return strings.TrimSpace(line) + " nginx-ui@provided", nil
}
