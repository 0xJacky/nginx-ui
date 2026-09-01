package nginx

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
)

func TestShellCommand(t *testing.T) {
	tests := []struct {
		name              string
		goos              string
		externalContainer bool
		wantName          string
		wantArgs          []string
	}{
		{
			name:     "local unix",
			goos:     "linux",
			wantName: "/bin/sh",
			wantArgs: []string{"-c", "nginx -t"},
		},
		{
			name:     "local windows",
			goos:     "windows",
			wantName: "cmd",
			wantArgs: []string{"/c", "nginx -t"},
		},
		{
			name:              "external container from unix",
			goos:              "linux",
			externalContainer: true,
			wantName:          "/bin/sh",
			wantArgs:          []string{"-c", "nginx -t"},
		},
		{
			name:              "external container from windows",
			goos:              "windows",
			externalContainer: true,
			wantName:          "/bin/sh",
			wantArgs:          []string{"-c", "nginx -t"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args := shellCommand(tt.goos, tt.externalContainer, "nginx -t")
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantArgs, args)
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
