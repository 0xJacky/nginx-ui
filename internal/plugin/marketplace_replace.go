package plugin

import "context"

// Replace installs the marketplace package of an installed plugin over the
// installed one, even at the same version. It is meant to swap an unsigned or
// community copy for the official or partner package, so the catalog entry
// must claim a higher trust than the installed package and the downloaded
// package must prove it. Settings, data and the enabled state are kept, and
// a changed permission set still waits for approval.
func (mp *Marketplace) Replace(ctx context.Context, id, source string) (*Info, error) {
	current, err := mp.manager.Get(id)
	if err != nil {
		return nil, err
	}

	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return nil, err
	}
	entry := findEntry(entries, id, source)
	if entry == nil {
		return nil, ErrMarketplaceNotFound
	}

	target := effectiveTrust(entry.ID, entry.Trust)
	if !replaceable(current.Trust, target) {
		return nil, ErrReplaceUnavailable
	}

	return mp.Install(ctx, id, "", entry.Source, InstallOptions{
		Enable:   current.Enabled,
		MinTrust: target,
	})
}

// replaceable reports whether a package of trust target may replace one of
// trust installed: only official or partner packages, and only upwards.
func replaceable(installed, target string) bool {
	if target != TrustOfficial && target != TrustVerified {
		return false
	}
	return trustRank(installed) < trustRank(target)
}
