package nginx

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
)

// shellCommand receives the GOOS of the target that runs the command, so a
// Windows hosted Nginx UI controlling a Linux container or an SSH host gets
// /bin/sh while a local Windows nginx gets cmd.
func TestShellCommand(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		wantName string
		wantArgs []string
	}{
		{
			name:     "linux target",
			goos:     "linux",
			wantName: "/bin/sh",
			wantArgs: []string{"-c", "nginx -t"},
		},
		{
			name:     "darwin target",
			goos:     "darwin",
			wantName: "/bin/sh",
			wantArgs: []string{"-c", "nginx -t"},
		},
		{
			name:     "windows target",
			goos:     "windows",
			wantName: "cmd",
			wantArgs: []string{"/c", "nginx -t"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args := shellCommand(tt.goos, "nginx -t")
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

// execShell must ask the runner for the target OS, not the local one: a
// Windows hosted Nginx UI controlling a Linux target must not send cmd /c.
func TestExecShellUsesTargetGOOS(t *testing.T) {
	originalResolve := resolveRunner
	t.Cleanup(func() { resolveRunner = originalResolve })

	tests := []struct {
		name string
		goos string
		want string
	}{
		{name: "linux target", goos: "linux", want: "/bin/sh -c nginx -t"},
		{name: "windows target", goos: "windows", want: "cmd /c nginx -t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordingRunner{goos: tt.goos}
			resolveRunner = func() Runner { return runner }

			_, err := execShell("nginx -t")
			assert.NoError(t, err)
			assert.Equal(t, []string{tt.want}, runner.commands)
		})
	}
}

func TestExecShellLocal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell output assertion is Unix-specific")
	}
	originalContainerName := settings.NginxSettings.ContainerName
	settings.NginxSettings.ContainerName = ""
	t.Cleanup(func() {
		settings.NginxSettings.ContainerName = originalContainerName
	})

	out, err := execShell("printf issue-1571")
	assert.NoError(t, err)
	assert.Equal(t, "issue-1571", out)
}

func TestExecShellContextStopsTimedOutCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell timeout command is Unix-specific")
	}
	originalContainerName := settings.NginxSettings.ContainerName
	settings.NginxSettings.ContainerName = ""
	t.Cleanup(func() {
		settings.NginxSettings.ContainerName = originalContainerName
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := execShellContext(ctx, "exec sleep 5")

	assert.Error(t, err)
	assert.Less(t, time.Since(started), time.Second)
}
