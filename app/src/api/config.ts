import Curd from '@/api/curd'
import type { ChatComplicationMessage } from '@/api/openai'
import http from '@/lib/http'

export interface Config {
  name: string
  content: string
  chatgpt_messages: ChatComplicationMessage[]
  filepath: string
  modified_at: string
}

class ConfigCurd extends Curd<Config> {
  constructor() {
    super('/config')
  }

  get_base_path() {
    return http.get('/config_base_path')
  }
}

const config: ConfigCurd = new ConfigCurd()

export default config
