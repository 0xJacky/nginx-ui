package plugin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	"github.com/0xJacky/Nginx-UI/internal/releasesign"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// A package is signed by two files at its root: plugin.sums lists the sha256
// of every other regular file, plugin.sums.minisig is a minisign signature
// over plugin.sums.
const (
	SumsFileName          = "plugin.sums"
	SumsSignatureFileName = "plugin.sums.minisig"
)

// Trust levels, derived from the key that signed a package. A catalog entry
// declares one as well, which is only shown in the listing. Verified comes
// from a partner key: one a partner certificate in the package proves, or
// one the signed partner keyring lists, see partners.go.
const (
	TrustOfficial  = "official"
	TrustVerified  = "verified"
	TrustCommunity = "community"
	TrustUnsigned  = "unsigned"
)

// The release keys pinned in the binary and the clock the expiry checks use,
// behind variables so the tests can swap them.
var (
	releaseKeys = releasesign.TrustedPublicKeys
	now         = time.Now
)

// packageTrust is what the embedded signature of a package proves.
type packageTrust struct {
	Trust string
	// Signer is the minisign key id in upper case hex, empty when unsigned.
	Signer string
	// AuthorKey is the public key that verified a community signature, which
	// a cluster push hands on to the node. Empty for any other trust.
	AuthorKey string
	// Partner is the partner name of a verified signature, empty otherwise.
	Partner string
}

var unsignedTrust = packageTrust{Trust: TrustUnsigned}

// trustRank orders the trust levels, unsigned and unknown values lowest.
func trustRank(trust string) int {
	switch trust {
	case TrustOfficial:
		return 3
	case TrustVerified:
		return 2
	case TrustCommunity:
		return 1
	default:
		return 0
	}
}

// trustTier is a key set and the trust a signature by one of its keys earns.
type trustTier struct {
	trust string
	keys  []string
}

// trustTiers lists the known keys from the highest trust down. authorKey is
// the catalog key of the entry a package came from, empty on other paths.
// partners are the certificate and keyring keys of the verified tier.
func trustTiers(authorKey string, partners []partnerCertificate, keyring *partnerKeyring) []trustTier {
	community := slices.Clone(settings.PluginSettings.TrustedPublicKeys)
	if strings.TrimSpace(authorKey) != "" {
		community = append(community, authorKey)
	}
	return append(partnerTiers(partners, keyring), trustTier{trust: TrustCommunity, keys: community})
}

// partnerTiers are the release keys and the partner keys the keyring did not
// revoke. Without the community keys it is all the linter knows.
func partnerTiers(partners []partnerCertificate, keyring *partnerKeyring) []trustTier {
	verified := make([]string, 0, len(partners))
	for _, partner := range partners {
		if !keyring.isRevoked(partner.KeyID) {
			verified = append(verified, partner.Key)
		}
	}
	return []trustTier{
		{trust: TrustOfficial, keys: releaseKeys()},
		{trust: TrustVerified, keys: verified},
	}
}

// packagePartners lists the partner keys for one extracted package: the key
// of a valid certificate first, then the keyring keys. A certificate that
// does not hold up is ignored with a debug log.
func packagePartners(root string, keyring *partnerKeyring) []partnerCertificate {
	partners := keyring.partnerKeys()
	certificate, err := readPartnerCertificate(root, keyring)
	if err != nil {
		logger.Debugf("Ignore the plugin partner certificate: %v", err)
		return partners
	}
	if certificate == nil {
		return partners
	}
	return append([]partnerCertificate{*certificate}, partners...)
}

// partnerName is the name a partner key id was issued to.
func partnerName(partners []partnerCertificate, keyID string) string {
	for _, partner := range partners {
		if partner.KeyID == keyID {
			return partner.Name
		}
	}
	return ""
}

// checkPackageTrust verifies the embedded signature of an extracted package
// and applies the node policy and the caller's floor to the derived trust.
func checkPackageTrust(root string, opts InstallOptions, keyring *partnerKeyring) (packageTrust, error) {
	trust, err := verifyPackageSignature(root, opts.AuthorPublicKey, keyring)
	if err != nil {
		return trust, err
	}
	if opts.MinTrust != "" && trustRank(trust.Trust) < trustRank(opts.MinTrust) {
		return trust, cosy.WrapErrorWithParams(ErrTrustDowngrade, trust.Trust, opts.MinTrust)
	}
	switch trust.Trust {
	case TrustUnsigned:
		if !settings.PluginSettings.DeveloperMode {
			return trust, ErrUnsignedPackage
		}
	case TrustCommunity:
		if !settings.PluginSettings.AllowCommunityPlugins {
			return trust, ErrCommunityNotAllowed
		}
	}
	return trust, nil
}

// verifyPackageSignature derives the trust of an extracted package. Missing
// signature files or an unknown signer make it unsigned. A signature a known
// key does not verify, or sums that do not match the files, are refused with
// ErrSignatureInvalid. keyring is the partner keyring, nil for none.
func verifyPackageSignature(root, authorKey string, keyring *partnerKeyring) (packageTrust, error) {
	sums, err := readRootFile(root, SumsFileName)
	if err != nil {
		return unsignedTrust, err
	}
	signature, err := readRootFile(root, SumsSignatureFileName)
	if err != nil {
		return unsignedTrust, err
	}
	if sums == nil || signature == nil {
		return unsignedTrust, nil
	}

	partners := packagePartners(root, keyring)
	trust, err := signatureTrust(sums, signature, trustTiers(authorKey, partners, keyring))
	if err != nil || trust.Trust == TrustUnsigned {
		return unsignedTrust, err
	}
	if err = checkSums(root, sums); err != nil {
		return unsignedTrust, cosy.WrapErrorWithParams(ErrSignatureInvalid, err.Error())
	}
	if trust.Trust == TrustVerified {
		trust.Partner = partnerName(partners, trust.Signer)
	}
	return trust, nil
}

