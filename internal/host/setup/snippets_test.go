package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleParams() SetupParams {
	return SetupParams{
		HostAddress:      "host.docker.internal:22",
		HostUser:         "nginxui",
		UseHostGateway:   true,
		SystemdUnit:      "nginx.service",
		SystemctlPath:    "/bin/systemctl",
		NginxSbinPath:    "/usr/sbin/nginx",
		HostConfigDir:    "/etc/nginx",
		HostLogDir:       "/var/log/nginx",
		UseGeneratedKey:  true,
		PublicKeyOpenSSH: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILABCDEF nginx-ui@generated",
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

func TestRenderLaunchdCompose(t *testing.T) {
	p := SetupParams{
		HostAddress:      "host.docker.internal:22",
		HostUser:         "hintay",
		ServiceManager:   "launchd",
		PublicKeyOpenSSH: "ssh-ed25519 AAAA test",
	}.FillDefaults()
	out, err := RenderCompose(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"NGINX_UI_NGINX_HOST_SERVICE_MANAGER=launchd",
		"NGINX_UI_NGINX_HOST_LAUNCHD_SERVICE=homebrew.mxcl.nginx",
		"/opt/homebrew/var/run:/opt/homebrew/var/run:ro",
		"NGINX_UI_NGINX_PID_PATH=/opt/homebrew/var/run/nginx.pid",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("launchd compose missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "HOST_SYSTEMD_UNIT_NAME") {
		t.Fatalf("launchd compose contains systemd settings:\n%s", out)
	}
}
