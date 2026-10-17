import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface NginxLogData {
  type?: string
  path?: string
  name?: string
  config_file?: string
}

export interface LogListResponse {
  data: NginxLogData[]
}

export interface DefaultLogDir {
  access_log_dir: string
  access_log_path?: string
  error_log_path?: string
}

const nginx_log = extendCurdApi(useCurdApi('/nginx_logs'), {
  list(params?: Pick<NginxLogData, 'type' | 'name' | 'path'>): Promise<LogListResponse> {
    return http.get('/nginx_logs', { params })
  },

  page(page = 0, data: NginxLogData | undefined = undefined) {
    return http.post(`/nginx_log/page?page=${page}`, data)
  },

  // Whether this node used the advanced indexing built into earlier releases
  // and has not handed it over to the log analytics plugin yet
  getLegacyIndexingStatus(): Promise<{ enabled: boolean }> {
    return http.get('/nginx_log/legacy_indexing')
  },

  // Directory nginx writes its default access log to, used to propose a
  // per-site access_log path that is already inside the log whitelist, and
  // the default log files a site without its own log directive falls back to
  getDefaultLogDir(): Promise<DefaultLogDir> {
    return http.get('/nginx_log/default_log_dir')
  },
})

export default nginx_log
