import { http } from '@uozi-admin/request'

export interface RuntimeInfo {
  name: string
  os: string
  arch: string
  ex_path: string
  cur_version: Info
  in_docker: boolean
}

export interface Info {
  version: string
  build_id: number
  total_build: number
  short_hash: string
  instance_id?: string
}

export interface ReleaseInfo extends RuntimeInfo {
  tag_name?: string
  html_url: string
  published_at: string
  body: string
}

const upgrade = {
  get_latest_release(channel: string) {
    return http.get('/upgrade/release', {
      params: {
        channel,
      },
    })
  },
  get_channel() {
    return http.get('/upgrade/channel')
  },
  save_channel(channel: string) {
    return http.post('/upgrade/channel', { channel })
  },
  current_version(nodeId?: number, signal?: AbortSignal) {
    return http.get<Info>('/upgrade/current', {
      skipNodeProxy: nodeId !== undefined,
      headers: nodeId
        ? { 'X-Node-ID': nodeId }
        : undefined,
      timeout: 5000,
      signal,
      skipErrHandling: true,
    })
  },
}

export default upgrade
