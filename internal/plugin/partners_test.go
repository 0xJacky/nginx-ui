package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"
	"time"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useNow freezes the clock the expiry checks use.
func useNow(t *testing.T, moment time.Time) {
	t.Helper()
	previous := now
	now = func() time.Time { return moment }
	t.Cleanup(func() { now = previous })
}

// certify issues a certificate for partner with release and returns the two
// files it consists of.
func certify(t *testing.T, partner minisign.PublicKey, release *minisign.PrivateKey, name, expires string) map[string]string {
	t.Helper()
	partnerFile, signature, err := NewPartnerCertificate([]byte(encodeKey(t, partner)), name, expires, *release)
	require.NoError(t, err)
	return map[string]string{PartnerFileName: string(partnerFile), PartnerSignatureFileName: string(signature)}
}

// certifiedPackage builds a webapp package that carries files next to the
// usual ones, signed by signer.
func certifiedPackage(t *testing.T, id string, files map[string]string, signer *minisign.PrivateKey) string {
	t.Helper()
	all := map[string]string{"webapp/main.js": "export default {}"}
	maps.Copy(all, files)
	return buildSignedTestPackage(t, marketplaceManifest(id, "1.0.0"), all, signer)
}

// useOfficialSource makes the test catalog the official one, whose keyring
// the host fetches.
func useOfficialSource(t *testing.T, server *catalogServer) {
	t.Helper()
	previous := officialSource
	officialSource = server.catalogURL()
	t.Cleanup(func() { officialSource = previous })
}

// keyringDocument lists partners by name and revokes key ids.
func keyringDocument(t *testing.T, updatedAt string, partners map[string]minisign.PublicKey, revoked ...string) partnersDocument {
	t.Helper()
	document := partnersDocument{SchemaVersion: partnersSchemaVersion, UpdatedAt: updatedAt, Revoked: revoked}
	for name, key := range partners {
		document.Partners = append(document.Partners, partnerEntry{Name: name, PublicKey: encodeKey(t, key)})
	}
	return document
}

// servePartners publishes a keyring signed by signer next to the catalog and
// returns the bytes served.
func (cs *catalogServer) servePartners(t *testing.T, document partnersDocument, signer *minisign.PrivateKey) []byte {
	t.Helper()
	raw, err := json.Marshal(document)
	require.NoError(t, err)
	signature, err := signSums(raw, *signer)
	require.NoError(t, err)
	cs.serve("/"+PartnersFileName, raw)
	cs.serve("/"+partnersSignatureName, signature)
	return raw
}

// inspectTrust inspects a package with a manager and returns trust, signer
// and partner.
func inspectTrust(t *testing.T, m *Manager, archive string) (string, string, string) {
	t.Helper()
	result, err := m.Inspect(archive)
	require.NoError(t, err)
	return result.Trust, result.Signer, result.Partner
}

func TestPartnerCertificateMakesAPackageVerified(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)
	settings.PluginSettings.DeveloperMode = false
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	archive := certifiedPackage(t, "com.example.alpha", certify(t, partnerPublic, release, "acme", "2099-12-31"), &partner)

	trust, err := extractedTrust(t, archive, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustVerified, trust.Trust)
	assert.Equal(t, keyID(&partner), trust.Signer)
	assert.Equal(t, "acme", trust.Partner)

	gotTrust, signer, partnerName := inspectTrust(t, manager, archive)
	assert.Equal(t, TrustVerified, gotTrust)
	assert.Equal(t, keyID(&partner), signer)
	assert.Equal(t, "acme", partnerName)

	// The install records the partner with the row.
	info, err := manager.Install(context.Background(), archive, InstallOptions{})
	require.NoError(t, err)
	assert.Equal(t, TrustVerified, info.Trust)
	assert.Equal(t, "acme", info.Partner)
	row, err := query.Plugin.WithContext(context.Background()).Where(query.Plugin.PluginID.Eq("com.example.alpha")).First()
	require.NoError(t, err)
	assert.Equal(t, "acme", row.Partner)

	// A package the certificate key did not sign gains nothing from it.
	_, stranger := newSigningKey(t)
	other := certifiedPackage(t, "com.example.beta", certify(t, partnerPublic, release, "acme", "2099-12-31"), &stranger)
	trust, err = extractedTrust(t, other, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, trust.Trust)
}

