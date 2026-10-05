import type { Translate } from '@nginxui/plugin-market-ui'
import type { CatalogEntry, PluginTrust } from '@/api/plugin_marketplace'
import { trustText } from '@nginxui/plugin-market-ui'

interface TrustPreset {
  /** Tag colour. */
  color: string
  /** Pill class of the detail views, empty for the plain grey pill. */
  tone: string
  label: () => string
  hint: () => string
}

const t: Translate = (msgid, params) => $gettext(msgid, params)

const colors: Record<PluginTrust, string> = {
  official: 'blue',
  verified: 'green',
  community: 'orange',
  unsigned: 'default',
}

/**
 * Trust level of a catalog entry or an installed package. It decides the badge
 * colour and the warning the install dialogs show, so it lives next to both.
 * The wording is the marketplace's, shared with the developer portal.
 */
function presetOf(trust: PluginTrust): TrustPreset {
  return {
    color: colors[trust],
    tone: trustText(t, trust).tone,
    label: () => trustText(t, trust).label,
    hint: () => trustText(t, trust).hint,
  }
}

const trustPresets: Record<PluginTrust, TrustPreset> = {
  official: presetOf('official'),
  verified: presetOf('verified'),
  community: presetOf('community'),
  unsigned: presetOf('unsigned'),
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

/** What the Unsigned tag means, in plain words. */
export function unsignedExplanation(): string {
  return $gettext('This version was not published by the official team or a partner. It may be a development build or a repackaged copy.')
}

/** A more trusted package of an installed plugin the marketplace offers. */
export interface TrustedOffer {
  entry: CatalogEntry
  trust: 'official' | 'verified'
  version: string
}

/**
 * The marketplace entry that can replace an installed package: it must be
 * official or from a partner while the installed one is unsigned or community.
 */
export function findTrustedOffer(installed: PluginTrust | undefined, entry: CatalogEntry | undefined): TrustedOffer | undefined {
  if (!entry?.installable_release)
    return undefined
  if (installed !== 'unsigned' && installed !== 'community')
    return undefined
  if (entry.trust !== 'official' && entry.trust !== 'verified')
    return undefined
  return { entry, trust: entry.trust, version: entry.installable_release.version }
}

/** Button text of an offer. */
export function trustedOfferAction(offer: TrustedOffer): string {
  return offer.trust === 'official'
    ? $gettext('Install the official version')
    : $gettext('Install the partner version')
}

/** One line telling what the marketplace has instead. */
export function trustedOfferSummary(offer: TrustedOffer): string {
  return offer.trust === 'official'
    ? $gettext('The marketplace has the official version %{version}.', { version: offer.version })
    : $gettext('The marketplace has the partner version %{version}.', { version: offer.version })
}
