import type { SyncScope, SyncSummary } from '@/api/cluster_sync'
import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Node extends ModelBase {
  name: string
  url: string
  status: boolean
  enabled: boolean
  auth_method: 'legacy_secret' | 'paired_ed25519'
  /** Only ever the redaction sentinel; reveal the real value with getSecret. */
  legacy_secret?: string
  has_credential: boolean
  credential_status: 'active' | 'unpaired' | 'rotating' | 'revoked'
  last_credential_use_at?: string
  response_at?: string
}

export interface NodeStatus {
  avg_load: {
    load1: number
    load5: number
    load15: number
  }
  cpu_percent: number
  memory_percent: number
  disk_percent: number
  network: {
    name: string
    bytesSent: number
    bytesRecv: number
    packetsSent: number
    packetsRecv: number
    errin: number
    errout: number
    dropin: number
    dropout: number
    fifoin: number
    fifoout: number
  }
  status: boolean
  response_at?: string
  upstream_status_map: {
    [key: string]: {
      online: boolean
      latency: number
    }
  }
}

export interface NodeInfo {
  node_runtime_info: {
    os: string
    arch: string
    ex_path: string
    cur_version: string
    in_docker: boolean
  }
  version: string
  cpu_num: number
  memory_total: string
  disk_total: string
}

export interface AnalyticNode extends Node, NodeInfo, NodeStatus {
}

const baseUrl = '/nodes'

function reloadNginx(nodeIds: number[]) {
  return http.post('/nodes/reload_nginx', { node_ids: nodeIds })
}

function restartNginx(nodeIds: number[]) {
  return http.post('/nodes/restart_nginx', { node_ids: nodeIds })
}

/** Pushes the local configurations, sites and streams to the given nodes. */
function syncConfigs(nodeIds: number[], scope: SyncScope = {}) {
  return http.post<SyncSummary>(`${baseUrl}/sync`, { node_ids: nodeIds, ...scope })
}

const nodeApi = extendCurdApi(useCurdApi<Node>(baseUrl), {
  load_from_settings: () => http.post(`${baseUrl}/load_from_settings`),
  reloadNginx,
  restartNginx,
  syncConfigs,
  getSecret: (id: number) => http.get<{ value: string }>(`${baseUrl}/${id}/secret`),
  rotateCredential: (id: number) => http.post(`${baseUrl}/${id}/credentials/rotate`),
})

export default nodeApi
