package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	"github.com/0xJacky/Nginx-UI/settings"
)

// A partner certificate is two ordinary files at the package root:
// plugin.partner holds the partner public key, plugin.partner.minisig is a
// release key signature over it whose trusted comment names the partner.
const (
	PartnerFileName          = "plugin.partner"
	PartnerSignatureFileName = "plugin.partner.minisig"
)

// PartnersFileName is the signed partner keyring published next to the
// official catalog.
const PartnersFileName = "partners.json"

const (
	// partnersSchemaVersion is the only keyring layout this host understands.
	partnersSchemaVersion = 1
	// partnersSignatureName is the detached signature of the keyring.
	partnersSignatureName = PartnersFileName + ".minisig"
	// maxPartnersBytes bounds the keyring document.
	maxPartnersBytes = 1 << 20
	// maxPartnersSignatureBytes bounds its signature.
	maxPartnersSignatureBytes = 16 << 10
	// partnerDateLayout is the expiry format, a UTC calendar date.
	partnerDateLayout = time.DateOnly
)

// officialSource is the catalog whose keyring is used, a variable so the
// tests can point it at a local server.
var officialSource = settings.DefaultPluginMarketplaceSource

// SetOfficialSourceForTesting points the keyring fetch at url, or turns it
// off with an empty string, and returns a function that restores the previous
// value. Tests of other packages use it so they never reach the network.
func SetOfficialSourceForTesting(url string) (restore func()) {
	previous := officialSource
	officialSource = url
	return func() { officialSource = previous }
}

