import type { ModelBase } from '@/api/curd'
import type { ConfigurationField } from '@/api/plugin'
import { http, useCurdApi } from '@uozi-admin/request'

export type RefreshStatus = '' | 'ok' | 'failed'

/**
 * A list of addresses to deny, fetched from a security.blocklist plugin on
 * an interval and written to a file of its own in the nginx configuration.
 */
export interface BlocklistSource extends ModelBase {
  name: string
  /** "plugin:<code>", the source kind of a security.blocklist plugin. */
  kind: string
  config: Record<string, string>
  refresh_seconds: number
  enabled: boolean
  last_run_at?: string | null
  next_run_at?: string | null
  last_status: RefreshStatus
  last_message: string
  entry_count: number
  /** The directive to add where the list should apply. */
  include?: string
  /** The absolute path of the generated file. */
  path?: string
}

/** A kind of source an enabled security.blocklist plugin offers. */
export interface BlocklistKind {
  kind: string
  name: string
  plugin_id?: string
  fields: ConfigurationField[]
  /** Default refresh interval of a new source of this kind. */
  refresh_seconds: number
}

const blocklistSource = useCurdApi<BlocklistSource>('/blocklist_sources')

/** Lists the source kinds enabled security.blocklist plugins offer. */
export function listBlocklistKinds(): Promise<{ data: BlocklistKind[] }> {
  return http.get('/blocklist_sources/kinds')
}

/** Fetches a source now and returns it with the outcome. */
export function refreshBlocklistSource(id: number): Promise<BlocklistSource> {
  return http.post(`/blocklist_sources/${id}/refresh`)
}

export default blocklistSource
