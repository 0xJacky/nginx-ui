package nginx

import (
	"context"
	"sync"

	hostssh "github.com/0xJacky/Nginx-UI/internal/host/ssh"
	"github.com/0xJacky/Nginx-UI/settings"
)

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

func buildSSHOptions() hostssh.ClientOptions {
	n := settings.NginxSettings

	kh, _ := hostssh.NewKnownHosts(n.HostKnownHostsPath)

	// TODO: decrypt HostPasswordRef via crypto.AesDecrypt when the import cycle
	// internal/nginx → internal/crypto → internal/cache → internal/nginx is resolved.
	// For now, password mode is gated off in the UI (spec §8.2), so passing the
	// raw ref here is acceptable — it will simply fail SSH authentication.
	password := n.HostPasswordRef

	sudo := n.HostSudoPrefix
	if sudo == "" {
		sudo = "sudo -n"
	}
	systemctl := n.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}

	return hostssh.ClientOptions{
		Address:        n.HostAddress,
		User:           n.HostUser,
		AuthMethod:     n.HostAuthMethod,
		PrivateKeyPath: n.HostPrivateKeyPath,
		Password:       password,
		KnownHosts:     kh,
		Strict:         n.HostStrictHostKey,
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
