import type { GetListResponse } from '@/api/curd'
import type { ChatComplicationMessage } from '@/api/openai'
import { http, useCurdApi } from '@uozi-admin/request'

export interface ModelBase {
  id: number
  created_at: string
  updated_at: string
}

export interface Config {
  name: string
  content: string
  chatgpt_messages: ChatComplicationMessage[]
  filepath: string
  modified_at: string
  sync_node_ids?: number[]
  sync_overwrite?: false
  dir: string
}

export interface ConfigBackup extends ModelBase {
  name: string
  filepath: string
  content: string
}

const config = useCurdApi<Config>('/configs', {
  get_base_path: () => http.get('/config_base_path'),
  mkdir: (basePath: string, name: string) => http.post('/config_mkdir', { base_path: basePath, folder_name: name }),
  rename: (basePath: string, origName: string, newName: string, syncNodeIds?: number[]) => http.post('/config_rename', {
    base_path: basePath,
    orig_name: origName,
    new_name: newName,
    sync_node_ids: syncNodeIds,
  }),
  get_history: (filepath: string) => http.get<GetListResponse<ConfigBackup>>('/config_histories', { params: { filepath } }),
})

export default config
