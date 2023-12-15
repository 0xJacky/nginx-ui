import ws from '@/lib/websocket'

export interface UpstreamStatus {
  online: boolean
  latency: number
}

const upstream = {
  availability_test() {
    return ws('/api/availability_test')
  },
}

export default upstream
