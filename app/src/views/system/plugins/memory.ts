import type { PluginManifest, PluginSpec } from '@/api/plugin'
import pluginApi from '@/api/plugin'

/** Formats a size in MiB, switching to GB from one GiB up. */
export function formatMemory(mb: number): string {
  if (mb >= 1024) {
    const gb = mb / 1024
    return `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB`
  }
  return `${mb} MB`
}

/** The memory hint of a manifest in MiB, 0 when it gives none. */
export function recommendedMemory(manifest?: Pick<PluginManifest, 'server'>): number {
  const value = manifest?.server?.resources?.recommended_memory_mb ?? 0
  return value > 0 ? value : 0
}

/** True only when both numbers are known and the server has less than advised. */
export function isBelowRecommended(recommendedMb: number | undefined, systemMb: number | undefined): boolean {
  return (recommendedMb ?? 0) > 0 && (systemMb ?? 0) > 0 && systemMb! < recommendedMb!
}

/** The warning shown when the server has less memory than advised. */
export function memoryWarning(recommendedMb: number, systemMb: number): string {
  return $gettext(
    'This server has %{system} of memory. This plugin is recommended for %{recommended} or more and may run slowly or stop while it works.',
    { system: formatMemory(systemMb), recommended: formatMemory(recommendedMb) },
  )
}

let specRequest: Promise<PluginSpec> | undefined

/** Memory of this server in MiB, 0 while unknown. The spec is fetched once. */
export function useSystemMemory() {
  const systemMemory = ref(0)
  specRequest ??= pluginApi.getSpec()
  specRequest
    .then(spec => {
      systemMemory.value = spec.system_memory_mb ?? 0
    })
    .catch(() => {
      // Only a hint depends on it, so a failed lookup shows no warning.
      specRequest = undefined
    })
  return systemMemory
}
