import type { PluginTrust } from '@/api/plugin_marketplace'

interface TrustPreset {
  /** Tag colour. */
  color: string
  /** Pill class of the detail views, empty for the plain grey pill. */
  tone: string
  label: () => string
  hint: () => string
}

/**
 * Trust level of a catalog entry or an installed package. It decides the badge
 * colour and the warning the install dialogs show, so it lives next to both.
 */
const trustPresets: Record<PluginTrust, TrustPreset> = {
  official: {
    color: 'blue',
    tone: 'is-accent',
    label: () => $gettext('Official'),
    hint: () => $gettext('Published by the Nginx UI project.'),
  },
  verified: {
    color: 'green',
    tone: 'is-success',
    label: () => $gettext('Verified'),
    hint: () => $gettext('Published by a partner of the Nginx UI project.'),
  },
  community: {
    color: 'orange',
    tone: 'is-warning',
    label: () => $gettext('Community'),
    hint: () => $gettext('Published by a third party and not reviewed. It runs with the permissions you grant it, so only install it if you trust the author.'),
  },
  unsigned: {
    color: 'default',
    tone: '',
    label: () => $gettext('Unsigned'),
    hint: () => $gettext('Not signed, so its publisher cannot be confirmed. Installing it requires developer mode.'),
  },
}

/** Preset of a catalog entry, where a missing level counts as community. */
export function trustPreset(trust?: PluginTrust): TrustPreset {
  return trustPresets[trust ?? 'community'] ?? trustPresets.community
}

/** Preset of an installed or uploaded package, none when the node reports no level. */
export function packageTrustPreset(trust?: PluginTrust): TrustPreset | undefined {
  return trust ? trustPresets[trust] : undefined
}

export function isCommunityTrust(trust?: PluginTrust): boolean {
  return (trust ?? 'community') === 'community'
}

export function isUnsignedTrust(trust?: PluginTrust): boolean {
  return trust === 'unsigned'
}

/** Order of the levels, a higher rank is trusted more. */
const trustRank: Record<PluginTrust, number> = {
  unsigned: 0,
  community: 1,
  verified: 2,
  official: 3,
}

/** Whether the next package ranks below the installed one, false when either level is unknown. */
export function isTrustDowngrade(next?: PluginTrust, installed?: PluginTrust): boolean {
  if (!next || !installed || !(next in trustRank) || !(installed in trustRank))
    return false
  return trustRank[next] < trustRank[installed]
}
