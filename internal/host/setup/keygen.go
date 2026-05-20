package setup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"

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

	if err := os.WriteFile(privateKeyPath, pemBytes, 0o600); err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeyfileWrite, privateKeyPath, err.Error())
	}

	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeygenFailed, err.Error())
	}
	line := string(gossh.MarshalAuthorizedKey(sshPub))
	return strings.TrimSpace(line) + " nginx-ui@generated", nil
}

// LoadPublicKey reads an OpenSSH private key file and returns its public key
// in OpenSSH single-line form. Useful for "show current public key" flows.
func LoadPublicKey(privateKeyPath string) (string, error) {
	raw, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeyfileRead, privateKeyPath, err.Error())
	}
	signer, err := gossh.ParsePrivateKey(raw)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrKeyfileRead, privateKeyPath, err.Error())
	}
	line := string(gossh.MarshalAuthorizedKey(signer.PublicKey()))
	return strings.TrimSpace(line) + " nginx-ui@generated", nil
}
