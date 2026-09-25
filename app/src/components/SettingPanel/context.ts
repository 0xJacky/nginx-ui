import type { InjectionKey, Ref } from 'vue'

export interface SettingRowContext {
  // Dot path of the row the search box asked to reveal.
  highlightedPath: Ref<string | undefined>
  // Dot paths whose value differs from the loaded snapshot.
  changedPaths: Ref<Set<string>>
}

export const SETTING_ROW_CONTEXT: InjectionKey<SettingRowContext> = Symbol('setting-row-context')
