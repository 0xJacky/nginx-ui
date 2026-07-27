package setup

import (
	"encoding/json"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestFillDefaultsLaunchd(t *testing.T) {
	p := (SetupParams{ServiceManager: settings.HostServiceManagerLaunchd}).FillDefaults()
	if p.NginxSbinPath != "/opt/homebrew/bin/nginx" || p.HostConfigDir != "/opt/homebrew/etc/nginx" {
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
