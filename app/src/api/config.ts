import type { GetListResponse } from '@/api/curd'
import type { ChatComplicationMessage } from '@/api/openai'
import Curd from '@/api/curd'
import http from '@/lib/http'

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

class ConfigCurd extends Curd<Config> {
  constructor() {
    super('/configs')
  }

  get_base_path() {
    return http.get('/config_base_path')
  }

  mkdir(basePath: string, name: string) {
    return http.post('/config_mkdir', { base_path: basePath, folder_name: name })
  }

  rename(basePath: string, origName: string, newName: string, syncNodeIds?: number[]) {
    return http.post('/config_rename', {
      base_path: basePath,
      orig_name: origName,
      new_name: newName,
      sync_node_ids: syncNodeIds,
    })
  }

  get_history(filepath: string) {
    return http.get<GetListResponse<ConfigBackup>>('/config_histories', { params: { filepath } })
  }
}

const config: ConfigCurd = new ConfigCurd()

export default config