func TestPartnerCertificateIsValidThroughItsExpiryDay(t *testing.T) {
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	useNow(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	archive := certifiedPackage(t, "com.example.alpha", certify(t, partnerPublic, release, "acme", "2026-01-31"), &partner)

	useNow(t, time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC))
	trust, err := extractedTrust(t, archive, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustVerified, trust.Trust)

	// The next day the certificate is ignored, which leaves the package
	// unsigned rather than invalid.
	useNow(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	trust, err = extractedTrust(t, archive, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, trust.Trust)
	assert.Empty(t, trust.Signer)
	assert.Empty(t, trust.Partner)
}

func TestPartnerRevocationWinsOverCertificateAndKeyring(t *testing.T) {
	useMarketplace(t)
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	archive := certifiedPackage(t, "com.example.alpha", certify(t, partnerPublic, release, "acme", "2099-12-31"), &partner)

	// The keyring lists the key and revokes it at the same time.
	keyring := partnerKeyringOf(t, partnerPublic)
	keyring.revoked[keyID(&partner)] = struct{}{}
	assert.Empty(t, keyring.partnerKeys())

	trust, err := extractedTrust(t, archive, "", keyring)
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, trust.Trust)

	// A revoked key the node trusts itself is still community.
	trustKey(t, partnerPublic)
	trust, err = extractedTrust(t, archive, "", keyring)
	require.NoError(t, err)
	assert.Equal(t, TrustCommunity, trust.Trust)
	assert.Empty(t, trust.Partner)
}

func TestPartnerCertificateIsIgnoredWhenItDoesNotHoldUp(t *testing.T) {
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	_, stranger := newSigningKey(t)
	certificate := certify(t, partnerPublic, release, "acme", "2099-12-31")

	withSignature := func(body []byte) map[string]string {
		return map[string]string{PartnerFileName: certificate[PartnerFileName], PartnerSignatureFileName: string(body)}
	}
	parsed := func() minisign.Signature {
		var signature minisign.Signature
		require.NoError(t, signature.UnmarshalText([]byte(certificate[PartnerSignatureFileName])))
		return signature
	}
	forgedComment := parsed()
	forgedComment.TrustedComment = "partner:acme;expires:2199-12-31"
	forgedText, err := forgedComment.MarshalText()
	require.NoError(t, err)

	for name, files := range map[string]map[string]string{
		"signed by a key that is not an official plugin key": certify(t, partnerPublic, &stranger, "acme", "2099-12-31"),
		"trusted comment edited":                             withSignature(forgedText),
		"trusted comment without the partner fields": withSignature(
			minisign.Sign(*release, []byte(certificate[PartnerFileName]))),
		"key bytes changed after signing": {
			PartnerFileName:          encodeKey(t, partnerPublic) + "\n\n",
			PartnerSignatureFileName: certificate[PartnerSignatureFileName],
		},
		"signature missing": {PartnerFileName: certificate[PartnerFileName]},
		"key missing":       {PartnerSignatureFileName: certificate[PartnerSignatureFileName]},
		"key is not a key": {
			PartnerFileName:          "not a key\n",
			PartnerSignatureFileName: string(minisign.SignWithComments(*release, []byte("not a key\n"), "partner:acme;expires:2099-12-31", "")),
		},
	} {
		t.Run(name, func(t *testing.T) {
			trust, err := extractedTrust(t, certifiedPackage(t, "com.example.alpha", files, &partner), "", nil)
			require.NoError(t, err)
			assert.Equal(t, TrustUnsigned, trust.Trust)
		})
	}

	// A plugin.partner swapped after the partner signed the package does not
	// verify, and the partner key is known nowhere else.
	archive := certifiedPackage(t, "com.example.alpha", certificate, &partner)
	strangerPublic, _ := newSigningKey(t)
	tampered := rewritePackage(t, archive, func(entry *tarEntry) {
		if entry.header.Name == PartnerFileName {
			entry.body = encodeKey(t, strangerPublic) + "\n"
		}
	})
	trust, err := extractedTrust(t, tampered, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, trust.Trust)

	// When the keyring knows the key the swap breaks plugin.sums instead.
	_, err = extractedTrust(t, tampered, "", partnerKeyringOf(t, partnerPublic))
	assertPluginError(t, err, ErrSignatureInvalid)
}

