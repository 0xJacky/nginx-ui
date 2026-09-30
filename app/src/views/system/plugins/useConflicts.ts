import type { MaybeRefOrGetter } from 'vue'
import type { PluginInfo } from '@/api/plugin'
import { localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import { conflictingIds, conflictNames, enabledConflicts } from './conflicts'
import { useInstalledPlugins } from './inventory'

function nameOf(plugin: PluginInfo) {
  return localizedPluginName(plugin, gettext.current)
}

/** Display names of the installed plugins that cannot be on together with this one. */
export function useConflictNames(plugin: MaybeRefOrGetter<Pick<PluginInfo, 'id' | 'conflicts'> | undefined>) {
  const installed = useInstalledPlugins()
  return computed(() => {
    const subject = toValue(plugin)
    if (!subject)
      return []
    return conflictNames(conflictingIds(subject, installed.value), installed.value, nameOf)
  })
}

/**
 * Names of the enabled plugins a package would turn off when it is turned on.
 * The lookup only runs while `active` is true.
 */
export function usePackageConflicts(
  manifest: MaybeRefOrGetter<{ id: string, conflicts?: string[] } | undefined>,
  active: MaybeRefOrGetter<boolean>,
) {
  const installed = useInstalledPlugins(active)
  return computed(() => {
    const subject = toValue(manifest)
    if (!subject || !toValue(active))
      return []
    return enabledConflicts(subject, installed.value).map(nameOf)
  })
}

/** Display names of enabled plugins, for the plugin that is about to be turned on. */
export function enabledConflictNames(plugin: PluginInfo, installed: PluginInfo[]): string[] {
  return conflictNames(plugin.conflicts_enabled ?? [], installed, nameOf)
}
