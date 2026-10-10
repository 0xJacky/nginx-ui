package plugin

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aead.dev/minisign"
	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useUploadStore points os.TempDir() at a fresh directory and returns the
// store path inside it.
func useUploadStore(t *testing.T) (string, string) {
	t.Helper()
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	return temp, filepath.Join(temp, uploadStoreName)
}

// newUploadContext builds a multipart request with the given fields and, when
// archive is set, the package file.
func newUploadContext(t *testing.T, target string, fields map[string]string, archive string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	if archive != "" {
		part, err := writer.CreateFormFile("file", filepath.Base(archive))
		require.NoError(t, err)
		data, err := os.ReadFile(archive)
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	c, recorder := newContext(http.MethodPost, target, &body, nil)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c, recorder
}

// inspectUpload runs the inspect endpoint and decodes its result.
func inspectUpload(t *testing.T, archive string) *plugin.InspectResult {
	t.Helper()

	c, recorder := newUploadContext(t, "/api/plugins/inspect", nil, archive)
	InspectPlugin(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var result plugin.InspectResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
	return &result
}

// installUpload runs the install endpoint with the given upload id.
func installUpload(t *testing.T, uploadID string) *httptest.ResponseRecorder {
	t.Helper()

	c, recorder := newUploadContext(t, "/api/plugins",
		map[string]string{"upload_id": uploadID, "enable": "true"}, "")
	InstallPlugin(c)
	return recorder
}

// leftoverUploadDirs lists the per request directories still on disk.
func leftoverUploadDirs(t *testing.T, temp string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(temp, "nginx-ui-plugin-upload-*"))
	require.NoError(t, err)
	return matches
}

func TestInspectPluginRefusesWhenUploadsAreDisabled(t *testing.T) {
	setupManager(t)
	temp, store := useUploadStore(t)
	previous := settings.PluginSettings.AllowUploads
	t.Cleanup(func() { settings.PluginSettings.AllowUploads = previous })
	settings.PluginSettings.AllowUploads = false

	archive := buildTestPackage(t, webappManifest("official.alpha"), nil)
	c, recorder := newUploadContext(t, "/api/plugins/inspect", nil, archive)
	InspectPlugin(c)

	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55015")
	assert.NoDirExists(t, store)
	assert.Empty(t, leftoverUploadDirs(t, temp))
}

func TestInstallPluginReusesTheInspectedUpload(t *testing.T) {
	setupManager(t)
	temp, store := useUploadStore(t)

	archive := buildTestPackage(t, webappManifest("official.alpha"), map[string]string{
		"webapp/main.js": "export default {}",
	})
	result := inspectUpload(t, archive)
	assert.Equal(t, "official.alpha", result.Manifest.ID)
	require.Regexp(t, `^[0-9a-f]{32}$`, result.UploadID)

	kept := filepath.Join(store, result.UploadID+uploadSuffix)
	assert.FileExists(t, kept)
	assert.Empty(t, leftoverUploadDirs(t, temp))

	recorder := installUpload(t, result.UploadID)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var info plugin.Info
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	assert.Equal(t, "official.alpha", info.ID)
	assert.True(t, info.Enabled)

	assert.NoFileExists(t, kept)
	assert.Empty(t, leftoverUploadDirs(t, temp))

	// The id is spent once the install took the package.
	recorder = installUpload(t, result.UploadID)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55007")
}

func TestInstallPluginDropsTheUploadWhenTheInstallFails(t *testing.T) {
	setupManager(t)
	temp, store := useUploadStore(t)

	// Inspect reports the missing dependency, the install refuses it.
	manifest := webappManifest("official.alpha")
	manifest.Requires = []protocol.ManifestRequirement{{ID: "official.absent"}}
	result := inspectUpload(t, buildTestPackage(t, manifest, nil))
	require.NotEmpty(t, result.UploadID)
	require.NotEmpty(t, result.RequiresMissing)

	recorder := installUpload(t, result.UploadID)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55011")
	assert.NoFileExists(t, filepath.Join(store, result.UploadID+uploadSuffix))
	assert.Empty(t, leftoverUploadDirs(t, temp))
}

func TestInstallPluginRefusesAnUnknownUpload(t *testing.T) {
	setupManager(t)
	useUploadStore(t)

	for _, uploadID := range []string{
		strings.Repeat("ab", 16),
		"../../etc/passwd",
		strings.Repeat("AB", 16),
	} {
		recorder := installUpload(t, uploadID)
		assert.NotEqual(t, http.StatusOK, recorder.Code, uploadID)
		assert.Contains(t, recorder.Body.String(), "55007", uploadID)
	}
}

func TestInstallPluginRefusesAnExpiredUpload(t *testing.T) {
	setupManager(t)
	_, store := useUploadStore(t)

	result := inspectUpload(t, buildTestPackage(t, webappManifest("official.alpha"), nil))
	kept := filepath.Join(store, result.UploadID+uploadSuffix)
	old := time.Now().Add(-uploadTTL - time.Minute)
	require.NoError(t, os.Chtimes(kept, old, old))

	recorder := installUpload(t, result.UploadID)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55007")
	assert.NoFileExists(t, kept)
}

func TestSweepUploadsDropsOnlyExpiredPackages(t *testing.T) {
	_, store := useUploadStore(t)
	dir, err := uploadStoreDir()
	require.NoError(t, err)
	require.Equal(t, store, dir)

	old := time.Now().Add(-uploadTTL - time.Minute)
	write := func(name string, modified time.Time) string {
		path := filepath.Join(store, name)
		require.NoError(t, os.WriteFile(path, []byte("package"), 0o600))
		require.NoError(t, os.Chtimes(path, modified, modified))
		return path
	}
	expired := write(strings.Repeat("0a", 16)+uploadSuffix, old)
	fresh := write(strings.Repeat("0b", 16)+uploadSuffix, time.Now())
	foreign := write("notes.txt", old)

	sweepUploads()
	assert.NoFileExists(t, expired)
	assert.FileExists(t, fresh)
	assert.FileExists(t, foreign)
}

func TestInstallPluginRefusesAnUnsignedUploadWithoutDeveloperMode(t *testing.T) {
	setupManager(t)
	useUploadStore(t)
	previous := settings.PluginSettings.DeveloperMode
	t.Cleanup(func() { settings.PluginSettings.DeveloperMode = previous })
	settings.PluginSettings.DeveloperMode = false

	// Inspect shows the package as unsigned, the install refuses it.
	result := inspectUpload(t, buildTestPackage(t, webappManifest("official.alpha"), nil))
	assert.Equal(t, plugin.TrustUnsigned, result.Trust)
	assert.Empty(t, result.Signer)

	recorder := installUpload(t, result.UploadID)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55023")

	// Developer mode lets the same upload in.
	settings.PluginSettings.DeveloperMode = true
	result = inspectUpload(t, buildTestPackage(t, webappManifest("official.alpha"), nil))
	recorder = installUpload(t, result.UploadID)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var info plugin.Info
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	assert.Equal(t, plugin.TrustUnsigned, info.Trust)
}

// buildSignedUploadPackage packs a webapp plugin signed with a fresh key and
// returns it together with the public key text.
func buildSignedUploadPackage(t *testing.T, id string) (string, string) {
	t.Helper()
	public, private, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encoded, err := public.MarshalText()
	require.NoError(t, err)

	archive := buildTestPackage(t, webappManifest(id), nil)
	require.NoError(t, plugin.SignPackage(archive, private))
	return archive, string(encoded)
}

// installWithAuthorKey runs the install endpoint on a direct upload.
func installWithAuthorKey(t *testing.T, archive, authorKey string) *httptest.ResponseRecorder {
	t.Helper()
	fields := map[string]string{"enable": "true"}
	if authorKey != "" {
		fields["author_public_key"] = authorKey
	}
	c, recorder := newUploadContext(t, "/api/plugins", fields, archive)
	InstallPlugin(c)
	return recorder
}

func TestInstallPluginTakesTheAuthorKeyOfAPush(t *testing.T) {
	setupManager(t)
	useUploadStore(t)
	archive, authorKey := buildSignedUploadPackage(t, "official.alpha")

	// Without the key the signer is unknown here, which is unsigned.
	recorder := installWithAuthorKey(t, archive, "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var info plugin.Info
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	assert.Equal(t, plugin.TrustUnsigned, info.Trust)

	// With it the package is community, as on the controller.
	recorder = installWithAuthorKey(t, archive, authorKey)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	assert.Equal(t, plugin.TrustCommunity, info.Trust)
	assert.NotEmpty(t, info.Signer)

	// A value that is not a public key is refused.
	recorder = installWithAuthorKey(t, archive, "not a key")
	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55107")
}