func TestParsePartnerComment(t *testing.T) {
	name, expires, err := parsePartnerComment("partner:acme.io;expires:2027-03-04")
	require.NoError(t, err)
	assert.Equal(t, "acme.io", name)
	assert.Equal(t, time.Date(2027, 3, 4, 0, 0, 0, 0, time.UTC), expires)

	// The expiry is optional.
	name, expires, err = parsePartnerComment("partner:acme-io")
	require.NoError(t, err)
	assert.Equal(t, "acme-io", name)
	assert.True(t, expires.IsZero())

	for _, comment := range []string{
		"",
		"timestamp:1700000000",
		"partner:",
		"partner:acme;",
		"partner:acme;expires:",
		"expires:2027-03-04",
		"expires:2027-03-04;partner:acme",
		"partner: acme;expires:2027-03-04",
		"partner:acme; expires:2027-03-04",
		"partner:acme;scope:all",
		"partner:acme;expires:2027-3-4",
		"partner:acme;expires:tomorrow",
		"partner:acme corp;expires:2027-03-04",
		"partner:-acme;expires:2027-03-04",
		"partner:acme;partner:other;expires:2027-03-04",
		"partner:acme;expires:2027-03-04;scope:all",
		"partner:" + strings.Repeat("a", 65) + ";expires:2027-03-04",
	} {
		_, _, err = parsePartnerComment(comment)
		assert.Error(t, err, comment)
	}
}

func TestPartnerCertificateWithoutExpiryLastsUntilRevoked(t *testing.T) {
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	useNow(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	certificate := certify(t, partnerPublic, release, "acme", "")
	var signature minisign.Signature
	require.NoError(t, signature.UnmarshalText([]byte(certificate[PartnerSignatureFileName])))
	assert.Equal(t, "partner:acme", signature.TrustedComment)
	archive := certifiedPackage(t, "com.example.alpha", certificate, &partner)

	// Decades later the certificate still holds.
	useNow(t, time.Date(2076, 1, 1, 0, 0, 0, 0, time.UTC))
	trust, err := extractedTrust(t, archive, "", nil)
	require.NoError(t, err)
	assert.Equal(t, TrustVerified, trust.Trust)
	assert.Equal(t, "acme", trust.Partner)

	// A revocation in the keyring ends it.
	keyring := partnerKeyringOf(t)
	keyring.revoked[keyID(&partner)] = struct{}{}
	trust, err = extractedTrust(t, archive, "", keyring)
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, trust.Trust)
}

