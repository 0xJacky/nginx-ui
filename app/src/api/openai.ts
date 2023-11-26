import http from '@/lib/http'

const openai = {
  store_record(data: any) {
    return http.post('/chat_gpt_record', data)
  }
}

export default openai
