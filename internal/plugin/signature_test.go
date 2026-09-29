package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readPackageEntries lists the entries of a package archive.
func readPackageEntries(t *testing.T, body []byte) []tarEntry {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	tr := tar.NewReader(gz)

	var entries []tarEntry
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return entries
		}
		require.NoError(t, err)
		content, err := io.ReadAll(tr)
		require.NoError(t, err)
		entries = append(entries, tarEntry{header: *header, body: string(content)})
	}
}

// rewritePackageBytes rebuilds an archive after mutate had a look at every
// entry. An entry whose name mutate clears is dropped.
func rewritePackageBytes(t *testing.T, body []byte, mutate func(entry *tarEntry)) []byte {
	t.Helper()
	entries := readPackageEntries(t, body)
	kept := make([]tarEntry, 0, len(entries)+1)
	for i := range entries {
		entry := entries[i]
		mutate(&entry)
		if entry.header.Name == "" {
			continue
		}
		entry.header.Size = int64(len(entry.body))
		kept = append(kept, entry)
	}
	rewritten, err := os.ReadFile(writeArchive(t, kept))
	require.NoError(t, err)
	return rewritten
}

// rewritePackage is rewritePackageBytes on a file, written to a new file.
func rewritePackage(t *testing.T, archive string, mutate func(entry *tarEntry)) string {
	t.Helper()
	body, err := os.ReadFile(archive)
	require.NoError(t, err)
	target := filepath.Join(t.TempDir(), filepath.Base(archive))
	require.NoError(t, os.WriteFile(target, rewritePackageBytes(t, body, mutate), 0o644))
	return target
}

// signedWebappPackage builds a webapp only package signed by signer.
func signedWebappPackage(t *testing.T, id string, signer *minisign.PrivateKey) string {
	t.Helper()
	return buildSignedTestPackage(t, marketplaceManifest(id, "1.0.0"),
		map[string]string{"webapp/main.js": "export default {}", "README.md": "# readme\n"}, signer)
}

// extractedTrust unpacks a package and derives its trust with keyring.
func extractedTrust(t *testing.T, archive, authorKey string, keyring *partnerKeyring) (packageTrust, error) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "payload")
	_, err := ExtractPackage(archive, dir)
	require.NoError(t, err)
	return verifyPackageSignature(dir, authorKey, keyring)
}

func TestTrustRank(t *testing.T) {
	ordered := []string{TrustUnsigned, TrustCommunity, TrustVerified, TrustOfficial}
	for i := 1; i < len(ordered); i++ {
		assert.Less(t, trustRank(ordered[i-1]), trustRank(ordered[i]))
	}
	assert.Equal(t, trustRank(TrustUnsigned), trustRank(""))
	assert.Equal(t, trustRank(TrustUnsigned), trustRank("partner"))
}

func TestVerifyPackageSignatureDerivesTheTrust(t *testing.T) {
	useMarketplace(t)
	official := useReleaseKey(t)
	partner, keyring := usePartnerKey(t)
	userPublic, user := newSigningKey(t)
	trustKey(t, userPublic)
	authorPublic, author := newSigningKey(t)
	_, stranger := newSigningKey(t)

	for name, testCase := range map[string]struct {
		signer    *minisign.PrivateKey
		authorKey string
		want      string
		// wantKey is the key recorded for a community package.
		wantKey string
		// wantPartner is the partner name recorded for a verified package.
		wantPartner string
	}{
		"release key": {signer: official, want: TrustOfficial},
		"partner key": {signer: partner, want: TrustVerified, wantPartner: "example"},
		"user key":    {signer: &user, want: TrustCommunity, wantKey: publisherKey(t, userPublic)},
		"catalog author key": {signer: &author, authorKey: encodeKey(t, authorPublic), want: TrustCommunity,
			wantKey: encodeKey(t, authorPublic)},
		"author key unknown":  {signer: &author, want: TrustUnsigned},
		"unknown signer":      {signer: &stranger, want: TrustUnsigned},
		"no signature at all": {want: TrustUnsigned},
		"broken author key":   {signer: official, authorKey: "garbage", want: TrustOfficial},
	} {
		t.Run(name, func(t *testing.T) {
			trust, err := extractedTrust(t, signedWebappPackage(t, "com.example.alpha", testCase.signer), testCase.authorKey, keyring)
			require.NoError(t, err)
			assert.Equal(t, testCase.want, trust.Trust)
			assert.Equal(t, testCase.wantKey, trust.AuthorKey)
			assert.Equal(t, testCase.wantPartner, trust.Partner)
			if testCase.want == TrustUnsigned {
				assert.Empty(t, trust.Signer)
			} else {
				assert.Equal(t, keyID(testCase.signer), trust.Signer)
			}
		})
	}
}

func TestVerifyPackageSignatureRefusesATamperedPackage(t *testing.T) {
	useMarketplace(t)
	signer := useReleaseKey(t)
	archive := signedWebappPackage(t, "com.example.alpha", signer)

	for name, mutate := range map[string]func(entry *tarEntry){
		"changed file": func(entry *tarEntry) {
			if entry.header.Name == "webapp/main.js" {
				entry.body = "alert(1)"
			}
		},
		"missing file": func(entry *tarEntry) {
			if entry.header.Name == "README.md" {
				entry.header.Name = ""
			}
		},
		"unlisted file": func(entry *tarEntry) {
			if entry.header.Name == "README.md" {
				entry.header.Name = "webapp/extra.js"
			}
		},
		"changed sums": func(entry *tarEntry) {
			if entry.header.Name == SumsFileName {
				entry.body = strings.Replace(entry.body, "README.md", "README.MD", 1)
			}
		},
		"broken signature": func(entry *tarEntry) {
			if entry.header.Name == SumsSignatureFileName {
				entry.body = "untrusted comment: nothing\n"
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := extractedTrust(t, rewritePackage(t, archive, mutate), "", nil)
			assertPluginError(t, err, ErrSignatureInvalid)
		})
	}

	// Without the signature file the sums prove nothing, the package is
	// unsigned rather than invalid.
	stripped := rewritePackage(t, archive, func(entry *tarEntry) {
		if entry.header.Name == SumsSignatureFileName {
			entry.header.Name = ""
		}
	})
	trust, err := extractedTrust(t, stripped, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, trust.Trust)
}

