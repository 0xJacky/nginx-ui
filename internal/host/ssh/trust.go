package ssh

import (
	"github.com/uozi-tech/cosy"
	gossh "golang.org/x/crypto/ssh"
)

// TrustHostKey parses an OpenSSH-formatted public key string and appends it to
// the known_hosts file at path. Returns ErrKnownHostsWrite on file errors.
func TrustHostKey(path, hostPort, publicKeyOpenSSH string) error {
	kh, err := NewKnownHosts(path)
	if err != nil {
		return err
	}
	parsed, _, _, _, err := gossh.ParseAuthorizedKey([]byte(publicKeyOpenSSH))
	if err != nil {
		return cosy.WrapErrorWithParams(ErrPublicKeyParse, err.Error())
	}
	return kh.Trust(hostPort, parsed)
}
