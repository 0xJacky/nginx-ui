import { http } from '@uozi-admin/request'

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

export interface LLMSessionResponse {
  session_id: string
  title: string
  path: string
  messages: ChatComplicationMessage[]
  message_count: number
  is_active: boolean
  created_at: string
  updated_at: string
}

const llm = {
  get_messages(path: string) {
    return http.get(`/llm_messages`, { params: { path } })
  },
  store_messages(data: { file_name?: string, messages?: ChatComplicationMessage[] }) {
    return http.post('/llm_messages', data)
  },
  codeCompletionWebSocketUrl: '/api/code_completion',
  get_code_completion_enabled_status() {
    return http.get<{ enabled: boolean }>('/code_completion/enabled')
  },

  // Session APIs
  get_sessions(pathOrType?: string, isType?: boolean) {
    const params: Record<string, string> = {}
    if (pathOrType) {
      if (isType) {
        params.type = pathOrType
      }
      else {
        params.path = pathOrType
      }
    }
    return http.get<LLMSessionResponse[]>('/llm_sessions', {
      params: Object.keys(params).length > 0 ? params : undefined,
    })
  },
  get_session(sessionId: string) {
    return http.get<LLMSessionResponse>(`/llm_sessions/${sessionId}`)
  },
  create_session(data: { title: string, path?: string, type?: string }) {
    return http.post<LLMSessionResponse>('/llm_sessions', data)
  },
  update_session(sessionId: string, data: { title?: string, messages?: ChatComplicationMessage[], is_active?: boolean }) {
    return http.put<LLMSessionResponse>(`/llm_sessions/${sessionId}`, data)
  },
  delete_session(sessionId: string) {
    return http.delete(`/llm_sessions/${sessionId}`)
  },
  duplicate_session(sessionId: string) {
    return http.post<LLMSessionResponse>(`/llm_sessions/${sessionId}/duplicate`)
  },
  generate_session_title(sessionId: string) {
    return http.post<{ title: string, message: string }>(`/llm_sessions/${sessionId}/generate_title`)
  },
}

export default llm