// partnerNamePattern is a short identifier: letters, digits, dots, hyphens.
var partnerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,63}$`)

// partnerKeyIDPattern is a minisign key id in hex.
var partnerKeyIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)

// Certificate problems the linter reports under their own rules.
var (
	errPartnerIncomplete = errors.New("partner certificate is incomplete")
	errPartnerExpired    = errors.New("partner certificate has expired")
	errPartnerRevoked    = errors.New("partner key is revoked")
)

// errPartnersInvalid marks a cached keyring that no longer verifies.
var errPartnersInvalid = errors.New("partner keyring is invalid")

// partnerCertificate names a partner key and, optionally, the last day it is
// valid. It is what plugin.partner proves and what one keyring entry lists.
type partnerCertificate struct {
	Name string
	// KeyID is the minisign key id in upper case hex.
	KeyID string
	// Key is the public key in its two line text form.
	Key string
	// Expires is the last valid day at midnight UTC, zero for no expiry.
	Expires time.Time
}

// validAt reports whether the certificate is still valid at moment.
func (c *partnerCertificate) validAt(moment time.Time) bool {
	return c.Expires.IsZero() || moment.UTC().Before(c.Expires.AddDate(0, 0, 1))
}

// partnersDocument is the layout of partners.json.
type partnersDocument struct {
	SchemaVersion int            `json:"schema_version"`
	UpdatedAt     string         `json:"updated_at"`
	Partners      []partnerEntry `json:"partners"`
	Revoked       []string       `json:"revoked"`
}

// partnerEntry is one partner of the keyring.
type partnerEntry struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	Expires   string `json:"expires,omitempty"`
}

// partnerKeyring is a verified partners.json. A nil keyring lists nothing.
type partnerKeyring struct {
	updatedAt time.Time
	partners  []partnerCertificate
	revoked   map[string]struct{}
	// raw and signature are the verified bytes the cache keeps.
	raw       []byte
	signature []byte
}

// partnerKeys lists the keyring keys that are neither expired nor revoked.
func (k *partnerKeyring) partnerKeys() []partnerCertificate {
	if k == nil {
		return nil
	}
	moment := now()
	keys := make([]partnerCertificate, 0, len(k.partners))
	for _, partner := range k.partners {
		if partner.validAt(moment) && !k.isRevoked(partner.KeyID) {
			keys = append(keys, partner)
		}
	}
	return keys
}

// isRevoked reports whether the keyring revoked a key id.
func (k *partnerKeyring) isRevoked(keyID string) bool {
	if k == nil {
		return false
	}
	_, revoked := k.revoked[strings.ToUpper(keyID)]
	return revoked
}

// partnerStore is the keyring state of one manager.
type partnerStore struct {
	mu      sync.RWMutex
	keyring *partnerKeyring
	// problem is the last keyring problem logged at warn level.
	problem string

	// refreshMu keeps the rollback check and the cache write together.
	refreshMu sync.Mutex
}

// partnerKeyring returns the keyring of this manager, nil when there is none.
func (m *Manager) partnerKeyring() *partnerKeyring {
	m.partners.mu.RLock()
	defer m.partners.mu.RUnlock()
	return m.partners.keyring
}

// setPartnerKeyring replaces the keyring of this manager.
func (m *Manager) setPartnerKeyring(keyring *partnerKeyring) {
	m.partners.mu.Lock()
	m.partners.keyring = keyring
	m.partners.mu.Unlock()
}

// partnerCachePaths are where the verified keyring and its signature live.
func (m *Manager) partnerCachePaths() (document, signature string) {
	return m.partnerCachePath(PartnersFileName), m.partnerCachePath(partnersSignatureName)
}

// partnerCachePath is the hidden cache file for one keyring file.
func (m *Manager) partnerCachePath(name string) string {
	return filepath.Join(m.Dir(), "."+name)
}

// loadPartnerCache reads the cached keyring, so a restart and an offline
// node keep the partners. Its signature is verified again, and a cache that
// fails is dropped.
func (m *Manager) loadPartnerCache() {
	keyring, err := m.readPartnerCache()
	switch {
	case errors.Is(err, errPartnersInvalid):
		m.setPartnerKeyring(nil)
		m.dropPartnerCache()
		m.warnPartners(fmt.Errorf("cached %s is dropped: %w", PartnersFileName, err))
	case err != nil:
		m.warnPartners(fmt.Errorf("cached %s: %w", PartnersFileName, err))
	case keyring != nil:
		m.setPartnerKeyring(keyring)
	}
}

// dropPartnerCache removes the cached keyring files.
func (m *Manager) dropPartnerCache() {
	for _, path := range []string{m.partnerCachePath(PartnersFileName), m.partnerCachePath(partnersSignatureName)} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			m.log.Warnf("Remove %s: %v", path, err)
		}
	}
}

// readPartnerCache verifies the cached keyring again, nil when there is none.
// A cache that does not verify or parse is errPartnersInvalid.
func (m *Manager) readPartnerCache() (*partnerKeyring, error) {
	documentPath, signaturePath := m.partnerCachePaths()
	raw, err := os.ReadFile(documentPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	signature, err := os.ReadFile(signaturePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s is missing", errPartnersInvalid, filepath.Base(signaturePath))
	}
	if err != nil {
		return nil, err
	}
	keyring, err := parsePartnerKeyring(raw, signature)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPartnersInvalid, err)
	}
	return keyring, nil
}

// refreshPartners fetches the keyring next to the official catalog, verifies
// it and caches it. Anything that goes wrong keeps the current keyring.
func (m *Manager) refreshPartners(ctx context.Context) {
	documentURL := partnersURL()
	if documentURL == "" {
		return
	}
	m.partners.refreshMu.Lock()
	defer m.partners.refreshMu.Unlock()

	if err := m.fetchPartners(ctx, documentURL); err != nil {
		m.warnPartners(err)
		return
	}
	m.partners.mu.Lock()
	m.partners.problem = ""
	m.partners.mu.Unlock()
}

// fetchPartners is one refresh: fetch, verify, refuse a rollback, cache.
func (m *Manager) fetchPartners(ctx context.Context, documentURL string) error {
	raw, err := fetchText(ctx, proxiedURL(documentURL), maxPartnersBytes)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", PartnersFileName, err)
	}
	signature, err := fetchText(ctx, proxiedURL(documentURL+".minisig"), maxPartnersSignatureBytes)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", partnersSignatureName, err)
	}
	keyring, err := parsePartnerKeyring([]byte(raw), []byte(signature))
	if err != nil {
		return fmt.Errorf("fetched %s: %w", PartnersFileName, err)
	}

	current := m.partnerKeyring()
	if current == nil {
		// An unusable cache proves nothing, so it cannot refuse a document.
		current, _ = m.readPartnerCache()
	}
	if current != nil {
		if keyring.updatedAt.Before(current.updatedAt) {
			return fmt.Errorf("fetched %s updated at %s is older than the cached one from %s",
				PartnersFileName, keyring.updatedAt.Format(time.RFC3339), current.updatedAt.Format(time.RFC3339))
		}
		if bytes.Equal(keyring.raw, current.raw) && bytes.Equal(keyring.signature, current.signature) {
			m.setPartnerKeyring(current)
			return nil
		}
	}

	m.setPartnerKeyring(keyring)
	if err = m.writePartnerCache(keyring); err != nil {
		return fmt.Errorf("cache %s: %w", PartnersFileName, err)
	}
	return nil
}

// writePartnerCache stores the verified keyring bytes next to the plugins.
func (m *Manager) writePartnerCache(keyring *partnerKeyring) error {
	documentPath, signaturePath := m.partnerCachePaths()
	if err := os.MkdirAll(filepath.Dir(documentPath), 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(signaturePath, keyring.signature); err != nil {
		return err
	}
	return writeFileAtomic(documentPath, keyring.raw)
}

// warnPartners logs a keyring problem at warn level once, a repeat at debug.
func (m *Manager) warnPartners(err error) {
	message := err.Error()
	m.partners.mu.Lock()
	repeated := m.partners.problem == message
	m.partners.problem = message
	m.partners.mu.Unlock()
	if repeated {
		m.log.Debugf("Plugin partner keyring: %v", err)
		return
	}
	m.log.Warnf("Plugin partner keyring: %v", err)
}

// partnersURL is the keyring URL next to the official catalog, empty while
// the plugin system or the marketplace is off. It does not depend on the
// configured sources, so a node that only uses a mirror still receives the
// revocations, and a custom source never publishes partners.
func partnersURL() string {
	if officialSource == "" || !settings.PluginSettings.Enabled || !settings.PluginSettings.MarketplaceEnabled {
		return ""
	}
	base, err := url.Parse(officialSource)
	if err != nil || base.Host == "" {
		return ""
	}
	return base.ResolveReference(&url.URL{Path: PartnersFileName}).String()
}

// parsePartnerKeyring verifies partners.json with the release keys and
// parses it. Any malformed field refuses the whole document.
func parsePartnerKeyring(raw, signature []byte) (*partnerKeyring, error) {
	if _, err := pkgsign.VerifyBytes(raw, signature, releaseKeys()); err != nil {
		return nil, fmt.Errorf("signature does not verify with a release key: %w", err)
	}
	var document partnersDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	if document.SchemaVersion != partnersSchemaVersion {
		return nil, fmt.Errorf("schema_version %d is not supported", document.SchemaVersion)
	}
	updatedAt, err := time.Parse(time.RFC3339, document.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updated_at: %w", err)
	}

	keyring := &partnerKeyring{
		updatedAt: updatedAt,
		partners:  make([]partnerCertificate, 0, len(document.Partners)),
		revoked:   make(map[string]struct{}, len(document.Revoked)),
		raw:       bytes.Clone(raw),
		signature: bytes.Clone(signature),
	}
	for i, entry := range document.Partners {
		if !partnerNamePattern.MatchString(entry.Name) {
			return nil, fmt.Errorf("partner %d has an invalid name %q", i+1, entry.Name)
		}
		key, err := parsePartnerKey([]byte(entry.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("partner %s: %w", entry.Name, err)
		}
		partner := partnerCertificate{Name: entry.Name, KeyID: key.KeyID, Key: key.Key}
		if entry.Expires != "" {
			if partner.Expires, err = parsePartnerDate(entry.Expires); err != nil {
				return nil, fmt.Errorf("partner %s: %w", entry.Name, err)
			}
		}
		keyring.partners = append(keyring.partners, partner)
	}
	for _, keyID := range document.Revoked {
		if !partnerKeyIDPattern.MatchString(keyID) {
			return nil, fmt.Errorf("revoked key id %q is not 16 hex digits", keyID)
		}
		keyring.revoked[strings.ToUpper(keyID)] = struct{}{}
	}
	return keyring, nil
}

// readPartnerCertificate checks the certificate at the root of an extracted
// package. It returns nil without an error when the package carries none.
func readPartnerCertificate(root string, keyring *partnerKeyring) (*partnerCertificate, error) {
	partner, err := readRootFile(root, PartnerFileName)
	if err != nil {
		return nil, err
	}
	signature, err := readRootFile(root, PartnerSignatureFileName)
	if err != nil {
		return nil, err
	}
	switch {
	case partner == nil && signature == nil:
		return nil, nil
	case partner == nil:
		return nil, fmt.Errorf("%w: %s is present without %s", errPartnerIncomplete, PartnerSignatureFileName, PartnerFileName)
	case signature == nil:
		return nil, fmt.Errorf("%w: %s is present without %s", errPartnerIncomplete, PartnerFileName, PartnerSignatureFileName)
	}
	return checkPartnerCertificate(partner, signature, keyring)
}

// checkPartnerCertificate verifies a certificate: a release key signed the
// exact partner key bytes, the trusted comment parses, the certificate has
// not expired when it carries an expiry and the keyring did not revoke the
// key. Without an expiry only a revocation ends it.
func checkPartnerCertificate(partner, signature []byte, keyring *partnerKeyring) (*partnerCertificate, error) {
	if _, err := pkgsign.VerifyBytes(partner, signature, releaseKeys()); err != nil {
		return nil, fmt.Errorf("%s does not verify with a release key: %w", PartnerSignatureFileName, err)
	}
	var parsed minisign.Signature
	if err := parsed.UnmarshalText(signature); err != nil {
		return nil, err
	}
	name, expires, err := parsePartnerComment(parsed.TrustedComment)
	if err != nil {
		return nil, err
	}
	key, err := parsePartnerKey(partner)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", PartnerFileName, err)
	}

	certificate := &partnerCertificate{Name: name, KeyID: key.KeyID, Key: key.Key, Expires: expires}
	if !certificate.validAt(now()) {
		return nil, fmt.Errorf("%w: partner %s expired on %s", errPartnerExpired, name, expires.Format(partnerDateLayout))
	}
	if keyring.isRevoked(certificate.KeyID) {
		return nil, fmt.Errorf("%w: key %s of partner %s", errPartnerRevoked, certificate.KeyID, name)
	}
	return certificate, nil
}

// parsePartnerKey reads a minisign public key, with or without the comment
// line, and returns its id and its canonical text form.
func parsePartnerKey(text []byte) (partnerCertificate, error) {
	var key minisign.PublicKey
	if err := key.UnmarshalText(bytes.TrimSpace(text)); err != nil {
		return partnerCertificate{}, err
	}
	encoded, err := key.MarshalText()
	if err != nil {
		return partnerCertificate{}, err
	}
	return partnerCertificate{KeyID: fmt.Sprintf("%016X", key.ID()), Key: string(encoded)}, nil
}

// parsePartnerComment reads the trusted comment of a certificate, exactly
// "partner:<name>" optionally followed by ";expires:<YYYY-MM-DD>". The
// expiry is zero when absent.
func parsePartnerComment(comment string) (string, time.Time, error) {
	malformed := fmt.Errorf("trusted comment %q is not partner:<name> or partner:<name>;expires:<YYYY-MM-DD>", comment)
	rest, ok := strings.CutPrefix(comment, "partner:")
	if !ok {
		return "", time.Time{}, malformed
	}
	name, expiry, hasExpiry := strings.Cut(rest, ";")
	if !partnerNamePattern.MatchString(name) {
		return "", time.Time{}, malformed
	}
	if !hasExpiry {
		return name, time.Time{}, nil
	}
	value, ok := strings.CutPrefix(expiry, "expires:")
	if !ok {
		return "", time.Time{}, malformed
	}
	day, err := parsePartnerDate(value)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %w", malformed, err)
	}
	return name, day, nil
}

// parsePartnerDate reads an expiry date, a UTC calendar date.
func parsePartnerDate(value string) (time.Time, error) {
	day, err := time.Parse(partnerDateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("expires %q is not a YYYY-MM-DD date", value)
	}
	return day, nil
}

// NewPartnerCertificate issues a partner certificate with a release key. It
// returns the contents of plugin.partner, the partner key in its two line
// form, and of plugin.partner.minisig, whose trusted comment names the
// partner and, unless expires is empty, the last day the certificate is valid.
func NewPartnerCertificate(partnerKey []byte, name, expires string, releaseKey minisign.PrivateKey) (partner, signature []byte, err error) {
	key, err := parsePartnerKey(partnerKey)
	if err != nil {
		return nil, nil, fmt.Errorf("partner key: %w", err)
	}
	if !partnerNamePattern.MatchString(name) {
		return nil, nil, fmt.Errorf("partner name %q may only hold letters, digits, dots and hyphens", name)
	}
	comment := "partner:" + name
	if expires != "" {
		day, err := parsePartnerDate(expires)
		if err != nil {
			return nil, nil, err
		}
		certificate := partnerCertificate{Name: name, KeyID: key.KeyID, Expires: day}
		if !certificate.validAt(now()) {
			return nil, nil, fmt.Errorf("expires %s is already in the past", expires)
		}
		comment += ";expires:" + day.Format(partnerDateLayout)
	}

	partner = []byte(key.Key + "\n")
	reader := minisign.NewReader(bytes.NewReader(partner))
	if _, err = io.Copy(io.Discard, reader); err != nil {
		return nil, nil, err
	}
	untrusted := fmt.Sprintf("partner certificate of %s, key %s", name, key.KeyID)
	return partner, reader.SignWithComments(releaseKey, comment, untrusted), nil
}

// writeFileAtomic writes a file through a temporary file and a rename.
func writeFileAtomic(target string, body []byte) error {
	file, err := os.CreateTemp(filepath.Dir(target), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Chmod(file.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(file.Name(), target)
}
