import Curd from '@/api/curd'
import type { ChatComplicationMessage } from '@/api/openai'

export interface Config {
  name: string
  content: string
  chatgpt_messages: ChatComplicationMessage[]
  file_path: string
  modified_at: string
}

const config: Curd<Config> = new Curd('/config')

export default config
