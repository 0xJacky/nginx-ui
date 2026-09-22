import type { BadgeProps } from 'antdv-next'
import type { PluginInfo, PluginStatus } from '@/api/plugin'

export interface StatusPreset {
  badge: BadgeProps['status']
  label: () => string
}

/** Badge colour and wording of every status the backend reports. */
export const statusPresets: Record<PluginStatus, StatusPreset> = {
  installed: { badge: 'default', label: () => $gettext('Installed') },
  starting: { badge: 'processing', label: () => $gettext('Starting') },
  running: { badge: 'success', label: () => $gettext('Running') },
  stopped: { badge: 'default', label: () => $gettext('Stopped') },
  error: { badge: 'error', label: () => $gettext('Error') },
  missing: { badge: 'error', label: () => $gettext('Missing') },
  incompatible: { badge: 'error', label: () => $gettext('Incompatible') },
  needs_approval: { badge: 'warning', label: () => $gettext('Needs approval') },
}

export function statusOf(plugin: PluginInfo): StatusPreset {
  // An on demand plugin is stopped most of the time by design, so its idle
  // state must not read like a problem.
  if (plugin.enabled && plugin.lifecycle === 'on_demand' && plugin.status === 'stopped')
    return { badge: 'default', label: () => $gettext('Idle') }

  return statusPresets[plugin.status] ?? { badge: 'default', label: () => plugin.status }
}

/** Statuses the operator has to act on before the plugin works. */
const attentionStatuses = new Set<PluginStatus>(['error', 'missing', 'incompatible', 'needs_approval'])

export function needsAttention(plugin: PluginInfo) {
  return attentionStatuses.has(plugin.status)
}

/** A plugin the host cannot run at all must not offer a toggle. */
export function isToggleDisabled(plugin: PluginInfo) {
  return plugin.status === 'incompatible' || plugin.status === 'missing'
}

export type InstalledFilter = 'all' | 'enabled' | 'disabled' | 'attention'

export function matchesFilter(plugin: PluginInfo, filter: InstalledFilter) {
  switch (filter) {
    case 'enabled':
      return plugin.enabled
    case 'disabled':
      return !plugin.enabled
    case 'attention':
      return needsAttention(plugin)
    default:
      return true
  }
}

/** Case-insensitive match on the fields a user is likely to remember. */
export function matchesKeyword(plugin: PluginInfo, keyword: string) {
  const needle = keyword.trim().toLowerCase()
  if (!needle)
    return true

  const haystack = [plugin.name, plugin.id, plugin.description ?? '', ...(plugin.capabilities ?? [])]
  return haystack.some(value => value.toLowerCase().includes(needle))
}

export type PluginDrawerTab = 'overview' | 'settings' | 'logs'
