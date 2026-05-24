package self_check

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	orig := bundledNginxUIConfPath
	bundledNginxUIConfPath = "/nonexistent/path"
	t.Cleanup(func() { bundledNginxUIConfPath = orig })
	assert.NoError(t, CheckBundledNginxUIConf())
}

func TestCheckBundledNginxUIConf_SkipsWhenBundledNginxDisabled(t *testing.T) {
	t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")
	t.Setenv("NGINX_UI_DISABLE_BUNDLED_NGINX", "true")

	dir := t.TempDir()
	target := filepath.Join(dir, "nginx-ui.conf")
	src := filepath.Join("test_cases", "bundled", "unfixed-default.conf")
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, data, 0o644))

	orig := bundledNginxUIConfPath
	bundledNginxUIConfPath = target
	t.Cleanup(func() { bundledNginxUIConfPath = orig })

	assert.NoError(t, CheckBundledNginxUIConf())
}

func TestCheckBundledNginxUIConf_RunsEvenWithDockerSocketIgnored(t *testing.T) {
	// IGNORE_DOCKER_SOCKET should NOT suppress this check — it's only meant
	// to opt out of the docker-socket feature, not all docker-only checks.
	t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")
	t.Setenv("NGINX_UI_IGNORE_DOCKER_SOCKET", "true")

	dir := t.TempDir()
	target := filepath.Join(dir, "nginx-ui.conf")
	src := filepath.Join("test_cases", "bundled", "unfixed-default.conf")
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, data, 0o644))

	orig := bundledNginxUIConfPath
	bundledNginxUIConfPath = target
	t.Cleanup(func() { bundledNginxUIConfPath = orig })

	err = CheckBundledNginxUIConf()
	var cErr *cosy.Error
	require.True(t, errors.As(err, &cErr))
	assert.Equal(t, int32(40421), cErr.Code)
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

func TestPatchOnDiskWithBackup_RewritesAndBacksUp(t *testing.T) {
	target := withFixture(t, "customized-unfixed.conf")
	orig, _ := os.ReadFile(target)

	bak := target + ".bak.test"
	require.NoError(t, os.WriteFile(bak, orig, 0o644))

	require.NoError(t, patchOnDiskWithBackup(orig, bak))

	got, _ := os.ReadFile(target)
	assert.True(t, reHeaderForwardedProto.Match(got), "target must be patched")
	assert.True(t, reHeaderForwardedHost.Match(got), "target must be patched")
	assert.Contains(t, string(got), "client_max_body_size 256M",
		"customization must survive")

	bakData, _ := os.ReadFile(bak)
	assert.Equal(t, orig, bakData, "backup must contain pre-patch bytes")
}

func TestPatchOnDiskWithBackup_DoesNotMutateTargetOnWriteError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only test: relies on chmod 0o555 to make a directory read-only")
	}

	target := withFixture(t, "customized-unfixed.conf")
	orig, _ := os.ReadFile(target)
	bak := target + ".bak.test"
	require.NoError(t, os.WriteFile(bak, orig, 0o644))

	// Make the parent dir read-only to force os.WriteFile(.tmp) to fail.
	dir := filepath.Dir(target)
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := patchOnDiskWithBackup(orig, bak)
	require.Error(t, err)

	// Target must be untouched (restore would also fail under the same chmod,
	// but since the .tmp write failed first the target file was never modified).
	got, _ := os.ReadFile(target)
	assert.Equal(t, orig, got, "failed patch must leave target byte-identical to its pre-patch state")
}
