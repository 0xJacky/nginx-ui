import type { PluginTrust } from '@/api/plugin_marketplace'

interface TrustPreset {
  color: string
  label: () => string
  hint: () => string
}

/**
 * Trust level of a catalog entry. It decides the badge colour and the warning
 * the install dialog shows, so it lives next to both of them.
 */
const trustPresets: Record<PluginTrust, TrustPreset> = {
  official: {
    color: 'blue',
    label: () => $gettext('Official'),
    hint: () => $gettext('Built and signed by the Nginx UI maintainers.'),
  },
  verified: {
    color: 'green',
    label: () => $gettext('Verified'),
    hint: () => $gettext('Reviewed by the Nginx UI maintainers and signed with a release key.'),
  },
  community: {
    color: 'orange',
    label: () => $gettext('Community'),
    hint: () => $gettext('Published by a third party and not reviewed. It runs with the permissions you grant it, so only install it if you trust the author.'),
  },
}

export function trustPreset(trust?: PluginTrust): TrustPreset {
  return trustPresets[trust ?? 'community'] ?? trustPresets.community
}

export function isCommunityTrust(trust?: PluginTrust): boolean {
  return (trust ?? 'community') === 'community'
}
