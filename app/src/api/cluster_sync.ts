/** Content kinds replicated by a cluster synchronization run. */
export type SyncKind = 'config' | 'site' | 'stream' | 'namespace' | 'certificate'

/** Outcome of a single item on a single node. */
export interface SyncResult {
  node_id: number
  node: string
  kind: SyncKind
  name: string
  success: boolean
  error?: string
  /** Files the node left alone because they exist there and overwrite was off. */
  skipped_existing?: number
  skipped_paths?: string[]
}

/** Why a local file was left out of a synchronization run. */
export type SyncSkipReason = 'unsupported_type' | 'too_large' | 'not_text' | 'unreadable' | 'entry_config'

/** A local file that was not replicated. */
export interface SyncSkippedFile {
  path: string
  reason: SyncSkipReason
}

/** Aggregated outcome of one synchronization run. */
export interface SyncSummary {
  total: number
  succeeded: number
  failed: number
  results: SyncResult[]
  skipped?: SyncSkippedFile[]
}

/** Selects which content a node synchronization replicates. */
export interface SyncScope {
  configs?: boolean
  sites?: boolean
  streams?: boolean
  overwrite?: boolean
}
