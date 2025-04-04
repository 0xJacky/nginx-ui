import { useUserStore } from '@/pinia'
import { SSE } from 'sse.js'

const cache_index = {
  index_status() {
    const { token } = useUserStore()
    const url = `/api/index/status`

    return new SSE(url, {
      headers: {
        Authorization: token,
      },
    })
  },
}

export default cache_index
