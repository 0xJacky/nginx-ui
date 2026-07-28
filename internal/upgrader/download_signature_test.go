package upgrader

import (
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadLatestReleaseVerifiesSignedArchivesThroughHTTPProxy(t *testing.T) {
	trustedPublicKey, trustedPrivateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	unknownPublicKey, unknownPrivateKey, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	trustedPublicKeyText, err := trustedPublicKey.MarshalText()
	require.NoError(t, err)

	originalArchive := []byte("authentic release archive")
	tamperedArchive := []byte("attacker-controlled archive")
	trustedSignature := signArchiveContent(t, trustedPrivateKey, originalArchive)
	unknownSignature := signArchiveContent(t, unknownPrivateKey, tamperedArchive)

	testCases := []struct {
		name      string
		archive   []byte
		signature []byte
		digest    string
		wantErr   error
	}{
		{
			name:      "accepts authentic archive",
			archive:   originalArchive,
			signature: trustedSignature,
			digest:    archiveDigest(originalArchive),
		},
		{
			name:      "rejects archive tampering despite matching attacker digest",
			archive:   tamperedArchive,
			signature: trustedSignature,
			digest:    archiveDigest(tamperedArchive),
			wantErr:   ErrSignatureInvalid,
		},
		{
			name:      "rejects attacker signature from unknown key",
			archive:   tamperedArchive,
			signature: unknownSignature,
			digest:    archiveDigest(tamperedArchive),
			wantErr:   ErrSignatureKeyUnknown,
		},
		{
			name:      "retains digest as corruption check",
			archive:   originalArchive,
			signature: trustedSignature,
			digest:    archiveDigest(tamperedArchive),
			wantErr:   ErrDigestMismatch,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setTrustedMinisignKeysForTest(t, string(trustedPublicKeyText))
			originalProxy := settings.HTTPSettings.GithubProxy
			t.Cleanup(func() {
				settings.HTTPSettings.GithubProxy = originalProxy
			})

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch {
				case strings.HasSuffix(request.URL.Path, ".tar.gz.minisig"):
					_, _ = writer.Write(testCase.signature)
				case strings.HasSuffix(request.URL.Path, ".tar.gz.digest"):
					_, _ = writer.Write([]byte(testCase.digest))
				case strings.HasSuffix(request.URL.Path, ".tar.gz"):
					writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(testCase.archive)))
					_, _ = writer.Write(testCase.archive)
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()
			settings.HTTPSettings.GithubProxy = server.URL

			downloadDir := t.TempDir()
			upgrader := newSignedDownloadTestUpgrader(downloadDir)
			archivePath, downloadErr := upgrader.DownloadLatestRelease(nil)

			if testCase.wantErr != nil {
				assert.ErrorIs(t, downloadErr, testCase.wantErr)
				if archivePath != "" {
					_, statErr := os.Stat(archivePath)
					assert.ErrorIs(t, statErr, os.ErrNotExist)
					_, statErr = os.Stat(archivePath + ".minisig")
					assert.ErrorIs(t, statErr, os.ErrNotExist)
				}
				return
			}

			require.NoError(t, downloadErr)
			t.Cleanup(func() {
				_ = os.Remove(archivePath)
				_ = os.Remove(archivePath + ".minisig")
			})
			downloadedArchive, err := os.ReadFile(archivePath)
			require.NoError(t, err)
			assert.Equal(t, originalArchive, downloadedArchive)
			_, err = verifyAdjacentArchiveSignature(archivePath)
			require.NoError(t, err)
		})
	}

	assert.NotEqual(t, trustedPublicKey.ID(), unknownPublicKey.ID())
}

func TestDownloadLatestReleaseFailsBeforeDownloadWithoutSignatureAsset(t *testing.T) {
	downloadDir := t.TempDir()
	upgrader := newSignedDownloadTestUpgrader(downloadDir)
	upgrader.Release.Assets = []version.TReleaseAsset{
		{
			Name:               "nginx-ui-linux-64.tar.gz",
			BrowserDownloadUrl: "https://releases.example/nginx-ui-linux-64.tar.gz",
		},
		{
			Name:               "nginx-ui-linux-64.tar.gz.digest",
			BrowserDownloadUrl: "https://releases.example/nginx-ui-linux-64.tar.gz.digest",
		},
	}

	archivePath, err := upgrader.DownloadLatestRelease(nil)
	assert.ErrorIs(t, err, ErrSignatureEmpty)
	assert.Empty(t, archivePath)
}

func newSignedDownloadTestUpgrader(downloadDir string) *Upgrader {
	const assetOrigin = "https://releases.example/"
	return &Upgrader{
		Channel: string(version.ReleaseTypeStable),
		Release: version.TRelease{
			Assets: []version.TReleaseAsset{
				{
					Name:               "nginx-ui-linux-64.tar.gz",
					BrowserDownloadUrl: assetOrigin + "nginx-ui-linux-64.tar.gz",
				},
				{
					Name:               "nginx-ui-linux-64.tar.gz.minisig",
					BrowserDownloadUrl: assetOrigin + "nginx-ui-linux-64.tar.gz.minisig",
				},
				{
					Name:               "nginx-ui-linux-64.tar.gz.digest",
					BrowserDownloadUrl: assetOrigin + "nginx-ui-linux-64.tar.gz.digest",
				},
			},
		},
		RuntimeInfo: version.RuntimeInfo{
			OS:     "linux",
			Arch:   "amd64",
			ExPath: filepath.Join(downloadDir, "nginx-ui"),
		},
	}
}

func archiveDigest(content []byte) string {
	digest := sha512.Sum512(content)
	return fmt.Sprintf("%x", digest)
}
