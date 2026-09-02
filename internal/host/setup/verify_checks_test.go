package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// These tests exercise the individual checks of the verify pipeline. The
// pipeline is built for every OS, so they run on the maintainer's macOS as
// well as in the Linux CI image.
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

// The directory checks are the only ones that need a platform helper, so the
// same test proves the unix implementation passes on a real directory and the
// Windows one degrades to a warning instead of a hard failure.
func TestCheckDirAccessOnEveryPlatform(t *testing.T) {
	dir := t.TempDir()

	outcome := checkDirAccess(dir, true)
	if runtime.GOOS == "windows" {
		if outcome.OK || outcome.Level != "warning" {
			t.Fatalf("windows must warn rather than fail: %+v", outcome)
		}
		if outcome.Remediation == nil || outcome.Remediation.Code != remediationVerifyBindMount {
			t.Fatalf("unexpected remediation: %+v", outcome.Remediation)
		}
	} else if !outcome.OK {
		t.Fatalf("a writable temp dir failed: %+v", outcome)
	}

	missing := checkDirAccess(filepath.Join(dir, "missing"), false)
	if missing.OK || missing.Level != "" {
		t.Fatalf("a missing directory must hard-fail: %+v", missing)
	}
	if missing.Remediation == nil || missing.Remediation.Code != remediationAddBindMount {
		t.Fatalf("unexpected remediation: %+v", missing.Remediation)
	}

	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if notDir := checkDirAccess(file, false); notDir.OK || !strings.Contains(notDir.Detail, "not a directory") {
		t.Fatalf("a file was accepted as a directory: %+v", notDir)
	}
}

// A matching inode proves the bind mount on unix. Windows has no inode to
// compare, so the check must warn instead of failing the wizard.
func TestCheckSharedPathComparesInodes(t *testing.T) {
	dir := t.TempDir()
	p := systemdParams()
	p.HostConfigDir = "/etc/nginx"

	inode, inodeErr := func() (uint64, error) {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		return localInode(info)
	}()

	runner := newFakeRunner().
		on("/usr/bin/stat -c %i /etc/nginx", "12345", nil)
	outcome := checkSharedPath(context.Background(), runner, p, dir, "/etc/nginx")

	if runtime.GOOS == "windows" {
		if inodeErr == nil {
			t.Fatal("windows reported an inode")
		}
		if outcome.OK || outcome.Level != "warning" {
			t.Fatalf("windows must warn rather than fail: %+v", outcome)
		}
		if len(runner.recordedCalls()) != 0 {
			t.Fatalf("the host was probed without a local inode: %v", runner.recordedCalls())
		}
		return
	}
	if inodeErr != nil {
		t.Fatal(inodeErr)
	}
	if outcome.OK || !strings.Contains(outcome.Detail, "different directories") {
		t.Fatalf("a mismatched inode passed: %+v", outcome)
	}

	same := newFakeRunner().on("/usr/bin/stat -c %i /etc/nginx", strconv.FormatUint(inode, 10), nil)
	if outcome := checkSharedPath(context.Background(), same, p, dir, "/etc/nginx"); !outcome.OK {
		t.Fatalf("a matching inode failed: %+v", outcome)
	}
}

// Only Linux has /proc/self/mountinfo. Elsewhere the launchd shared-path check
// must degrade to a warning rather than block the wizard.
func TestCheckVirtualizedSharedPathWarnsWithoutMountTable(t *testing.T) {
	if _, err := os.Stat("/proc/self/mountinfo"); err == nil {
		t.Skip("the host exposes a mount table, so the degraded path is not reachable")
	}
	runner := newFakeRunner().on("/usr/bin/stat -f %HT /opt/homebrew/etc/nginx", "Directory", nil)

	outcome := checkVirtualizedSharedPath(context.Background(), runner, "/etc/nginx", "/opt/homebrew/etc/nginx")
	if outcome.OK || outcome.Level != "warning" {
		t.Fatalf("a missing mount table must warn: %+v", outcome)
	}
	if outcome.Remediation == nil || outcome.Remediation.Code != remediationVerifyBindMount {
		t.Fatalf("unexpected remediation: %+v", outcome.Remediation)
	}
}
