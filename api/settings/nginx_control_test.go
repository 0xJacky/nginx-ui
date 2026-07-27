package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hostsetup "github.com/0xJacky/Nginx-UI/internal/host/setup"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNginxControlSettings(t *testing.T) {
	tests := []struct {
		name    string
		payload nginxControlSettingsPayload
		wantErr string
	}{
		{name: "local", payload: nginxControlSettingsPayload{Mode: appsettings.ControlModeLocal}},
		{
			name: "external container",
			payload: nginxControlSettingsPayload{
				Mode:          appsettings.ControlModeExternalContainer,
				ContainerName: "nginx-1",
			},
		},
		{
			name:    "external container requires name",
			payload: nginxControlSettingsPayload{Mode: appsettings.ControlModeExternalContainer},
			wantErr: "container name is required",
		},
		{
			name: "external container rejects invalid name",
			payload: nginxControlSettingsPayload{
				Mode:          appsettings.ControlModeExternalContainer,
				ContainerName: "nginx container",
			},
			wantErr: "invalid characters",
		},
		{
			name: "ssh",
			payload: nginxControlSettingsPayload{
				Mode:               appsettings.ControlModeHostViaSSH,
				HostAddress:        "host.docker.internal:22",
				HostUser:           "nginxui",
				HostAuthMethod:     "key",
				HostKeySource:      appsettings.HostKeySourceGenerated,
				HostPrivateKeyPath: "/etc/nginx-ui/host_key",
				HostKnownHostsPath: "/etc/nginx-ui/known_hosts",
				HostServiceManager: appsettings.HostServiceManagerSystemd,
			},
		},
		{
			name:    "ssh requires connection",
			payload: nginxControlSettingsPayload{Mode: appsettings.ControlModeHostViaSSH},
			wantErr: "host address and user are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNginxControlSettings(tt.payload)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNormalizeNginxControlSettingsInfersLegacyKeySource(t *testing.T) {
	generated := normalizeNginxControlSettings(nginxControlSettingsPayload{
		HostPrivateKeyPath: appsettings.DefaultHostPrivateKeyPath,
	})
	assert.Equal(t, appsettings.HostKeySourceGenerated, generated.HostKeySource)

	existing := normalizeNginxControlSettings(nginxControlSettingsPayload{
		HostPrivateKeyPath: "/run/secrets/nginx_ui_ssh",
	})
	assert.Equal(t, appsettings.HostKeySourceExisting, existing.HostKeySource)
}

func TestApplyNginxControlSettings(t *testing.T) {
	t.Run("local clears active selectors and preserves SSH details", func(t *testing.T) {
		target := &appsettings.Nginx{
			ContainerName: "nginx",
			HostMode:      appsettings.HostModeSSH,
			HostAddress:   "host.example:22",
		}

		applyNginxControlSettings(target, nginxControlSettingsPayload{Mode: appsettings.ControlModeLocal})

		assert.Empty(t, target.ContainerName)
		assert.Empty(t, target.HostMode)
		assert.Equal(t, "host.example:22", target.HostAddress)
	})

	t.Run("external container selects container and disables SSH", func(t *testing.T) {
		target := &appsettings.Nginx{HostMode: appsettings.HostModeSSH}

		applyNginxControlSettings(target, nginxControlSettingsPayload{
			Mode:          appsettings.ControlModeExternalContainer,
			ContainerName: "nginx-1",
		})

		assert.Equal(t, "nginx-1", target.ContainerName)
		assert.Empty(t, target.HostMode)
	})

	t.Run("ssh applies protected host settings", func(t *testing.T) {
		target := &appsettings.Nginx{ContainerName: "nginx"}
		payload := nginxControlSettingsPayload{
			Mode:               appsettings.ControlModeHostViaSSH,
			HostAddress:        "host.docker.internal:22",
			HostUser:           "nginxui",
			HostAuthMethod:     "key",
			HostKeySource:      appsettings.HostKeySourceExisting,
			HostPrivateKeyPath: "/etc/nginx-ui/host_key",
			HostKnownHostsPath: "/etc/nginx-ui/known_hosts",
			HostServiceManager: appsettings.HostServiceManagerLaunchd,
		}

		applyNginxControlSettings(target, payload)

		assert.Empty(t, target.ContainerName)
		assert.Equal(t, appsettings.HostModeSSH, target.HostMode)
		assert.Equal(t, payload.HostAddress, target.HostAddress)
		assert.Equal(t, payload.HostServiceManager, target.HostServiceManager)
		assert.Equal(t, appsettings.HostKeySourceExisting, target.HostKeySource)
	})
}

func TestSaveNginxControlSettingsRequiresVerifiedTwoFactor(t *testing.T) {
	t.Run("rejects user without verified session", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/control", nil)

		SaveNginxControlSettings(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects node secret authentication", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("Secret", "node-secret")
		c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/control", nil)

		SaveNginxControlSettings(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestSaveNginxControlSettingsWithVerifiedTwoFactor(t *testing.T) {
	originalUpdate := updateNginxControlSettings
	originalNginx := *appsettings.NginxSettings
	t.Cleanup(func() {
		updateNginxControlSettings = originalUpdate
		*appsettings.NginxSettings = originalNginx
	})

	var saved nginxControlSettingsPayload
	updateNginxControlSettings = func(payload nginxControlSettingsPayload) error {
		saved = payload
		applyNginxControlSettings(appsettings.NginxSettings, payload)
		return nil
	}

	body, err := json.Marshal(nginxControlSettingsPayload{
		Mode:          appsettings.ControlModeExternalContainer,
		ContainerName: "nginx-1",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.SecureSessionVerifiedKey, true)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/control", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveNginxControlSettings(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nginx-1", saved.ContainerName)
	assert.Equal(t, appsettings.ControlModeExternalContainer, appsettings.NginxSettings.ControlMode())
}

func TestSaveNginxPrivateKeyRequiresVerifiedTwoFactor(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/private-key",
		bytes.NewBufferString(`{"private_key":"secret"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveNginxPrivateKey(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSaveNginxPrivateKeyReturnsOnlyDerivedPublicKey(t *testing.T) {
	originalSave := saveManagedHostPrivateKey
	t.Cleanup(func() {
		saveManagedHostPrivateKey = originalSave
	})
	saveManagedHostPrivateKey = func(path string, raw []byte) (string, error) {
		assert.Equal(t, appsettings.DefaultHostPrivateKeyPath, path)
		assert.Equal(t, "submitted-private-key", string(raw))
		return "ssh-ed25519 AAAA nginx-ui@provided", nil
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.SecureSessionVerifiedKey, true)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/private-key",
		bytes.NewBufferString(`{"private_key":"submitted-private-key"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveNginxPrivateKey(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "submitted-private-key")
	assert.Contains(t, w.Body.String(), "ssh-ed25519 AAAA")
}

func TestSaveNginxPrivateKeyRejectsInvalidKey(t *testing.T) {
	originalSave := saveManagedHostPrivateKey
	t.Cleanup(func() {
		saveManagedHostPrivateKey = originalSave
	})
	saveManagedHostPrivateKey = func(string, []byte) (string, error) {
		return "", errors.Join(hostsetup.ErrInvalidPrivateKey, errors.New("parse failed"))
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.SecureSessionVerifiedKey, true)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/private-key",
		bytes.NewBufferString(`{"private_key":"invalid"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveNginxPrivateKey(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "parse failed")
}

func TestSaveNginxPrivateKeyRejectsOversizedInput(t *testing.T) {
	originalSave := saveManagedHostPrivateKey
	t.Cleanup(func() {
		saveManagedHostPrivateKey = originalSave
	})
	saveManagedHostPrivateKey = func(string, []byte) (string, error) {
		t.Fatal("oversized private key reached storage")
		return "", nil
	}

	body, err := json.Marshal(nginxPrivateKeyPayload{
		PrivateKey: strings.Repeat("x", maxPastedPrivateKeySize+1),
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.SecureSessionVerifiedKey, true)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/private-key", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveNginxPrivateKey(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApplyNginxControlSettingsClearsHostPathsWhenLeavingSSH(t *testing.T) {
	target := &appsettings.Nginx{
		HostMode:      appsettings.HostModeSSH,
		HostAddress:   "host.docker.internal:22",
		SbinPath:      "/opt/homebrew/opt/nginx/bin/nginx",
		PIDPath:       "/opt/homebrew/var/run/nginx.pid",
		ConfigDir:     "/opt/homebrew/etc/nginx",
		ConfigPath:    "/opt/homebrew/etc/nginx/nginx.conf",
		AccessLogPath: "/opt/homebrew/var/log/nginx/access.log",
		ErrorLogPath:  "/opt/homebrew/var/log/nginx/error.log",
		HostConfigDir: "/opt/homebrew/etc/nginx",
		HostLogDir:    "/opt/homebrew/var/log/nginx",
	}

	applyNginxControlSettings(target, nginxControlSettingsPayload{Mode: appsettings.ControlModeLocal})

	assert.Empty(t, target.HostMode)
	assert.Empty(t, target.SbinPath)
	assert.Empty(t, target.PIDPath)
	assert.Empty(t, target.ConfigDir)
	assert.Empty(t, target.ConfigPath)
	assert.Empty(t, target.AccessLogPath)
	assert.Empty(t, target.ErrorLogPath)
	assert.Empty(t, target.HostConfigDir)
	assert.Empty(t, target.HostLogDir)
}

func TestApplyNginxControlSettingsKeepsLocalPathsWhenAlreadyLocal(t *testing.T) {
	target := &appsettings.Nginx{SbinPath: "/usr/local/sbin/nginx"}

	applyNginxControlSettings(target, nginxControlSettingsPayload{Mode: appsettings.ControlModeLocal})

	assert.Equal(t, "/usr/local/sbin/nginx", target.SbinPath)
}
