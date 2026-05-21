package nginx

import (
	"context"
	"os"
	"sync"

	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// sshOnce ensures we share a single long-lived SSH client across all Exec calls.
//
// LIMITATION: if settings.NginxSettings.Host* fields change at runtime (e.g.
// the user saves new SSH config via the Web UI), the cached client is NOT
// rebuilt with the new settings. Settings handler must call ResetSSHClient()
// after writes that affect host SSH config. See spec §6.3.
var (
	sshOnce   sync.Once
	sshShared *hostssh.Client
)

func newSSHRunner() Runner {
	sshOnce.Do(func() {
		opts := buildSSHOptions()
		sshShared = hostssh.NewClient(opts)
	})
	return &sshRunner{client: sshShared}
}

// ResetSSHClient invalidates the cached SSH client so the next nginx command
// re-dials with the current settings. Safe to call concurrently with Exec.
func ResetSSHClient() {
	if sshShared != nil {
		_ = sshShared.Close()
	}
	sshOnce = sync.Once{}
	sshShared = nil
}

func buildSSHOptions() hostssh.ClientOptions {
	n := settings.NginxSettings

	kh, _ := hostssh.NewKnownHosts(n.HostKnownHostsPath)

	password := ""
	if n.HostAuthMethod == "password" {
		logger.Warn("SSH password auth is configured but not yet supported (pending crypto package refactor); SSH connections will fail until you switch to key auth")
	}
	_ = n.HostPasswordRef // suppress unused-field lint until decryption lands

	sudo := n.HostSudoPrefix
	if sudo == "" {
		sudo = "sudo -n"
	}
	systemctl := n.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}

	// Default to strict host key checking unless explicitly disabled.
	// This is critical for the TOFU security model: silently accepting any
	// host key on first connection would allow MITM substitution.
	// Since we cannot distinguish "unset bool" from "explicitly false" with
	// a plain bool field, we default to true here UNLESS the env var
	// NGINX_UI_NGINX_HOST_STRICT_HOST_KEY is explicitly set to "false".
	strict := n.HostStrictHostKey
	if !strict {
		if os.Getenv("NGINX_UI_NGINX_HOST_STRICT_HOST_KEY") != "false" {
			strict = true
		}
	}

	return hostssh.ClientOptions{
		Address:        n.HostAddress,
		User:           n.HostUser,
		AuthMethod:     n.HostAuthMethod,
		PrivateKeyPath: n.HostPrivateKeyPath,
		Password:       password,
		KnownHosts:     kh,
		Strict:         strict,
		Config: hostssh.Config{
			SudoPrefix:    sudo,
			SystemctlPath: systemctl,
			NginxSbinPath: n.SbinPath,
		},
	}
}

type sshRunner struct {
	client *hostssh.Client
}

func (s *sshRunner) Exec(ctx context.Context, name string, args ...string) (string, error) {
	return s.client.Exec(ctx, name, args...)
}

func (s *sshRunner) Stat(path string) bool {
	return s.client.Stat(path)
}