func TestNewPartnerCertificateChecksItsInput(t *testing.T) {
	useNow(t, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	release := useOfficialKey(t)
	partnerPublic, _ := newSigningKey(t)
	encoded := []byte(encodeKey(t, partnerPublic))

	// The bare key line is accepted and written in the two line form.
	bare := encoded[bytes.IndexByte(encoded, '\n')+1:]
	partnerFile, signature, err := NewPartnerCertificate(bare, "acme", "2026-06-01", *release)
	require.NoError(t, err)
	assert.Equal(t, encodeKey(t, partnerPublic)+"\n", string(partnerFile))
	certificate, err := checkPartnerCertificate(partnerFile, signature, nil)
	require.NoError(t, err)
	assert.Equal(t, "acme", certificate.Name)
	assert.Equal(t, fmt.Sprintf("%016X", partnerPublic.ID()), certificate.KeyID)

	for name, input := range map[string]struct{ key, name, expires string }{
		"bad key":      {key: "garbage", name: "acme", expires: "2027-01-01"},
		"bad name":     {key: string(encoded), name: "acme corp", expires: "2027-01-01"},
		"bad date":     {key: string(encoded), name: "acme", expires: "01/01/2027"},
		"date in past": {key: string(encoded), name: "acme", expires: "2026-05-31"},
	} {
		_, _, err = NewPartnerCertificate([]byte(input.key), input.name, input.expires, *release)
		assert.Error(t, err, name)
	}
}

func TestPartnerKeyringMakesAPackageVerified(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	useOfficialSource(t, server)
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	expiredPublic, expired := newSigningKey(t)

	document := keyringDocument(t, "2026-09-01T00:00:00Z", map[string]minisign.PublicKey{"acme": partnerPublic})
	document.Partners = append(document.Partners, partnerEntry{
		Name: "gone", PublicKey: encodeKey(t, expiredPublic), Expires: "2020-01-01",
	})
	served := server.servePartners(t, document, release)

	_, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	require.NotNil(t, manager.partnerKeyring())

	// A package without a certificate is verified by the keyring alone.
	gotTrust, signer, partnerName := inspectTrust(t, manager, signedWebappPackage(t, "com.example.alpha", &partner))
	assert.Equal(t, TrustVerified, gotTrust)
	assert.Equal(t, keyID(&partner), signer)
	assert.Equal(t, "acme", partnerName)

	// An expired entry proves nothing.
	gotTrust, _, _ = inspectTrust(t, manager, signedWebappPackage(t, "com.example.beta", &expired))
	assert.Equal(t, TrustUnsigned, gotTrust)

	// The verified document is cached next to the plugins.
	documentPath, signaturePath := manager.partnerCachePaths()
	cached, err := os.ReadFile(documentPath)
	require.NoError(t, err)
	assert.Equal(t, served, cached)
	assert.FileExists(t, signaturePath)

	// A revocation published later takes the key away.
	server.servePartners(t, keyringDocument(t, "2026-09-02T00:00:00Z",
		map[string]minisign.PublicKey{"acme": partnerPublic}, strings.ToLower(keyID(&partner))), release)
	_, err = manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	gotTrust, _, _ = inspectTrust(t, manager, signedWebappPackage(t, "com.example.alpha", &partner))
	assert.Equal(t, TrustUnsigned, gotTrust)
}

func TestPartnerKeyringRefusesARollback(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	useOfficialSource(t, server)
	release := useOfficialKey(t)
	newerPublic, newer := newSigningKey(t)
	olderPublic, older := newSigningKey(t)

	current := server.servePartners(t, keyringDocument(t, "2026-09-02T00:00:00Z",
		map[string]minisign.PublicKey{"newer": newerPublic}), release)
	manager.refreshPartners(context.Background())

	// An older document, even one an official plugin key signed, is refused.
	server.servePartners(t, keyringDocument(t, "2026-09-01T00:00:00Z",
		map[string]minisign.PublicKey{"older": olderPublic}), release)
	manager.refreshPartners(context.Background())

	gotTrust, _, _ := inspectTrust(t, manager, signedWebappPackage(t, "com.example.alpha", &newer))
	assert.Equal(t, TrustVerified, gotTrust)
	gotTrust, _, _ = inspectTrust(t, manager, signedWebappPackage(t, "com.example.beta", &older))
	assert.Equal(t, TrustUnsigned, gotTrust)

	documentPath, _ := manager.partnerCachePaths()
	cached, err := os.ReadFile(documentPath)
	require.NoError(t, err)
	assert.Equal(t, current, cached)

	// The rollback check also holds against a cache loaded after a restart.
	restarted := newManager(manager.Dir())
	require.NoError(t, restarted.LoadOffline(context.Background()))
	restarted.refreshPartners(context.Background())
	gotTrust, _, _ = inspectTrust(t, restarted, signedWebappPackage(t, "com.example.beta", &older))
	assert.Equal(t, TrustUnsigned, gotTrust)
}

func TestPartnerKeyringWithABadSignatureKeepsTheCache(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	useOfficialSource(t, server)
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	otherPublic, _ := newSigningKey(t)
	_, stranger := newSigningKey(t)

	current := server.servePartners(t, keyringDocument(t, "2026-09-01T00:00:00Z",
		map[string]minisign.PublicKey{"acme": partnerPublic}), release)
	manager.refreshPartners(context.Background())
	require.NotNil(t, manager.partnerKeyring())

	// A newer document that revokes the partner, signed by a stranger.
	server.servePartners(t, keyringDocument(t, "2026-09-02T00:00:00Z",
		map[string]minisign.PublicKey{"other": otherPublic}, keyID(&partner)), &stranger)
	manager.refreshPartners(context.Background())

	// A document changed after an official plugin key signed it.
	raw := server.servePartners(t, keyringDocument(t, "2026-09-03T00:00:00Z",
		map[string]minisign.PublicKey{"acme": partnerPublic}), release)
	server.serve("/"+PartnersFileName, bytes.Replace(raw, []byte("acme"), []byte("evil"), 1))
	manager.refreshPartners(context.Background())

	// A keyring that cannot be fetched at all.
	server.mu.Lock()
	delete(server.files, "/"+partnersSignatureName)
	server.mu.Unlock()
	manager.refreshPartners(context.Background())

	gotTrust, _, partnerName := inspectTrust(t, manager, signedWebappPackage(t, "com.example.alpha", &partner))
	assert.Equal(t, TrustVerified, gotTrust)
	assert.Equal(t, "acme", partnerName)

	documentPath, _ := manager.partnerCachePaths()
	cached, err := os.ReadFile(documentPath)
	require.NoError(t, err)
	assert.Equal(t, current, cached)
}

func TestPartnerKeyringCacheSurvivesANewManager(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	useOfficialSource(t, server)
	release := useOfficialKey(t)
	partnerPublic, partner := newSigningKey(t)
	ctx := context.Background()

	server.servePartners(t, keyringDocument(t, "2026-09-01T00:00:00Z",
		map[string]minisign.PublicKey{"acme": partnerPublic}), release)
	manager.refreshPartners(ctx)
	documentPath, signaturePath := manager.partnerCachePaths()
	cachedDocument, err := os.ReadFile(documentPath)
	require.NoError(t, err)
	cachedSignature, err := os.ReadFile(signaturePath)
	require.NoError(t, err)

	// The node goes offline and restarts.
	server.Close()
	restarted := newManager(manager.Dir())
	restarted.Start(ctx)
	t.Cleanup(func() { restarted.Stop(ctx) })
	require.NotNil(t, restarted.partnerKeyring())

	gotTrust, signer, partnerName := inspectTrust(t, restarted, signedWebappPackage(t, "com.example.alpha", &partner))
	assert.Equal(t, TrustVerified, gotTrust)
	assert.Equal(t, keyID(&partner), signer)
	assert.Equal(t, "acme", partnerName)

	// The command line tools load the cache as well.
	offline := newManager(manager.Dir())
	require.NoError(t, offline.LoadOffline(ctx))
	require.NotNil(t, offline.partnerKeyring())

	// A cache that does not verify is dropped, here one edited on disk and
	// one a rotated official plugin key no longer verifies.
	for name, prepare := range map[string]func(t *testing.T){
		"edited on disk": func(t *testing.T) {
			edited := bytes.Replace(cachedDocument, []byte("acme"), []byte("evil"), 1)
			require.NoError(t, os.WriteFile(documentPath, edited, 0o644))
		},
		"rotated official plugin key": func(t *testing.T) { useOfficialKey(t) },
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(documentPath, cachedDocument, 0o644))
			require.NoError(t, os.WriteFile(signaturePath, cachedSignature, 0o644))
			prepare(t)

			dropped := newManager(manager.Dir())
			require.NoError(t, dropped.LoadOffline(ctx))
			assert.Nil(t, dropped.partnerKeyring())
			assert.NoFileExists(t, documentPath)
			assert.NoFileExists(t, signaturePath)
		})
	}
}

