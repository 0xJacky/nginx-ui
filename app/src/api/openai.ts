import http from '@/lib/http'

export interface ChatComplicationMessage {
  role: string
  content: string
  name?: string
}

const openai = {
  store_record(data: { file_name?: string; messages?: ChatComplicationMessage[] }) {
    return http.post('/chat_gpt_record', data)
  },
}

export default openai
