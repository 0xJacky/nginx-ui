package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/host/setup"
	"github.com/urfave/cli/v3"
)

var HostSetupCommand = &cli.Command{
	Name:  "host-setup",
	Usage: "Generate or verify configuration for host_via_ssh nginx control mode",
	Commands: []*cli.Command{
		hostSetupPrintCommand(),
		hostSetupKeygenCommand(),
		hostSetupTestCommand(),
	},
}

func hostSetupPrintCommand() *cli.Command {
	return &cli.Command{
		Name:  "print",
		Usage: "Print setup snippets (compose / override / docker-run / authorized_keys / sudoers / acl)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "host-address", Usage: "remote address host:port"},
			&cli.StringFlag{Name: "host-user", Usage: "remote user"},
			&cli.StringFlag{Name: "systemd-unit", Usage: "systemd unit name", Value: "nginx.service"},
			&cli.StringFlag{Name: "systemctl-path", Usage: "absolute path to systemctl on host", Value: "/bin/systemctl"},
			&cli.StringFlag{Name: "nginx-sbin", Usage: "absolute path to nginx on host", Value: "/usr/sbin/nginx"},
			&cli.StringFlag{Name: "config-dir", Usage: "host nginx config dir", Value: "/etc/nginx"},
			&cli.StringFlag{Name: "log-dir", Usage: "host nginx log dir", Value: "/var/log/nginx"},
			&cli.StringFlag{Name: "public-key", Usage: "path to public key file"},
			&cli.BoolFlag{Name: "compose", Usage: "print only the compose snippet"},
			&cli.BoolFlag{Name: "override", Usage: "print only the full override file"},
			&cli.BoolFlag{Name: "docker-run", Usage: "print only the docker run command"},
			&cli.BoolFlag{Name: "host", Usage: "print only host-side snippets (sudoers + authorized_keys + acl)"},
			&cli.BoolFlag{Name: "json", Usage: "JSON output"},
		},
		Action: hostSetupPrint,
	}
}

func hostSetupKeygenCommand() *cli.Command {
	return &cli.Command{
		Name:  "keygen",
		Usage: "Generate an ed25519 keypair for host SSH",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "out", Required: true, Usage: "private key output path"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			pub, err := setup.GenerateKeypair(c.String("out"))
			if err != nil {
				return err
			}
			fmt.Println(pub)
			return nil
		},
	}
}

func hostSetupTestCommand() *cli.Command {
	return &cli.Command{
		Name:   "test",
		Usage:  "Run the verify pipeline against current settings; print JSON result",
		Action: hostSetupTest,
	}
}

func paramsFromFlags(c *cli.Command) (setup.SetupParams, error) {
	p := setup.SetupParams{
		HostAddress:   c.String("host-address"),
		HostUser:      c.String("host-user"),
		SystemdUnit:   c.String("systemd-unit"),
		SystemctlPath: c.String("systemctl-path"),
		NginxSbinPath: c.String("nginx-sbin"),
		HostConfigDir: c.String("config-dir"),
		HostLogDir:    c.String("log-dir"),
	}
	if addr := c.String("host-address"); strings.HasPrefix(addr, "host.docker.internal") {
		p.UseHostGateway = true
	}
	if pubPath := c.String("public-key"); pubPath != "" {
		raw, err := os.ReadFile(pubPath)
		if err != nil {
			return p, err
		}
		p.PublicKeyOpenSSH = strings.TrimSpace(string(raw))
	}
	return p, nil
}

func hostSetupPrint(ctx context.Context, c *cli.Command) error {
	p, err := paramsFromFlags(c)
	if err != nil {
		return err
	}
	r, err := setup.RenderAll(p)
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(r)
	}
	if c.Bool("compose") {
		fmt.Print(r.ComposeSnippet)
		return nil
	}
	if c.Bool("override") {
		fmt.Print(r.ComposeOverride)
		return nil
	}
	if c.Bool("docker-run") {
		fmt.Print(r.DockerRun)
		return nil
	}
	if c.Bool("host") {
		fmt.Print("# --- /etc/sudoers.d/nginx-ui ---\n")
		fmt.Print(r.Sudoers)
		fmt.Print("\n# --- authorized_keys ---\n")
		fmt.Print(r.AuthorizedKeys)
		fmt.Print("\n# --- ACL commands ---\n")
		fmt.Print(r.ACLCommands)
		return nil
	}
	// Default: everything, sectioned.
	fmt.Print("# === docker-compose snippet ===\n", r.ComposeSnippet)
	fmt.Print("\n# === docker-compose.override.yml ===\n", r.ComposeOverride)
	fmt.Print("\n# === docker run equivalent ===\n", r.DockerRun)
	fmt.Print("\n# === host: /etc/sudoers.d/nginx-ui ===\n", r.Sudoers)
	fmt.Print("\n# === host: authorized_keys ===\n", r.AuthorizedKeys)
	fmt.Print("\n# === host: ACL commands ===\n", r.ACLCommands)
	return nil
}

// hostSetupTest is a stub here — Task 14 wires it through the shared
// setup.NewClientFromSettings + setup.Verify helpers introduced in that task.
func hostSetupTest(ctx context.Context, c *cli.Command) error {
	return fmt.Errorf("host-setup test: not yet wired to live settings; use the Web UI verify endpoint until Task 14 lands")
}
