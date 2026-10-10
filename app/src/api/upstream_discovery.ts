import type { RefreshStatus } from '@/api/blocklist'
import type { ModelBase } from '@/api/curd'
import type { ConfigurationField } from '@/api/plugin'
import { http, useCurdApi } from '@uozi-admin/request'

/**
 * Binds an nginx upstream to a service an upstream.discovery plugin
 * resolves. The servers are written on an interval to a file of its own in
 * the nginx configuration.
 */
export interface UpstreamDiscovery extends ModelBase {
  upstream_name: string
  /** "plugin:<code>", the provider of an upstream.discovery plugin. */
  kind: string
  config: Record<string, string>
  service: string
  refresh_seconds: number
  /** Appended inside the upstream block, e.g. keepalive. */
  extra_directives: string
  enabled: boolean
  last_run_at?: string | null
  next_run_at?: string | null
  last_status: RefreshStatus
  last_message: string
  target_count: number
  /** The directive to add to the http block. */
  include?: string
  /** The absolute path of the generated file. */
  path?: string
}

/** A provider an enabled upstream.discovery plugin offers. */
export interface DiscoveryProvider {
  kind: string
  name: string
  plugin_id?: string
  fields: ConfigurationField[]
}

const upstreamDiscovery = useCurdApi<UpstreamDiscovery>('/upstream_discoveries')

/** Lists the providers enabled upstream.discovery plugins offer. */
export function listDiscoveryProviders(): Promise<{ data: DiscoveryProvider[] }> {
  return http.get('/upstream_discoveries/kinds')
}

/** Resolves a binding now and returns it with the outcome. */
export function refreshUpstreamDiscovery(id: number): Promise<UpstreamDiscovery> {
  return http.post(`/upstream_discoveries/${id}/refresh`)
}

export default upstreamDiscovery
