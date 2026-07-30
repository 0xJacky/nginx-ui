/** Content kinds replicated by a cluster synchronization run. */
export type SyncKind = 'config' | 'site' | 'stream' | 'namespace'

/** Outcome of a single item on a single node. */
export interface SyncResult {
  node_id: number
  node: string
  kind: SyncKind
  name: string
  success: boolean
  error?: string
}

/** Aggregated outcome of one synchronization run. */
export interface SyncSummary {
  total: number
  succeeded: number
  failed: number
  results: SyncResult[]
}

/** Selects which content a node synchronization replicates. */
export interface SyncScope {
  configs?: boolean
  sites?: boolean
  streams?: boolean
  overwrite?: boolean
}
