import type { FrontendTask } from '../types'
import type { ReportStatusType } from '@/api/self_check'
import { ReportStatus } from '@/api/self_check'
import { useSSE } from '@/composables/useSSE'

/**
 * SSE Task
 *
 * Checks if the application is able to connect to the backend through the Server-Sent Events protocol
 */
const SSETask: FrontendTask = {
  key: 'sse',
  name: () => 'SSE',
  description: () => $gettext('Support communication with the backend through the Server-Sent Events protocol. '
    + 'If your Nginx UI is being used via an Nginx reverse proxy, '
    + 'please refer to this link to write the corresponding configuration file: '
    + 'https://nginxui.com/guide/nginx-proxy-example.html'),
  check: async (): Promise<ReportStatusType> => {
    try {
      const connected = await new Promise<boolean>(resolve => {
        const { connect, disconnect } = useSSE()

        // Use the connect method from useSSE
        connect({
          url: '/api/self_check/sse',
          onMessage: () => {
            resolve(true)
          },
          onError: () => {
            resolve(false)
            disconnect()
          },
          reconnectInterval: 0,
        })

        // Set a timeout for the connection attempt
        setTimeout(() => {
          resolve(false)
          disconnect()
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

export default SSETask
