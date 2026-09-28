import type { RouteLocationNormalizedLoaded } from 'vue-router'

/** Why a node cannot be switched to right now. */
export type NodeSwitchBlocker = 'offline' | 'version_unknown' | 'version_mismatch'

export interface SwitchableNode {
  id?: number
  name?: string
  url?: string
  status?: boolean
  version?: string
}

export type NodeVersionState = 'compatible' | 'mismatch' | 'unknown'

/** Remote UIs must run exactly the local version: mismatched APIs break the proxied pages. */
export function getNodeVersionState(node: SwitchableNode, localVersion: string): NodeVersionState {
  if (!node.version || !localVersion)
    return 'unknown'

  return node.version === localVersion ? 'compatible' : 'mismatch'
}

/** Mirrors the dashboard Link button: a node must be online and version-compatible. */
export function getNodeSwitchBlocker(node: SwitchableNode, localVersion: string): NodeSwitchBlocker | null {
  if (!node.status)
    return 'offline'

  const versionState = getNodeVersionState(node, localVersion)
  if (versionState === 'unknown')
    return 'version_unknown'

  if (versionState === 'mismatch')
    return 'version_mismatch'

  return null
}

/** Online nodes first, then by name. */
export function sortSwitchNodes<T extends SwitchableNode>(nodes: T[]): T[] {
  return [...nodes].sort((a, b) => {
    if (!!a.status !== !!b.status)
      return a.status ? -1 : 1

    return (a.name ?? '').localeCompare(b.name ?? '')
  })
}

/** Case-insensitive match against the node name and URL. */
export function filterSwitchNodes<T extends SwitchableNode>(nodes: T[], keyword: string): T[] {
  const query = keyword.trim().toLowerCase()
  if (!query)
    return nodes

  return nodes.filter(node => (node.name ?? '').toLowerCase().includes(query)
    || (node.url ?? '').toLowerCase().includes(query))
}

type LandingRoute = Pick<RouteLocationNormalizedLoaded, 'path' | 'params' | 'matched'>

/**
 * Decides where to land after switching nodes. Pages addressing a specific
 * resource (route params) rarely exist on the other node, and some pages are
 * hidden for remote nodes, so both fall back to the dashboard.
 *
 * Call it after the node has changed: `hiddenInSidebar` reads the current node.
 */
export function resolveSwitchLanding(route: LandingRoute, fallback = '/dashboard'): string {
  const hasParams = Object.values(route.params)
    .some(value => Array.isArray(value) ? value.length > 0 : !!value)
  if (hasParams)
    return fallback

  const isHidden = route.matched.some(record => {
    const hidden = record.meta?.hiddenInSidebar
    return typeof hidden === 'function' && hidden()
  })
  if (isHidden)
    return fallback

  return route.path
}