func TestInstallAppliesTheTrustPolicy(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)
	settings.PluginSettings.DeveloperMode = false
	ctx := context.Background()

	official := useReleaseKey(t)
	userPublic, user := newSigningKey(t)
	trustKey(t, userPublic)

	// Unsigned is refused before anything is moved.
	_, err := manager.Install(ctx, signedWebappPackage(t, "com.example.unsigned", nil), InstallOptions{})
	assert.ErrorIs(t, err, ErrUnsignedPackage)
	assert.NoDirExists(t, filepath.Join(manager.Dir(), "com.example.unsigned"))

	// Community needs AllowCommunityPlugins.
	community := signedWebappPackage(t, "com.example.community", &user)
	settings.PluginSettings.AllowCommunityPlugins = false
	_, err = manager.Install(ctx, community, InstallOptions{})
	assert.ErrorIs(t, err, ErrCommunityNotAllowed)
	settings.PluginSettings.AllowCommunityPlugins = true

	// MinTrust refuses anything below it.
	_, err = manager.Install(ctx, community, InstallOptions{MinTrust: TrustVerified})
	assertPluginError(t, err, ErrTrustDowngrade)
	assert.Contains(t, err.Error(), TrustCommunity)
	assert.Contains(t, err.Error(), TrustVerified)

	info, err := manager.Install(ctx, community, InstallOptions{MinTrust: TrustCommunity})
	require.NoError(t, err)
	assert.Equal(t, TrustCommunity, info.Trust)
	// The key that verified it is kept for a cluster push.
	row, ok := manager.syncRow("com.example.community")
	require.True(t, ok)
	assert.Equal(t, publisherKey(t, userPublic), row.AuthorPublicKey)

	// A tampered package is refused in developer mode too.
	settings.PluginSettings.DeveloperMode = true
	tampered := rewritePackage(t, signedWebappPackage(t, "com.example.tampered", official), func(entry *tarEntry) {
		if entry.header.Name == "webapp/main.js" {
			entry.body = "alert(1)"
		}
	})
	_, err = manager.Install(ctx, tampered, InstallOptions{})
	assertPluginError(t, err, ErrSignatureInvalid)
	assert.NoDirExists(t, filepath.Join(manager.Dir(), "com.example.tampered"))

	// An upgrade records the trust of the new package.
	upgraded := buildSignedTestPackage(t, marketplaceManifest("com.example.community", "1.1.0"),
		map[string]string{"webapp/main.js": "export default {}"}, official)
	info, err = manager.Install(ctx, upgraded, InstallOptions{})
	require.NoError(t, err)
	assert.Equal(t, TrustOfficial, info.Trust)
	assert.Equal(t, keyID(official), info.Signer)
	row, ok = manager.syncRow("com.example.community")
	require.True(t, ok)
	assert.Empty(t, row.AuthorPublicKey)
}

func TestParseSumsFollowsPKG19(t *testing.T) {
	digest := strings.Repeat("ab", 32)
	line := func(name string) string { return digest + "  " + name + "\n" }

	listed, err := parseSums([]byte(line("README.md") + line("plugin.json") + line("webapp/main.js")))
	require.NoError(t, err)
	assert.Len(t, listed, 3)

	for name, sums := range map[string]string{
		"unsorted":          line("plugin.json") + line("README.md"),
		"repeated":          line("README.md") + line("README.md"),
		"no final newline":  strings.TrimSuffix(line("README.md"), "\n"),
		"empty line":        line("README.md") + "\n" + line("plugin.json"),
		"carriage return":   digest + "  README.md\r\n",
		"upper case digest": strings.ToUpper(digest) + "  README.md\n",
		"one space":         digest + " README.md\n",
		"unsafe path":       line("../README.md"),
	} {
		_, err = parseSums([]byte(sums))
		assert.Error(t, err, name)
	}
}

func TestInspectReportsTheTrust(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)
	settings.PluginSettings.DeveloperMode = false
	official := useReleaseKey(t)
	authorPublic, author := newSigningKey(t)

	result, err := manager.Inspect(signedWebappPackage(t, "com.example.alpha", official))
	require.NoError(t, err)
	assert.Equal(t, TrustOfficial, result.Trust)
	assert.Equal(t, keyID(official), result.Signer)

	// An upload has no catalog entry, so an author key alone proves nothing.
	result, err = manager.Inspect(signedWebappPackage(t, "com.example.beta", &author))
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, result.Trust)
	assert.Empty(t, result.Signer)

	// Unless the node trusts that key itself.
	trustKey(t, authorPublic)
	result, err = manager.Inspect(signedWebappPackage(t, "com.example.beta", &author))
	require.NoError(t, err)
	assert.Equal(t, TrustCommunity, result.Trust)

	tampered := rewritePackage(t, signedWebappPackage(t, "com.example.gamma", official), func(entry *tarEntry) {
		if entry.header.Name == "README.md" {
			entry.body = "changed"
		}
	})
	_, err = manager.Inspect(tampered)
	assertPluginError(t, err, ErrSignatureInvalid)
}
