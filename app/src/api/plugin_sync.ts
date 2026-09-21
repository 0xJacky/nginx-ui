import type { PluginInfo, PluginStatus, PluginSyncPolicy } from '@/api/plugin'
import { http } from '@uozi-admin/request'

/** State of one plugin on one node. */
export type PluginSyncState
  = | 'in_sync'
    | 'outdated'
    | 'missing'
    | 'unsupported_platform'
    | 'unsupported'
    | 'offline'
    | 'opted_out'
    | 'error'

/** One column of the cluster matrix. */
export interface PluginMatrixNode {
  id: number
  name: string
  url: string
  online: boolean
  accept_plugin_sync: boolean
  os?: string
  arch?: string
}

/** State of one plugin on one node. */
export interface PluginMatrixCell {
  node_id: number
  version?: string
  enabled: boolean
  status?: PluginStatus
  state: PluginSyncState
  message?: string
}

/** One plugin together with its cluster sync intent. */
export interface PluginMatrixRow {
  plugin_id: string
  name: string
  version: string
  enabled: boolean
  api_version: number
  sync_policy: PluginSyncPolicy
  sync_node_ids: number[]
  sync_settings: boolean
  cells: PluginMatrixCell[]
}

/** The plugin inventory of the whole cluster. */
export interface PluginMatrix {
  nodes: PluginMatrixNode[]
  rows: PluginMatrixRow[]
}

/** What one sync run did on one node. */
export interface PluginNodeResult {
  node_id: number
  node: string
  plugin_id: string
  success: boolean
  state: PluginSyncState
  actions: string[]
  version?: string
  error?: string
}

/** Payload of the sync policy endpoint. */
export interface PluginSyncPolicyPayload {
  sync_policy: PluginSyncPolicy
  /** Empty means every child node, including the ones added later. */
  sync_node_ids: number[]
  sync_settings: boolean
}

function pluginPath(id: string, suffix = '') {
  return `/plugins/${encodeURIComponent(id)}${suffix}`
}

/** Every installed plugin against every enabled node. */
export function getMatrix(): Promise<PluginMatrix> {
  return http.get('/plugins/matrix')
}

/** Pushes one plugin to the given nodes, or to the configured target set. */
export function syncPlugin(id: string, nodeIds?: number[]): Promise<{ results: PluginNodeResult[] }> {
  return http.post(pluginPath(id, '/sync'), { node_ids: nodeIds ?? [] })
}

/** Stores whether a plugin is kept in sync automatically. */
export function setSyncPolicy(id: string, payload: PluginSyncPolicyPayload): Promise<PluginInfo> {
  return http.post(pluginPath(id, '/sync_policy'), payload)
}
