import type { FrontendTask } from '../types'
import type { ReportStatusType } from '@/api/self_check'
import selfCheck, { ReportStatus } from '@/api/self_check'
import { useWebSocket } from '@/lib/websocket'

/**
 * WebSocket Task
 *
 * Checks if the application is able to connect to the backend through the WebSocket protocol
 */
const WebsocketTask: FrontendTask = {
  key: 'websocket',
  name: () => 'WebSocket',
  description: () => $gettext('Support communication with the backend through the WebSocket protocol. '
    + 'If your Nginx UI is being used via an Nginx reverse proxy, '
    + 'please refer to this link to write the corresponding configuration file: '
    + 'https://nginxui.com/guide/nginx-proxy-example.html'),
  check: async (): Promise<ReportStatusType> => {
    try {
      const connected = await new Promise<boolean>(resolve => {
        const { ws } = useWebSocket(selfCheck.websocketUrl, false)
        const socket = ws.value!
        socket.onopen = () => {
          socket.close()
          resolve(true)
        }
        socket.onerror = () => {
          resolve(false)
        }
        // Set a timeout for the connection attempt
        setTimeout(() => {
          resolve(false)
        }, 5000)
      })

      if (connected) {
        return ReportStatus.Success
      }
      else {
        return ReportStatus.Error
      }
    }
    catch (error) {
      console.error(error)
      return ReportStatus.Error
    }
  },
}

export default WebsocketTask
