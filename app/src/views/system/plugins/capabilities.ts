import type { Translate } from '@nginxui/plugin-market-ui'
import { capabilityText } from '@nginxui/plugin-market-ui'

export interface CapabilityPreset {
  label: () => string
  /** Tabler icon class shown next to the label on the plugin cards. */
  icon: string
  description: () => string
  /** Host page where the feature is used, if there is one. */
  link?: {
    to: string
    label: () => string
  }
}

/**
 * Icons and host pages of the capabilities the host knows. Their wording is
 * the marketplace's, shared with the developer portal, so the page never
 * shows the raw capability ids.
 */
const capabilityExtras: Record<string, Pick<CapabilityPreset, 'icon' | 'link'>> = {
  'dns01': { icon: 'i-tabler-world-www', link: { to: '/dns/credentials', label: () => $gettext('DNS credentials') } },
  'http': { icon: 'i-tabler-browser' },
  'notify': { icon: 'i-tabler-bell' },
  'probe': { icon: 'i-tabler-heartbeat' },
  'mcp': { icon: 'i-tabler-robot' },
  'storage': { icon: 'i-tabler-database', link: { to: '/backup/auto-backup', label: () => $gettext('Auto backup') } },
  'cert.deploy': { icon: 'i-tabler-certificate', link: { to: '/certificates/deploy_targets', label: () => $gettext('Deploy targets') } },
  'security.blocklist': { icon: 'i-tabler-ban', link: { to: '/blocklists', label: () => $gettext('Blocklists') } },
  'upstream.discovery': { icon: 'i-tabler-radar' },
  'log.sink': { icon: 'i-tabler-file-analytics' },
}

const t: Translate = (msgid, params) => $gettext(msgid, params)

/** Preset of a capability, a generic one for a capability the host does not know. */
export function capabilityPreset(capability: string): CapabilityPreset {
  const extras = capabilityExtras[capability] ?? { icon: 'i-tabler-plug' }
  return {
    ...extras,
    label: () => capabilityText(t, capability).label,
    description: () => capabilityText(t, capability).description,
  }
}

export function capabilityLabel(capability: string): string {
  return capabilityPreset(capability).label()
}

export function capabilityIcon(capability: string): string {
  return capabilityPreset(capability).icon
}
