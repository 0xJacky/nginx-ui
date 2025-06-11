import { http } from '@uozi-admin/request'

export interface RuntimeInfo {
  name: string
  os: string
  arch: string
  ex_path: string
  cur_version: Info
  in_docker: boolean
}

interface Info {
  version: string
  build_id: number
  total_build: number
  short_hash: string
}

export interface ReleaseInfo extends RuntimeInfo {
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
  current_version() {
    return http.get('/upgrade/current')
  },
}

export default upgrade
