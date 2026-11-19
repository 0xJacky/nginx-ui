import { http } from '@uozi-admin/request'

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

/** WebSocket event carrying the progress and outcome of a background sync. */
export const SYNC_TASK_EVENT = 'cluster_sync_task'

/** Lifecycle state of a background synchronization task. */
export type SyncTaskStatus = 'running' | 'finished'

/**
 * A synchronization run executed in the background. Pushing many sites can take
 * longer than a reverse proxy waits for a request, so the server answers with
 * the task right away and reports its outcome later.
 */
export interface SyncTask {
  id: string
  label: string
  status: SyncTaskStatus
  /** Completed item-on-node pushes. */
  done: number
  total: number
  /** Set once the task has finished. */
  summary?: SyncSummary
  started_at: string
  finished_at?: string
}

/** Reads the progress or outcome of a background synchronization task. */
export function getSyncTask(id: string) {
  return http.get<SyncTask>(`/cluster/sync_tasks/${id}`)
}
