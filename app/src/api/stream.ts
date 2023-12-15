import Curd from '@/api/curd'
import http from '@/lib/http'
import type { ChatComplicationMessage } from '@/api/openai'
import type { NgxConfig } from '@/api/ngx'

export interface Stream {
  modified_at: string
  advanced: boolean
  enabled: boolean
  name: string
  config: string
  chatgpt_messages: ChatComplicationMessage[]
  tokenized?: NgxConfig
}

class StreamCurd extends Curd<Stream> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  enable(name: string, config?: any) {
    return http.post(`${this.baseUrl}/${name}/enable`, undefined, config)
  }

  disable(name: string) {
    return http.post(`${this.baseUrl}/${name}/disable`)
  }

  duplicate(name: string, data: { name: string }): Promise<{ dst: string }> {
    return http.post(`${this.baseUrl}/${name}/duplicate`, data)
  }

  advance_mode(name: string, data: { advanced: boolean }) {
    return http.post(`${this.baseUrl}/${name}/advance`, data)
  }
}

const stream = new StreamCurd('/stream')

export default stream
