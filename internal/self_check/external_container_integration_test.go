//go:build integration

package self_check

import (
	"errors"
	"os"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

func TestExternalContainerConfigSharedDocker(t *testing.T) {
	containerName := os.Getenv("NGINX_UI_TEST_EXTERNAL_CONTAINER")
	sharedConfigDir := os.Getenv("NGINX_UI_TEST_SHARED_CONFIG_DIR")
	if containerName == "" || sharedConfigDir == "" {
		t.Skip("external container integration environment is not configured")
	}

	originalNginx := *appsettings.NginxSettings
	t.Cleanup(func() {
		*appsettings.NginxSettings = originalNginx
	})
	appsettings.NginxSettings.ContainerName = containerName
	appsettings.NginxSettings.ConfigDir = sharedConfigDir

	require.NoError(t, CheckExternalContainerConfigShared())
}

func TestExternalContainerConfigUnsharedDocker(t *testing.T) {
	containerName := os.Getenv("NGINX_UI_TEST_EXTERNAL_CONTAINER")
	if containerName == "" {
		t.Skip("NGINX_UI_TEST_EXTERNAL_CONTAINER is not set")
	}

	originalNginx := *appsettings.NginxSettings
	t.Cleanup(func() {
		*appsettings.NginxSettings = originalNginx
	})
	appsettings.NginxSettings.ContainerName = containerName
	appsettings.NginxSettings.ConfigDir = t.TempDir()

	err := CheckExternalContainerConfigShared()
	require.Error(t, err)
	var cosyErr *cosy.Error
	require.True(t, errors.As(err, &cosyErr))
	assert.Equal(t, int32(40422), cosyErr.Code)
}
