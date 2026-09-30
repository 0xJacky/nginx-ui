import type { PluginChannel, PluginInfo, PluginManifest } from '@/api/plugin'
import { http } from '@uozi-admin/request'
import { localizedText } from '@/api/plugin'

/**
 * How much the node trusts the publisher of a catalog entry or a package.
 * 'unsigned' marks a package that carries no signature.
 */
export type PluginTrust = 'official' | 'verified' | 'community' | 'unsigned'

/** Who signed a release: the nginx-ui release key or the plugin author. */
export type PluginSignedBy = 'official' | 'author'

/** Phases of a marketplace install, as published on the event bus. */
export type PluginInstallStatus = 'downloading' | 'verifying' | 'installing' | 'done' | 'error'

/** Websocket event carrying the install progress. */
export const PLUGIN_INSTALL_PROGRESS_EVENT = 'plugin_install_progress'

/** Key of a platform independent package in `CatalogRelease.downloads`. */
export const ANY_PLATFORM = 'any'

/** One package file of a release. */
export interface CatalogDownload {
  url: string
  sha256?: string
  /** Defaults to `<url>.minisig`. */
  signature_url?: string
}

/** One downloadable version of a catalog entry. */
export interface CatalogRelease {
  version: string
  released_at?: string
  api_version: number
  min_nginx_ui_version?: string
  /**
   * Summary of where the release installs: the keys of `downloads` plus the
   * platforms the portable package covers. Empty means the portable package
   * runs everywhere.
   */
  platforms?: string[]
  /** Per-platform packages keyed by "<goos>-<goarch>" or "any". */
  downloads?: Record<string, CatalogDownload>
  /** Portable package, the fallback for platforms `downloads` does not name. */
  download_url: string
  sha256?: string
  signature_url?: string
  signed_by?: PluginSignedBy
  release_notes_url?: string
  yanked?: boolean
  /** Channel of the release, filled from the version when the catalog names none. */
  channel?: PluginChannel
  /** Snapshot of the plugin.json this release ships. */
  manifest?: PluginManifest
}

/** One plugin as the catalog describes it, plus the state of this node. */
export interface CatalogEntry {
  id: string
  /** Locale code to display name, "en" is the fallback. */
  name?: Record<string, string>
  description?: Record<string, string>
  author?: string
  author_public_key?: string
  homepage_url?: string
  repository_url?: string
  readme_url?: string
  icon_url?: string
  categories?: string[]
  capabilities?: string[]
  license?: string
  trust?: PluginTrust
  stage?: string
  /** Channel of the release this node would install, at least beta in the beta stage. */
  channel?: PluginChannel
  releases: CatalogRelease[]
  /** Catalog URL this entry was merged from. */
  source: string
  /** Newest release this node can actually install. */
  installable_release?: CatalogRelease
  /** Versions this node can install, newest first, without withdrawn ones. */
  installable_versions?: string[]
  installed_version?: string
  update_available: boolean
}

/** One installed plugin the catalog offers a newer release for. */
export interface PluginUpdateInfo {
  id: string
  name: string
  installed_version: string
  latest_version: string
  source: string
  trust?: PluginTrust
  /** True when the catalog withdrew the version this node runs. */
  installed_yanked: boolean
  permissions_changed: boolean
  release?: CatalogRelease
}

export interface MarketplaceListResponse {
  plugins: CatalogEntry[]
  sources: string[]
  /** "<goos>-<goarch>" of this node, what installable_release was picked for. */
  host_platform?: string
}

export interface MarketplaceDetailResponse {
  plugin: CatalogEntry
  readme: string
  host_platform?: string
}

export interface MarketplaceSourcesResponse {
  sources: string[]
  default: string
}

export interface MarketplaceListParams {
  keyword?: string
  category?: string
  source?: string
  /** Bypasses the one hour catalog cache on the server. */
  refresh?: boolean
}

export interface MarketplaceInstallPayload {
  id: string
  version?: string
  source?: string
  enable?: boolean
  approve_permissions?: boolean
  /** Turns off the enabled plugins that cannot be on together with the package. */
  replace_conflicts?: boolean
}

export interface PluginUpdatePayload {
  version?: string
  approve_permissions?: boolean
}

/** Payload of the PLUGIN_INSTALL_PROGRESS_EVENT websocket event. */
export interface PluginInstallProgress {
  plugin_id: string
  status: PluginInstallStatus
  progress: number
  message?: string
  /** Downloads key of the package being installed, empty for the portable one. */
  platform?: string
}

function pluginPath(id: string, suffix = '') {
  return `/plugins/${encodeURIComponent(id)}${suffix}`
}

/** The merged catalog of every configured source. */
export function getMarketplaceList(params: MarketplaceListParams = {}): Promise<MarketplaceListResponse> {
  return http.get('/plugins/marketplace', {
    params: {
      keyword: params.keyword || undefined,
      category: params.category || undefined,
      source: params.source || undefined,
      refresh: params.refresh ? 'true' : undefined,
    },
  })
}

/** One catalog entry together with its readme. */
export function getMarketplacePlugin(id: string, source?: string): Promise<MarketplaceDetailResponse> {
  return http.get(`/plugins/marketplace/${encodeURIComponent(id)}`, {
    params: { source: source || undefined },
  })
}

/** Downloads, verifies and installs one catalog release. */
export function installFromMarketplace(payload: MarketplaceInstallPayload): Promise<PluginInfo> {
  return http.post('/plugins/marketplace/install', payload)
}

/** Installed plugins with a newer release in the catalog. */
export function getPluginUpdates(): Promise<PluginUpdateInfo[]> {
  return http.get('/plugins/updates')
}

/** Upgrades one installed plugin, keeping its settings and enabled state. */
export function updatePlugin(id: string, payload: PluginUpdatePayload = {}): Promise<PluginInfo> {
  return http.post(pluginPath(id, '/update'), payload)
}

/**
 * Replaces an installed plugin with the more trusted package the marketplace
 * offers, even at the same version. Settings, data and the enabled state stay.
 */
export function replacePlugin(id: string, source?: string): Promise<PluginInfo> {
  return http.post(pluginPath(id, '/replace'), { source })
}

export function getMarketplaceSources(): Promise<MarketplaceSourcesResponse> {
  return http.get('/plugins/marketplace/sources')
}

export function saveMarketplaceSources(sources: string[]): Promise<MarketplaceSourcesResponse> {
  return http.post('/plugins/marketplace/sources', { sources })
}

/**
 * Platforms a release can be installed on: the keys of `downloads` that carry
 * a url, falling back to the `platforms` summary. An empty list means any.
 */
export function releasePlatforms(release: CatalogRelease): string[] {
  const platforms = new Set(
    Object.entries(release.downloads ?? {})
      .filter(([, download]) => Boolean(download?.url))
      .map(([platform]) => platform),
  )
  if (platforms.size === 0)
    return release.platforms ?? []

  // The portable package still serves whatever else the summary lists.
  if (release.download_url) {
    for (const platform of release.platforms ?? [])
      platforms.add(platform)
  }
  return [...platforms].sort()
}

/** Display name of a catalog entry in the active language, English fallback. */
export function catalogEntryName(entry: CatalogEntry, language: string): string {
  return localizedText(entry.name, language) || entry.id
}

/** Description of a catalog entry in the active language, English fallback. */
export function catalogEntryDescription(entry: CatalogEntry, language: string): string {
  return localizedText(entry.description, language)
}
