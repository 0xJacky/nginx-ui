package ssh

import "github.com/uozi-tech/cosy"

var e = cosy.NewErrorScope("host_ssh")

var (
	ErrConnectFailed     = e.New(510001, "ssh connect failed: {0}")
	ErrAuthFailed        = e.New(510002, "ssh authentication failed: {0}")
	ErrHostKeyMismatch   = e.New(510003, "host key verification failed: expected {0}, got {1}")
	ErrSudoNoPassword    = e.New(510004, "sudo requires a password; check /etc/sudoers.d/nginx-ui")
	ErrSystemctlNotFound = e.New(510005, "systemctl not found on remote host")
	ErrCommandTimeout    = e.New(510006, "remote command timed out after {0}")
	ErrSessionFailed     = e.New(510007, "failed to open ssh session: {0}")
	ErrKnownHostsRead    = e.New(510008, "failed to read known_hosts: {0}")
	ErrKnownHostsWrite   = e.New(510009, "failed to write known_hosts: {0}")
)
