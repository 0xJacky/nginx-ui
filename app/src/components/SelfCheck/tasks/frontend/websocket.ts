import type { FrontendTask } from '../types'
import type { ReportStatusType, SelfCheckAccessOptions } from '@/api/self_check'
import selfCheck, { ReportStatus } from '@/api/self_check'
import { useWebSocket } from '@/lib/websocket'

const CONNECT_TIMEOUT_MS = 5000

/**
 * WebSocket Task
 *
 * Probes the backend WebSocket endpoint. `autoClose: false` is required:
 * this task runs from a store action (no effectScope), so VueUse's
 * autoClose would register a `beforeunload` listener that never gets
 * cleaned up. We own cleanup via the `settled` gate.
 */
const WebsocketTask: FrontendTask = {
  key: 'websocket',
  name: () => 'WebSocket',
  description: () => $gettext('Support communication with the backend through the WebSocket protocol. '
    + 'If your Nginx UI is being used via an Nginx reverse proxy, '
    + 'please refer to this link to write the corresponding configuration file: '
    + 'https://nginxui.com/guide/nginx-proxy-example.html'),
  check: (options?: SelfCheckAccessOptions): Promise<ReportStatusType> => {
    return new Promise<ReportStatusType>(resolve => {
      let settled = false
      let timer: ReturnType<typeof setTimeout> | undefined
      let closeSocket: (() => void) | undefined

      function finish(status: ReportStatusType) {
        if (settled)
          return
        settled = true
        clearTimeout(timer)
        closeSocket?.()
        resolve(status)
      }

      const { close } = useWebSocket(
        selfCheck.getWebsocketUrl(options),
        false,
        {
          autoClose: false,
          onConnected: () => finish(ReportStatus.Success),
          onError: () => finish(ReportStatus.Error),
          // Fast loopback can deliver close before open; a clean close means
          // the handshake succeeded.
          onDisconnected: (_ws, ev) => finish(ev.wasClean ? ReportStatus.Success : ReportStatus.Error),
        },
        selfCheck.getWebsocketQuery(options),
      )
      closeSocket = close

      timer = setTimeout(finish, CONNECT_TIMEOUT_MS, ReportStatus.Error)
    })
  },
}

export default WebsocketTask
