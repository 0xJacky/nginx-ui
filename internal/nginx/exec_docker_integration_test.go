//go:build integration

package nginx

import (
	"context"
	"fmt"
	"os"
	"testing"

	internaldocker "github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecShellExternalContainer(t *testing.T) {
	containerName := os.Getenv("NGINX_UI_TEST_EXTERNAL_CONTAINER")
	if containerName == "" {
		t.Skip("NGINX_UI_TEST_EXTERNAL_CONTAINER is not set")
	}

	originalContainerName := settings.NginxSettings.ContainerName
	settings.NginxSettings.ContainerName = containerName
	t.Cleanup(func() {
		settings.NginxSettings.ContainerName = originalContainerName
	})

	const markerPath = "/tmp/nginx-ui-issue-1571-command-marker"
	_ = os.Remove(markerPath)
	_, _ = internaldocker.Exec(context.Background(), []string{"rm", "-f", markerPath})
	t.Cleanup(func() {
		_ = os.Remove(markerPath)
		_, _ = internaldocker.Exec(context.Background(), []string{"rm", "-f", markerPath})
	})

	_, err := execShell("printf issue-1571 > " + markerPath)
	require.NoError(t, err)

	_, localErr := os.Stat(markerPath)
	assert.ErrorIs(t, localErr, os.ErrNotExist, "external command must not create a local marker")

	externalContent, err := internaldocker.Exec(context.Background(), []string{"cat", markerPath})
	require.NoError(t, err)
	assert.Equal(t, "issue-1571", externalContent)
}

func TestExecShellMissingExternalContainerFailsClosed(t *testing.T) {
	originalContainerName := settings.NginxSettings.ContainerName
	settings.NginxSettings.ContainerName = fmt.Sprintf("nginx-ui-issue-1571-missing-%d", os.Getpid())
	t.Cleanup(func() {
		settings.NginxSettings.ContainerName = originalContainerName
	})

	const markerPath = "/tmp/nginx-ui-issue-1571-missing-target-marker"
	_ = os.Remove(markerPath)
	t.Cleanup(func() {
		_ = os.Remove(markerPath)
	})

	_, err := execShell("touch " + markerPath)
	require.Error(t, err)

	_, localErr := os.Stat(markerPath)
	assert.ErrorIs(t, localErr, os.ErrNotExist, "failed external execution must not fall back locally")
}
