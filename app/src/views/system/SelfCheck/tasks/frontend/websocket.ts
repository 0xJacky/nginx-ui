import type { FrontendTask, TaskReport } from '../types'
import selfCheck from '@/api/self_check'

const WebsocketTask: FrontendTask = {
  name: () => 'WebSocket',
  description: () => $gettext('Support communication with the backend through the WebSocket protocol. '
    + 'If your Nginx UI is being used via an Nginx reverse proxy, '
    + 'please refer to this link to write the corresponding configuration file: '
    + 'https://nginxui.com/guide/nginx-proxy-example.html'),
  type: 'frontend',
  check: async (): Promise<TaskReport> => {
    // Task name for the report
    const name = 'Frontend-Websocket'

    try {
      const connected = await new Promise<boolean>(resolve => {
        const ws = selfCheck.websocket()
        ws.onopen = () => {
          resolve(true)
        }
        ws.onerror = () => {
          resolve(false)
        }
        // Set a timeout for the connection attempt
        setTimeout(() => {
          resolve(false)
        }, 5000)
      })

      if (connected) {
        return {
          name,
          status: 'success',
          type: 'frontend',
          message: 'WebSocket connection successful.',
        }
      }
      else {
        return {
          name,
          status: 'error',
          type: 'frontend',
          message: 'WebSocket connection failed.',
          err: new Error('WebSocket connection failed.'),
        }
      }
    }
    catch (error) {
      return {
        name,
        status: 'error',
        type: 'frontend',
        message: 'WebSocket connection error.',
        err: error instanceof Error ? error : new Error(String(error)),
      }
    }
  },
}

export default WebsocketTask
