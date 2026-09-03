package setup

import (
	"context"
	"strings"
	"testing"
)

func TestDiagnoseHostDarwinHomebrew(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/usr/bin/uname -s":                     "Darwin\n",
		"/usr/bin/uname -m":                     "arm64\n",
		"/bin/launchctl list":                   "863\t0\thomebrew.mxcl.nginx\n",
		"/opt/homebrew/bin/brew --prefix":       "/opt/homebrew\n",
		"/opt/homebrew/bin/brew --prefix nginx": "/opt/homebrew/opt/nginx\n",
		"/opt/homebrew/opt/nginx/bin/nginx -V":  "nginx version: nginx/1.31.3\nconfigure arguments: --conf-path=/opt/homebrew/etc/nginx/nginx.conf --pid-path=/opt/homebrew/var/run/nginx.pid --http-log-path=/opt/homebrew/var/log/nginx/access.log --error-log-path=/opt/homebrew/var/log/nginx/error.log\n",
	}}

	result, err := DiagnoseHost(context.Background(), executor, SetupParams{ServiceManager: "systemd"})
	if err != nil {
		t.Fatal(err)
	}
	if result.OS != "Darwin" || result.Arch != "arm64" || result.ServiceManager != "launchd" {
		t.Fatalf("unexpected platform diagnosis: %+v", result)
	}
	if result.HomebrewPrefix != "/opt/homebrew" || result.Nginx == nil || result.Nginx.Version != "nginx/1.31.3" {
		t.Fatalf("unexpected Homebrew diagnosis: %+v", result)
	}
	if result.LaunchctlPath != "/bin/launchctl" || result.LaunchdService != "homebrew.mxcl.nginx" {
		t.Fatalf("unexpected launchd diagnosis: %+v", result)
	}
}

func TestDiagnoseHostLinux(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/usr/bin/uname -s":            "Linux\n",
		"/usr/bin/uname -m":            "x86_64\n",
		"/usr/bin/systemctl --version": "systemd 257\n",
		"/usr/sbin/nginx -V":           "nginx version: nginx/1.26.3\nconfigure arguments: --conf-path=/etc/nginx/nginx.conf --pid-path=/run/nginx.pid\n",
	}}

	result, err := DiagnoseHost(context.Background(), executor, SetupParams{SystemctlPath: "/usr/bin/systemctl"})
	if err != nil {
		t.Fatal(err)
	}
	if result.OS != "Linux" || result.ServiceManager != "systemd" || result.SystemctlPath != "/usr/bin/systemctl" {
		t.Fatalf("unexpected Linux diagnosis: %+v", result)
	}
	if result.Nginx == nil || result.Nginx.ExecutablePath != "/usr/sbin/nginx" {
		t.Fatalf("unexpected nginx diagnosis: %+v", result)
	}
}

func TestDiagnoseHostRejectsUnavailableUname(t *testing.T) {
	_, err := DiagnoseHost(context.Background(), discoveryExecutor{outputs: map[string]string{}}, SetupParams{})
	if err == nil {
		t.Fatal("expected uname failure")
	}
}

func TestDiagnoseHostKeepsConfiguredNginxPathOnDarwin(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/usr/bin/uname -s":               "Darwin\n",
		"/usr/bin/uname -m":               "arm64\n",
		"/opt/homebrew/bin/brew --prefix": "/opt/homebrew\n",
		"/opt/nginx/sbin/nginx -V":        "nginx version: nginx/1.27.0\nconfigure arguments: --conf-path=/opt/nginx/conf/nginx.conf --pid-path=/opt/nginx/logs/nginx.pid\n",
	}}

	result, err := DiagnoseHost(context.Background(), executor, SetupParams{NginxSbinPath: "/opt/nginx/sbin/nginx"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Nginx == nil {
		t.Fatalf("source build was not discovered: %+v", result)
	}
	if result.Nginx.ExecutablePath != "/opt/nginx/sbin/nginx" {
		t.Fatalf("executable path = %q, want the configured source build", result.Nginx.ExecutablePath)
	}
}

func TestDetectSystemdUnitPrefersTheConfiguredUnit(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/bin/systemctl show openresty.service --property=LoadState": "LoadState=loaded\n",
		"/bin/systemctl show nginx.service --property=LoadState":     "LoadState=loaded\n",
	}}

	if got := detectSystemdUnit(context.Background(), executor, "/bin/systemctl", "openresty"); got != "openresty.service" {
		t.Fatalf("unit = %q, want the configured openresty.service", got)
	}
	if got := detectSystemdUnit(context.Background(), executor, "/bin/systemctl", ""); got != "nginx.service" {
		t.Fatalf("unit = %q, want nginx.service", got)
	}
}