func TestPartnerKeyringOnlyComesFromTheOfficialSource(t *testing.T) {
	manager := newTestManager(t)
	official := newCatalogServer(t)
	custom := newCatalogServer(t)
	release := useOfficialKey(t)
	officialPublic, officialPartner := newSigningKey(t)
	customPublic, customPartner := newSigningKey(t)
	official.servePartners(t, keyringDocument(t, "2026-09-01T00:00:00Z",
		map[string]minisign.PublicKey{"acme": officialPublic}), release)
	custom.servePartners(t, keyringDocument(t, "2026-09-01T00:00:00Z",
		map[string]minisign.PublicKey{"evil": customPublic}), release)
	partnersPath := "/" + PartnersFileName
	requests := func(server *catalogServer) int {
		server.mu.Lock()
		defer server.mu.Unlock()
		return server.requests[partnersPath]
	}

	// Nothing is fetched while the marketplace is off.
	useMarketplace(t, custom.catalogURL())
	useOfficialSource(t, official)
	settings.PluginSettings.MarketplaceEnabled = false
	manager.refreshPartners(context.Background())
	assert.Nil(t, manager.partnerKeyring())
	assert.Zero(t, requests(official))

	// A node that only lists a custom source still gets the official keyring,
	// and the custom source is never asked for one.
	settings.PluginSettings.MarketplaceEnabled = true
	_, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	require.NotNil(t, manager.partnerKeyring())
	assert.Equal(t, 1, requests(official))
	assert.Zero(t, requests(custom))

	gotTrust, _, partnerName := inspectTrust(t, manager, signedWebappPackage(t, "com.example.alpha", &officialPartner))
	assert.Equal(t, TrustVerified, gotTrust)
	assert.Equal(t, "acme", partnerName)
	gotTrust, _, _ = inspectTrust(t, manager, signedWebappPackage(t, "com.example.beta", &customPartner))
	assert.Equal(t, TrustUnsigned, gotTrust)

	// A cached catalog is no refresh, so it fetches nothing.
	_, err = manager.Marketplace().Catalog(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, requests(official))
}