// signatureTrust returns the highest trust whose key verifies the signature
// over sums. A key that is not known leaves the package unsigned, a known key
// that fails and a signature that does not parse are ErrSignatureInvalid.
func signatureTrust(sums, signature []byte, tiers []trustTier) (packageTrust, error) {
	keyID, err := pkgsign.KeyID(signature)
	if err != nil {
		return unsignedTrust, cosy.WrapErrorWithParams(ErrSignatureInvalid, err.Error())
	}

	failed := false
	for _, tier := range tiers {
		// One key at a time, so a broken or duplicate key only skips itself.
		for _, key := range tier.keys {
			_, err = pkgsign.VerifyBytes(sums, signature, []string{key})
			if err == nil {
				trust := packageTrust{Trust: tier.trust, Signer: fmt.Sprintf("%016X", keyID)}
				if tier.trust == TrustCommunity {
					trust.AuthorKey = strings.TrimSpace(key)
				}
				return trust, nil
			}
			if errors.Is(err, pkgsign.ErrSignatureInvalid) {
				failed = true
			}
		}
	}
	if failed {
		return unsignedTrust, cosy.WrapErrorWithParams(ErrSignatureInvalid,
			fmt.Sprintf("%s does not verify with key %016X", SumsFileName, keyID))
	}
	return unsignedTrust, nil
}

// readRootFile reads one signature file, nil when it is not a regular file.
func readRootFile(root, name string) ([]byte, error) {
	target := filepath.Join(root, name)
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	return os.ReadFile(target)
}

// checkSums compares plugin.sums with the regular files under root: every
// file is listed with its digest and every listed file exists.
func checkSums(root string, sums []byte) error {
	listed, err := parseSums(sums)
	if err != nil {
		return err
	}
	return matchSums(root, listed)
}

// matchSums compares parsed plugin.sums lines with the files under root.
func matchSums(root string, listed map[string]string) error {
	files, err := sumsFiles(root)
	if err != nil {
		return err
	}
	for _, name := range files {
		want, ok := listed[name]
		if !ok {
			return fmt.Errorf("%s is not listed in %s", name, SumsFileName)
		}
		got, err := fileDigest(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("%s does not match its digest in %s", name, SumsFileName)
		}
		delete(listed, name)
	}
	if len(listed) > 0 {
		missing := slices.Sorted(maps.Keys(listed))
		return fmt.Errorf("%s is listed in %s but missing", missing[0], SumsFileName)
	}
	return nil
}

// parseSums reads the "<sha256>  <path>" lines of plugin.sums, each ending
// with LF and sorted bytewise by path, as formatSums writes them.
func parseSums(sums []byte) (map[string]string, error) {
	listed := map[string]string{}
	if len(sums) == 0 {
		return listed, nil
	}
	text, ok := strings.CutSuffix(string(sums), "\n")
	if !ok {
		return nil, fmt.Errorf("%s does not end with a newline", SumsFileName)
	}
	previous := ""
	for number, line := range strings.Split(text, "\n") {
		digest, name, ok := strings.Cut(line, "  ")
		if !ok || !isSHA256Hex(digest) || !isSafeRelPath(name) || strings.ContainsAny(name, "\r\n") {
			return nil, fmt.Errorf("%s line %d is malformed", SumsFileName, number+1)
		}
		if number > 0 && name <= previous {
			return nil, fmt.Errorf("%s line %d is out of order or repeats %s", SumsFileName, number+1, name)
		}
		previous = name
		listed[name] = digest
	}
	return listed, nil
}

// formatSums renders plugin.sums: one line per file, sorted bytewise by path.
func formatSums(digests map[string]string) []byte {
	var buffer bytes.Buffer
	for _, name := range slices.Sorted(maps.Keys(digests)) {
		buffer.WriteString(digests[name])
		buffer.WriteString("  ")
		buffer.WriteString(name)
		buffer.WriteByte('\n')
	}
	return buffer.Bytes()
}

// signSums signs plugin.sums the way "minisign -S" does, prehashed.
func signSums(sums []byte, key minisign.PrivateKey) ([]byte, error) {
	reader := minisign.NewReader(bytes.NewReader(sums))
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return nil, err
	}
	return reader.Sign(key), nil
}

// sumsFiles lists the slash separated paths of the regular files under root
// that plugin.sums has to cover.
func sumsFiles(root string) ([]string, error) {
	files := make([]string, 0, 32)
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if !isSignatureFile(name) {
			files = append(files, name)
		}
		return nil
	})
	return files, err
}

// isSignatureFile reports whether a package path is one of the two files
// the signature consists of, which plugin.sums never lists.
func isSignatureFile(name string) bool {
	return name == SumsFileName || name == SumsSignatureFileName
}

// fileDigest is the lowercase hex sha256 of a file.
func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// isSHA256Hex reports whether value is a lowercase hex sha256 digest.
func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