func TestDetectSystemdUnitIgnoresUnloadedUnits(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/bin/systemctl show nginx.service --property=LoadState":     "LoadState=not-found\n",
		"/bin/systemctl show openresty.service --property=LoadState": "LoadState=loaded\n",
	}}

	if got := detectSystemdUnit(context.Background(), executor, "/bin/systemctl", ""); got != "openresty.service" {
		t.Fatalf("unit = %q, want the loaded openresty.service", got)
	}
}

func TestDetectLaunchdServicePrefersHomebrewLabel(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/bin/launchctl list": "-\t0\tcom.example.nginx-helper\n1234\t0\thomebrew.mxcl.nginx\n-\t0\tcom.apple.something\n",
	}}

	if got := detectLaunchdService(context.Background(), executor, "", "/bin/launchctl", ""); got != "homebrew.mxcl.nginx" {
		t.Fatalf("label = %q, want homebrew.mxcl.nginx", got)
	}
}

func TestDetectLaunchdServiceAcceptsAKnownNginxLabel(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/bin/launchctl list": "-\t0\tcom.apple.something\n4321\t0\torg.nginx.nginx\n",
	}}

	if got := detectLaunchdService(context.Background(), executor, "", "/bin/launchctl", ""); got != "org.nginx.nginx" {
		t.Fatalf("label = %q, want org.nginx.nginx", got)
	}
}

func TestDetectLaunchdServiceRejectsAnUnrelatedJob(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/bin/launchctl list": "-\t0\tcom.example.nginx-exporter\n7\t0\tlocal.nginx-logrotate\n",
	}}

	if got := detectLaunchdService(context.Background(), executor, "", "/bin/launchctl", ""); got != "" {
		t.Fatalf("label = %q, want no detection so the operator is asked", got)
	}
}

func TestDetectLaunchdServiceMatchesTheConfiguredLabel(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/bin/launchctl list": "-\t0\tcom.example.custom-nginx\n",
	}}

	if got := detectLaunchdService(context.Background(), executor, "", "/bin/launchctl", "com.example.custom-nginx"); got != "com.example.custom-nginx" {
		t.Fatalf("label = %q, want the configured label", got)
	}
}

func TestDetectLaunchdServicePrefersBrewOverLaunchctl(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/opt/homebrew/bin/brew services info nginx --json": `[{"name":"nginx","service_name":"homebrew.mxcl.nginx","running":true}]`,
		"/bin/launchctl list":                               "-\t0\torg.custom.nginx\n",
	}}

	got := detectLaunchdService(context.Background(), executor, "/opt/homebrew", "/bin/launchctl", "")
	if got != "homebrew.mxcl.nginx" {
		t.Fatalf("label = %q, want the label Homebrew reports", got)
	}
}

func TestDetectLaunchdServiceFallsBackWhenBrewIsUnhelpful(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/opt/homebrew/bin/brew services info nginx --json": "Warning: nginx is not installed\n[]",
		"/bin/launchctl list":                               "-\t0\thomebrew.mxcl.nginx\n",
	}}

	got := detectLaunchdService(context.Background(), executor, "/opt/homebrew", "/bin/launchctl", "")
	if got != "homebrew.mxcl.nginx" {
		t.Fatalf("label = %q, want the launchctl fallback", got)
	}
}

// The architecture probe overlaps the platform probes, so the merged warnings
// must keep their documented order and an unsupported OS must still be
// reported after the architecture warning.
func TestDiagnoseHostKeepsWarningOrderAcrossConcurrentProbes(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/usr/bin/uname -s": "FreeBSD\n",
	}}

	result, err := DiagnoseHost(context.Background(), executor, SetupParams{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Arch != "" {
		t.Fatalf("arch = %q, want empty when uname -m fails", result.Arch)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings = %v, want the architecture and OS warnings", result.Warnings)
	}
	if !strings.HasPrefix(result.Warnings[0], "could not detect target architecture") {
		t.Fatalf("first warning = %q, want the architecture warning", result.Warnings[0])
	}
	if result.Warnings[1] != "unsupported target operating system: FreeBSD" {
		t.Fatalf("second warning = %q, want the OS warning", result.Warnings[1])
	}
}

// On macOS the launchctl and Homebrew probes run at the same time, so both
// missing must still produce both warnings in the documented order.
func TestDiagnoseHostDarwinWarnsForEachMissingProbe(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/usr/bin/uname -s": "Darwin\n",
		"/usr/bin/uname -m": "arm64\n",
	}}

	result, err := DiagnoseHost(context.Background(), executor, SetupParams{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"launchctl was not detected at a standard path",
		"no loaded nginx launchd service was found; enter the service label manually",
		"Homebrew was not detected at /opt/homebrew or /usr/local",
	}
	if len(result.Warnings) < len(want) {
		t.Fatalf("warnings = %v, want at least %v", result.Warnings, want)
	}
	for i, message := range want {
		if result.Warnings[i] != message {
			t.Fatalf("warning[%d] = %q, want %q", i, result.Warnings[i], message)
		}
	}
}
