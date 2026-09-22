import type { ExternalNotifierChannel } from '@/api/external_notify'
import { listExternalNotifierChannels } from '@/api/external_notify'
import configMap from './index'

// Notifier types enabled plugins offer next to the generated built-in configs.
// Their forms come from the schema each plugin declares.
export const pluginChannels = ref<ExternalNotifierChannel[]>([])

let pending: Promise<void> | undefined

/** Loads the plugin channels once; pass force to refresh them. */
export function loadPluginChannels(force = false): Promise<void> {
  if (pending && !force)
    return pending

  pending = listExternalNotifierChannels()
    .then(res => {
      pluginChannels.value = res.data ?? []
    })
    .catch(error => {
      console.error('Failed to load plugin notification channels:', error)
      pending = undefined
    })
  return pending
}

export function findPluginChannel(type?: string): ExternalNotifierChannel | undefined {
  if (!type)
    return undefined
  return pluginChannels.value.find(channel => channel.type === type)
}

/** The config keys a notifier type stores, for a built-in or a plugin type. */
export function configKeysOf(type?: string): string[] {
  const builtin = configMap[type?.toLowerCase() ?? '']
  if (builtin)
    return builtin.config.map(item => item.key)
  return findPluginChannel(type)?.fields.map(field => field.key) ?? []
}

/** Drops the config values the notifier type does not declare. */
export function sanitizeConfig(type: string | undefined, config: Record<string, string> | undefined): Record<string, string> {
  const allowed = new Set(configKeysOf(type))
  return Object.fromEntries(Object.entries(config ?? {}).filter(([key]) => allowed.has(key)))
}
