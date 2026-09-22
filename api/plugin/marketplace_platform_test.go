package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// otherPlatform is a platform the test binary does not run on.
func otherPlatform() string {
	if plugin.HostPlatform() == "linux-arm64" {
		return "linux-amd64"
	}
	return "linux-arm64"
}

// platformPackage packs a plugin whose only executable is built for platform.
func platformPackage(t *testing.T, id, pluginVersion, platform string) []byte {
	t.Helper()

	manifest := &protocol.Manifest{
		ID:         id,
		Name:       "Test " + id,
		Version:    pluginVersion,
		APIVersion: protocol.APIVersion,
		Server: &protocol.ManifestServer{
			Executables: map[string]string{platform: "bin/plugin"},
			Lifecycle:   protocol.LifecycleOnDemand,
		},
	}
	staging := filepath.Join(t.TempDir(), id)
	require.NoError(t, os.MkdirAll(filepath.Join(staging, "bin"), 0o755))
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, plugin.ManifestFileName), encoded, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(staging, "bin", "plugin"), []byte("#!/bin/sh\n# "+platform+"\n"), 0o755))

	archive := filepath.Join(t.TempDir(), plugin.PackageFileName(id, pluginVersion, platform))
	require.NoError(t, plugin.BuildPackage(staging, archive))
	body, err := os.ReadFile(archive)
	require.NoError(t, err)
	return body
}

func TestInstallFromMarketplacePicksThePlatformDownload(t *testing.T) {
	manager := setupManager(t)
	fixture := newMarketplaceFixture(t)

	const id = "com.example.native"
	host, other := plugin.HostPlatform(), otherPlatform()
	downloads := map[string]any{}
	for _, platform := range []string{host, other} {
		body := platformPackage(t, id, "1.0.0", platform)
		name := "/pkg/" + plugin.PackageFileName(id, "1.0.0", platform)
		fixture.files[name] = body
		digest := sha256.Sum256(body)
		downloads[platform] = map[string]any{
			"url":    fixture.server.URL + name,
			"sha256": hex.EncodeToString(digest[:]),
		}
	}
	fixture.document["plugins"] = append(fixture.document["plugins"].([]any), map[string]any{
		"id":    id,
		"name":  map[string]string{"en": "Native"},
		"trust": plugin.TrustOfficial,
		"releases": []any{map[string]any{
			"version":     "1.0.0",
			"api_version": protocol.APIVersion,
			"platforms":   []string{host, other},
			"downloads":   downloads,
			// The portable package is gone, a node must never fall back to it.
			"download_url": fixture.server.URL + "/pkg/missing.tar.gz",
		}},
	})

	c, recorder := newContext(http.MethodGet, "/api/plugins/marketplace/"+id, nil, gin.Params{{Key: "id", Value: id}})
	GetMarketplacePlugin(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var detail struct {
		Plugin       *plugin.CatalogEntry `json:"plugin"`
		HostPlatform string               `json:"host_platform"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &detail))
	assert.Equal(t, host, detail.HostPlatform)
	require.NotNil(t, detail.Plugin.InstallableRelease)
	assert.Len(t, detail.Plugin.InstallableRelease.Downloads, 2)

	c, recorder = newContext(http.MethodPost, "/api/plugins/marketplace/install",
		strings.NewReader(`{"id":"`+id+`","approve_permissions":true}`), nil)
	InstallFromMarketplace(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	manifest, ok := manager.Manifest(id)
	require.True(t, ok)
	assert.Equal(t, map[string]string{host: "bin/plugin"}, manifest.Server.Executables)
}

func TestInstallFromMarketplaceRefusesAMissingPlatform(t *testing.T) {
	setupManager(t)
	fixture := newMarketplaceFixture(t)

	const id = "com.example.native"
	other := otherPlatform()
	body := platformPackage(t, id, "1.0.0", other)
	name := "/pkg/" + plugin.PackageFileName(id, "1.0.0", other)
	fixture.files[name] = body
	fixture.document["plugins"] = append(fixture.document["plugins"].([]any), map[string]any{
		"id":    id,
		"trust": plugin.TrustOfficial,
		"releases": []any{map[string]any{
			"version":     "1.0.0",
			"api_version": protocol.APIVersion,
			"platforms":   []string{other},
			"downloads":   map[string]any{other: map[string]any{"url": fixture.server.URL + name}},
		}},
	})

	c, recorder := newContext(http.MethodPost, "/api/plugins/marketplace/install",
		strings.NewReader(`{"id":"`+id+`"}`), nil)
	InstallFromMarketplace(c)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55110")
}
