import http from '@/lib/http'

export interface RuntimeInfo {
  name: string
  os: string
  arch: string
  ex_path: string
  body: string
  published_at: string
  cur_version: Info
}

interface Info {
  version: string
  build_id: number
  total_build: number
  short_hash: string
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
