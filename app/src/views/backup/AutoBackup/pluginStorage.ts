import type { StorageBackend } from '@/api/backup'
import { listStorageBackends } from '@/api/backup'

// Storage types enabled plugins offer next to the built-in local and S3
// storage. Their forms come from the schema each plugin declares.
export const pluginBackends = ref<StorageBackend[]>([])

let pending: Promise<void> | undefined

/** Loads the plugin backends once; pass force to refresh them. */
export function loadPluginBackends(force = false): Promise<void> {
  if (pending && !force)
    return pending

  pending = listStorageBackends()
    .then(res => {
      pluginBackends.value = (res.data ?? []).filter(backend => !backend.builtin)
    })
    .catch(error => {
      console.error('Failed to load plugin storage backends:', error)
      pending = undefined
    })
  return pending
}

/** Reports whether a storage type is provided by a plugin. */
export function isPluginStorageType(type?: string): boolean {
  return !!type && type.startsWith('plugin:')
}

export function findPluginBackend(type?: string): StorageBackend | undefined {
  if (!type)
    return undefined
  return pluginBackends.value.find(backend => backend.type === type)
}

/** Display name of a storage type, falling back to the stored value. */
export function storageTypeLabel(type?: string): string {
  switch (type) {
    case 'local':
      return $gettext('Local')
    case 's3':
      return $gettext('S3')
    default:
      return findPluginBackend(type)?.name ?? type ?? ''
  }
}

/** Drops the values the backend form does not declare. */
export function sanitizeStorageConfig(type: string | undefined, config: Record<string, string> | undefined): Record<string, string> {
  const backend = findPluginBackend(type)
  if (!backend)
    return { ...config }
  const allowed = new Set(backend.fields.map(field => field.key))
  return Object.fromEntries(Object.entries(config ?? {}).filter(([key]) => allowed.has(key)))
}

/** Formats a byte count for people. */
export function formatSize(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${unit === 0 ? value : value.toFixed(1)} ${units[unit]}`
}
