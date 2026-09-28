import type { InjectionKey, Ref } from 'vue'

/**
 * Provided by the editor of an existing site. It lets the Upstream section
 * convert one of the site's upstream blocks into a shared upstream group,
 * which rewrites the site file on the server: the editor has to be in sync
 * with the file before, and reload it after.
 */
export interface SiteUpstreamContext {
  /** Name of the site file in sites-available. */
  siteName: Ref<string>
  /** Whether the editor holds changes that are not saved to the file yet. */
  hasUnsavedChanges: () => Promise<boolean>
  /** Saves the editor content to the file. Rejects when the save fails. */
  save: () => Promise<void>
  /** Drops the unsaved changes and reloads the file. */
  reload: () => Promise<void>
}

export const siteUpstreamContextKey: InjectionKey<SiteUpstreamContext> = Symbol('siteUpstreamContext')
