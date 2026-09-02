//go:build linux

package setup

import (
	"errors"
	"strings"
	"testing"
)

// These tests exercise helpers that only exist in the linux build of the
// verify pipeline (verify.go); the !linux stub does not define them.
func TestCheckLogReadableUsesCrossPlatformRemediation(t *testing.T) {
	outcome := checkLogReadable("/path/that/does/not/exist/access.log")
	if outcome.OK {
		t.Fatalf("missing log unexpectedly passed: %+v", outcome)
	}
	if outcome.Remediation == nil || outcome.Remediation.Code != remediationMountNginxLogs {
		t.Fatalf("unexpected remediation message: %+v", outcome.Remediation)
	}
}

func TestMountedHostPathFor(t *testing.T) {
	mountInfo := []byte(`841 817 0:32 / / rw,relatime - overlay overlay rw
854 841 0:36 /opt/homebrew/etc/nginx /opt/homebrew/etc/nginx rw,relatime - virtiofs mac rw
855 841 0:37 /opt/homebrew/var /opt/homebrew/var rw,relatime - virtiofs mac rw`)

	tests := []struct {
		name   string
		target string
		want   string
		ok     bool
	}{
		{"exact mount", "/opt/homebrew/etc/nginx", "/opt/homebrew/etc/nginx", true},
		{"directory under parent mount", "/opt/homebrew/var/log/nginx", "/opt/homebrew/var/log/nginx", true},
		{"root filesystem only", "/etc/nginx", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mountedHostPathFor(mountInfo, tt.target)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("mountedHostPathFor(%q) = %q, %v; want %q, %v", tt.target, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestVirtualizedMountRootMatches(t *testing.T) {
	for _, tt := range []struct {
		name, mounted, host string
		want                bool
	}{
		{"native root", "/opt/homebrew/etc/nginx", "/opt/homebrew/etc/nginx", true},
		{"vm prefix", "/host_mnt/opt/homebrew/etc/nginx", "/opt/homebrew/etc/nginx", true},
		{"wrong source", "/private/tmp/nginx", "/opt/homebrew/etc/nginx", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := virtualizedMountRootMatches(tt.mounted, tt.host); got != tt.want {
				t.Fatalf("virtualizedMountRootMatches(%q, %q) = %v, want %v", tt.mounted, tt.host, got, tt.want)
			}
		})
	}
}

func TestCheckMacOSHostGateway(t *testing.T) {
	resolved := checkMacOSHostGateway("host.docker.internal:22", func(host string) ([]string, error) {
		if host != "host.docker.internal" {
			t.Fatalf("lookup host = %q", host)
		}
		return []string{"0.250.250.254"}, nil
	})
	if !resolved.OK || !strings.Contains(resolved.Detail, "0.250.250.254") {
		t.Fatalf("resolved macOS gateway did not pass: %+v", resolved)
	}

	unresolved := checkMacOSHostGateway("host.docker.internal:22", func(string) ([]string, error) {
		return nil, errors.New("not found")
	})
	if unresolved.OK || unresolved.Level != "warning" {
		t.Fatalf("unresolved macOS gateway did not warn: %+v", unresolved)
	}

	wrongAlias := checkMacOSHostGateway("192.168.1.10:22", func(string) ([]string, error) {
		return []string{"192.168.1.10"}, nil
	})
	if wrongAlias.OK || wrongAlias.Remediation == nil || wrongAlias.Remediation.Code != remediationUseMacOSHostAlias {
		t.Fatalf("non-standard alias unexpectedly passed: %+v", wrongAlias)
	}
}
