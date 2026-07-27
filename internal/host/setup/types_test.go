package setup

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestFillDefaultsLaunchd(t *testing.T) {
	p := (SetupParams{ServiceManager: settings.HostServiceManagerLaunchd}).FillDefaults()
	if p.NginxSbinPath != "/opt/homebrew/opt/nginx/bin/nginx" || p.HostConfigDir != "/opt/homebrew/etc/nginx" {
		t.Fatalf("unexpected Homebrew defaults: %+v", p)
	}
	if p.PIDPath != "/opt/homebrew/var/run/nginx.pid" || p.PIDDir != "/opt/homebrew/var/run" {
		t.Fatalf("unexpected Homebrew PID defaults: %+v", p)
	}
}

func TestSetupParamsDecodeSnakeCase(t *testing.T) {
	var p SetupParams
	err := json.Unmarshal([]byte(`{"host_address":"host.docker.internal:22","service_manager":"launchd","pid_path":"/tmp/nginx.pid"}`), &p)
	if err != nil {
		t.Fatal(err)
	}
	if p.HostAddress != "host.docker.internal:22" || p.ServiceManager != "launchd" || p.PIDPath != "/tmp/nginx.pid" {
		t.Fatalf("snake_case request did not decode: %+v", p)
	}
}

func TestFillDefaultsDerivesHostGateway(t *testing.T) {
	p := (SetupParams{HostAddress: "host.docker.internal:22"}).FillDefaults()
	if !p.UseHostGateway {
		t.Fatal("expected host.docker.internal to enable host gateway")
	}
}

func TestFillDefaultsPreservesContainerKeyPath(t *testing.T) {
	p := (SetupParams{ContainerKeyPath: "/run/secrets/nginx_ui_ssh"}).FillDefaults()
	if p.ContainerKeyPath != "/run/secrets/nginx_ui_ssh" {
		t.Fatalf("container key path was overwritten: %q", p.ContainerKeyPath)
	}
}

func TestNeedsSudoers(t *testing.T) {
	tests := []struct {
		name string
		p    SetupParams
		want bool
	}{
		{name: "systemd with an unprivileged user", p: SetupParams{ServiceManager: settings.HostServiceManagerSystemd, HostUser: "nginxui"}, want: true},
		{name: "systemd as root", p: SetupParams{ServiceManager: settings.HostServiceManagerSystemd, HostUser: "root"}, want: false},
		{name: "systemd as root with padding", p: SetupParams{ServiceManager: settings.HostServiceManagerSystemd, HostUser: " root "}, want: false},
		{name: "launchd", p: SetupParams{ServiceManager: settings.HostServiceManagerLaunchd, HostUser: "hintay"}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.NeedsSudoers(); got != tc.want {
				t.Fatalf("NeedsSudoers() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateHostUser(t *testing.T) {
	valid := []string{"root", "nginxui", "nginx-ui", "_svc", "a", strings.Repeat("a", 32)}
	for _, user := range valid {
		if err := ValidateHostUser(user); err != nil {
			t.Fatalf("ValidateHostUser(%q) = %v, want nil", user, err)
		}
	}

	invalid := []string{
		"",
		"Root",
		"1user",
		"user name",
		"user;rm -rf /",
		"nginxui ALL=(ALL) NOPASSWD: ALL\n#x",
		"user$(id)",
		strings.Repeat("a", 33),
	}
	for _, user := range invalid {
		if err := ValidateHostUser(user); err == nil {
			t.Fatalf("ValidateHostUser(%q) = nil, want a rejection", user)
		}
	}
}

func TestRenderAllRejectsAnUnsafeHostUser(t *testing.T) {
	p := SetupParams{
		HostAddress:    "host.docker.internal:22",
		HostUser:       "nginxui ALL=(ALL) NOPASSWD: ALL",
		ServiceManager: settings.HostServiceManagerSystemd,
	}
	if _, err := RenderAll(p); !errors.Is(err, ErrInvalidHostUser) {
		t.Fatalf("RenderAll error = %v, want ErrInvalidHostUser", err)
	}
}
