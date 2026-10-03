import type { RouteLocationRaw } from 'vue-router'

/** The official plugin that took over the advanced log indexing. */
export const LOG_ANALYTICS_PLUGIN_ID = 'com.nginxui.log-analytics'

/**
 * Whether the node still needs the log analytics plugin: it used the indexing
 * built into earlier releases and the plugin is not installed. A list that
 * could not be loaded (`null`) is no proof of an installation.
 */
export function needsLogAnalyticsPlugin(legacyEnabled: boolean, installedIds: string[] | null): boolean {
  return legacyEnabled && !(installedIds ?? []).includes(LOG_ANALYTICS_PLUGIN_ID)
}

/** The plugin page, focused on the log analytics entry of the catalog. */
export function logAnalyticsInstallRoute(): RouteLocationRaw {
  return {
    path: '/system/plugins',
    query: { tab: 'marketplace', catalog: LOG_ANALYTICS_PLUGIN_ID },
  }
}
