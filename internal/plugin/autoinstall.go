package plugin

import (
	"context"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
)

// recursiveNameserversKey is the setting the DNS-01 plugin reads the legacy
// core resolver list from.
const recursiveNameserversKey = "recursive_nameservers"

// autoInstallMu serialises the whole auto install pass so the boot goroutine
// and the daily job never run it twice at the same time.
var autoInstallMu sync.Mutex

// EnsureDNS01Plugin keeps a node that issues DNS-01 certificates working after
// the provider code moved into a plugin: when a certificate needs DNS-01 and
// the official plugin is not installed, it is fetched from the offline package
// directory or from the marketplace. It is safe to call repeatedly.
func (m *Manager) EnsureDNS01Plugin(ctx context.Context) {
	if !settings.PluginSettings.Enabled {
		return
	}

	autoInstallMu.Lock()
	defer autoInstallMu.Unlock()

	// A core upgrade can leave plugins behind, repair them before deciding
	// that the DNS-01 plugin is missing.
	m.repairIncompatiblePlugins(ctx)

	if m.isUsable(OfficialDNS01PluginID) {
		return
	}
	if !dns01CertificatesExist(ctx) {
		return
	}

	info, err := m.installDNS01Plugin(ctx)
	if err != nil {
		m.log.Warnf("[plugin:%s] automatic install: %v", OfficialDNS01PluginID, err)
		notification.Error("DNS-01 plugin unavailable",
			"Certificates using the DNS-01 challenge cannot be renewed until the DNS-01 plugin is installed.",
			map[string]any{"plugin_id": OfficialDNS01PluginID, "error": err.Error()})
		return
	}

	m.log.Infof("[plugin:%s] installed %s automatically for DNS-01 certificates", info.ID, info.Version)
}

// installDNS01Plugin prefers a package the operator already placed on the
// node, then falls back to the marketplace. Either way only a package signed
// with an official plugin key is installed without the user asking.
func (m *Manager) installDNS01Plugin(ctx context.Context) (*Info, error) {
	opts := InstallOptions{Enable: true, ApprovePermissions: true, MinTrust: TrustOfficial}

	info, err := m.InstallLocalPackage(ctx, OfficialDNS01PluginID, opts)
	if err == nil {
		return info, nil
	}
	if err != ErrLocalPackageNotFound {
		m.log.Warnf("[plugin:%s] local package: %v", OfficialDNS01PluginID, err)
	}

	if !settings.PluginSettings.MarketplaceEnabled {
		return nil, ErrMarketplaceDisabled
	}
	marketplace := m.Marketplace()
	entries, err := marketplace.Catalog(ctx, false)
	if err != nil {
		return nil, err
	}
	entry := findEntry(entries, OfficialDNS01PluginID, "")
	if entry == nil {
		return nil, ErrMarketplaceNotFound
	}
	// A pre filter on the catalog claim, MinTrust checks the signature.
	if entry.Trust != TrustOfficial {
		return nil, ErrCommunityNotAllowed
	}
	return marketplace.Install(ctx, OfficialDNS01PluginID, "", entry.Source, opts)
}

// seedSettings fills the settings a plugin takes over from the core when its
// row is first created on a node, whichever way the package arrived: the
// official DNS-01 plugin inherits the deprecated core resolver list, which it
// now uses for the propagation check.
func seedSettings(manifest *protocol.Manifest, row *model.Plugin) {
	if manifest.ID != OfficialDNS01PluginID {
		return
	}
	nameservers := settings.CertSettings.RecursiveNameservers
	if len(nameservers) == 0 {
		return
	}
	if row.Settings == nil {
		row.Settings = map[string]any{}
	}
	row.Settings[recursiveNameserversKey] = strings.Join(nameservers, ",")
}

// repairIncompatiblePlugins upgrades the vetted plugins a core upgrade left
// behind, so a node does not lose a capability after an update. The new
// package has to be signed with an official plugin key.
func (m *Manager) repairIncompatiblePlugins(ctx context.Context) {
	if !settings.PluginSettings.MarketplaceEnabled {
		return
	}

	broken := make([]string, 0, 2)
	for _, info := range m.List() {
		if info.Status == StatusIncompatible {
			broken = append(broken, info.ID)
		}
	}
	if len(broken) == 0 {
		return
	}

	marketplace := m.Marketplace()
	entries, err := marketplace.Catalog(ctx, false)
	if err != nil {
		m.log.Warnf("Repair incompatible plugins: %v", err)
		return
	}

	for _, id := range broken {
		entry := findEntry(entries, id, "")
		if entry == nil || entry.Trust != TrustOfficial || entry.InstallableRelease == nil {
			continue
		}
		if _, err = marketplace.update(ctx, id, entry.InstallableRelease.Version, true, TrustOfficial); err != nil {
			m.log.Warnf("[plugin:%s] repair after core upgrade: %v", id, err)
			continue
		}
		m.log.Infof("[plugin:%s] upgraded to %s after a core upgrade", id, entry.InstallableRelease.Version)
	}
}

// isUsable reports whether a plugin is installed with its files in place.
func (m *Manager) isUsable(id string) bool {
	info, err := m.Get(id)
	if err != nil {
		return false
	}
	return info.Status != StatusMissing && info.Status != StatusIncompatible
}

// dns01CertificatesExist reports whether any certificate still needs DNS-01.
func dns01CertificatesExist(ctx context.Context) bool {
	if query.Cert == nil {
		return false
	}
	count, err := query.Cert.WithContext(ctx).
		Where(query.Cert.ChallengeMethod.Eq(model.CertChallengeMethodDNS01)).
		Count()
	if err != nil {
		return false
	}
	return count > 0
}
