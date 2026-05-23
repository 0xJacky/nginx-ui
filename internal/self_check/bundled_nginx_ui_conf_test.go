package self_check

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

// withFixture copies the named fixture from test_cases/bundled/ into a tempdir
// and redirects bundledNginxUIConfPath to it for the duration of the test.
// Also forces InNginxUIOfficialDocker() to true via env var.
func withFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("test_cases", "bundled", name)
	data, err := os.ReadFile(src)
	require.NoError(t, err, "fixture %s", name)

	dir := t.TempDir()
	target := filepath.Join(dir, "nginx-ui.conf")
	require.NoError(t, os.WriteFile(target, data, 0o644))

	orig := bundledNginxUIConfPath
	bundledNginxUIConfPath = target
	t.Cleanup(func() { bundledNginxUIConfPath = orig })

	// Force the docker guard on.
	t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")
	t.Setenv("NGINX_UI_IGNORE_DOCKER_SOCKET", "")
	return target
}

func TestCheckBundledNginxUIConf(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		wantOK  bool
		wantErr int32 // cosy error code; ignored if wantOK
	}{
		{"unfixed default", "unfixed-default.conf", false, 40421},
		{"fixed default", "fixed-default.conf", true, 0},
		{"customized unfixed", "customized-unfixed.conf", false, 40421},
		{"customized fixed", "customized-fixed.conf", true, 0},
		{"half-fixed (one map missing)", "maps-only-half.conf", false, 40421},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withFixture(t, tc.fixture)
			err := CheckBundledNginxUIConf()
			if tc.wantOK {
				assert.NoError(t, err)
				return
			}
			var cErr *cosy.Error
			require.True(t, errors.As(err, &cErr), "want cosy.Error, got %T", err)
			assert.Equal(t, tc.wantErr, cErr.Code)
		})
	}
}

func TestCheckBundledNginxUIConf_MissingFile(t *testing.T) {
	dir := t.TempDir()
	orig := bundledNginxUIConfPath
	bundledNginxUIConfPath = filepath.Join(dir, "does-not-exist.conf")
	t.Cleanup(func() { bundledNginxUIConfPath = orig })
	t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")
	t.Setenv("NGINX_UI_IGNORE_DOCKER_SOCKET", "")

	// Missing file is delegated to other tasks; CheckFunc returns nil.
	assert.NoError(t, CheckBundledNginxUIConf())
}

func TestCheckBundledNginxUIConf_NotInDocker(t *testing.T) {
	t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "")
	// Even with a missing path, no error when not in docker.
	bundledNginxUIConfPath = "/nonexistent/path"
	assert.NoError(t, CheckBundledNginxUIConf())
}

func TestApplyBundledConfPatch_Idempotent(t *testing.T) {
	fixed, err := os.ReadFile(filepath.Join("test_cases", "bundled", "fixed-default.conf"))
	require.NoError(t, err)
	assert.Equal(t, fixed, applyBundledConfPatch(fixed),
		"already-fixed input must be byte-equal output")
}

func TestApplyBundledConfPatch_UpgradesUnfixed(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("test_cases", "bundled", "unfixed-default.conf"))
	require.NoError(t, err)
	out := applyBundledConfPatch(in)

	assert.True(t, reMapForwardedProto.Match(out), "must inject forwarded_proto map")
	assert.True(t, reMapForwardedHost.Match(out), "must inject forwarded_host map")
	assert.True(t, reHeaderForwardedProto.Match(out), "must rewrite X-Forwarded-Proto to $forwarded_proto")
	assert.True(t, reHeaderForwardedHost.Match(out), "must rewrite X-Forwarded-Host to $forwarded_host")
}

func TestApplyBundledConfPatch_PreservesCustomization(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("test_cases", "bundled", "customized-unfixed.conf"))
	require.NoError(t, err)
	out := applyBundledConfPatch(in)

	assert.Contains(t, string(out), "client_max_body_size 256M",
		"user customization must survive")
	assert.Contains(t, string(out), "server_name  nginx-ui.example.com",
		"user customization must survive")
	assert.True(t, reHeaderForwardedProto.Match(out))
	assert.True(t, reHeaderForwardedHost.Match(out))
}

func TestApplyBundledConfPatch_HalfFixedFillsOnlyMissing(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("test_cases", "bundled", "maps-only-half.conf"))
	require.NoError(t, err)
	out := applyBundledConfPatch(in)

	// Both maps now present; should not duplicate the existing one.
	assert.Equal(t, 1, len(reMapForwardedProto.FindAll(out, -1)),
		"forwarded_proto map must appear exactly once")
	assert.Equal(t, 1, len(reMapForwardedHost.FindAll(out, -1)))
}

func TestInjectBeforeFirstServer_FallbackToPrepend(t *testing.T) {
	in := []byte("# only comments, no server block\n")
	out := injectBeforeFirstServer(in, "INJECTED\n")
	assert.Equal(t, "INJECTED\n# only comments, no server block\n", string(out))
}