func TestParsePartnerKeyringRefusesMalformedDocuments(t *testing.T) {
	release := useOfficialKey(t)
	partnerPublic, _ := newSigningKey(t)
	valid := func() partnersDocument {
		return keyringDocument(t, "2026-09-01T00:00:00Z", map[string]minisign.PublicKey{"acme": partnerPublic}, "0123456789abcdef")
	}
	parse := func(document partnersDocument) (*partnerKeyring, error) {
		raw, err := json.Marshal(document)
		require.NoError(t, err)
		signature, err := signSums(raw, *release)
		require.NoError(t, err)
		return parsePartnerKeyring(raw, signature)
	}

	keyring, err := parse(valid())
	require.NoError(t, err)
	assert.Len(t, keyring.partnerKeys(), 1)
	assert.True(t, keyring.isRevoked("0123456789ABCDEF"))

	for name, mutate := range map[string]func(document *partnersDocument){
		"schema version": func(document *partnersDocument) { document.SchemaVersion = 2 },
		"updated at":     func(document *partnersDocument) { document.UpdatedAt = "yesterday" },
		"partner name":   func(document *partnersDocument) { document.Partners[0].Name = "acme corp" },
		"partner key":    func(document *partnersDocument) { document.Partners[0].PublicKey = "garbage" },
		"expiry":         func(document *partnersDocument) { document.Partners[0].Expires = "soon" },
		"revoked id":     func(document *partnersDocument) { document.Revoked = []string{"XYZ"} },
	} {
		document := valid()
		mutate(&document)
		_, err = parse(document)
		assert.Error(t, err, name)
	}
}
