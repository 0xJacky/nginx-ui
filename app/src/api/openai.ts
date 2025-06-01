import type ReconnectingWebSocket from 'reconnecting-websocket'
import { http } from '@uozi-admin/request'
import ws from '@/lib/websocket'

export interface ChatComplicationMessage {
  role: string
  content: string
  name?: string
}

export interface CodeCompletionRequest {
  context: string // Context of the code
  code: string // Code before the cursor
  suffix?: string // Code after the cursor
  language?: string // Programming language
  position?: { // Cursor position
    row: number
    column: number
  }
}

export interface CodeCompletionResponse {
  code: string // Completed code
}

const openai = {
  get_record(path: string) {
    return http.get(`/chatgpt/history`, { params: { path } })
  },
  store_record(data: { file_name?: string, messages?: ChatComplicationMessage[] }) {
    return http.post('/chatgpt_record', data)
  },
  code_completion() {
    return ws('/api/code_completion') as ReconnectingWebSocket
  },
  get_code_completion_enabled_status() {
    return http.get<{ enabled: boolean }>('/code_completion/enabled')
  },
}

export default openai
