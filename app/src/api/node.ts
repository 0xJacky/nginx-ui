import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Node extends ModelBase {
  name: string
  url: string
  token: string
  status: boolean
  enabled: boolean
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

const nodeApi = extendCurdApi(useCurdApi<Node>(baseUrl), {
  load_from_settings: () => http.post(`${baseUrl}/load_from_settings`),
  reloadNginx,
  restartNginx,
})

export default nodeApi
