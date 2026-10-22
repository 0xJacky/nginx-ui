import type { UpgradeOptions } from '@/lib/upgrade'
import upgrade from '@/api/upgrade'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { runUpgrade } from '@/lib/upgrade'
import { useWebSocket } from '@/lib/websocket'
import { useSettingsStore } from '@/pinia'

interface PerformUpgradeOptions extends Pick<UpgradeOptions, 'control' | 'target' | 'onLog' | 'onProgress'> {
  nodeId?: number
  secureSessionId: string
}

export function useUpgrade() {
  const settings = useSettingsStore()
  const { open: authorize } = use2FAModal()
  const controller = new AbortController()
  onScopeDispose(() => controller.abort())

  function perform(options: PerformUpgradeOptions) {
    const nodeId = options.nodeId ?? settings.node.id ?? 0
    const secureSessionId = options.secureSessionId
    return runUpgrade({
      ...options,
      signal: controller.signal,
      readVersion: signal => upgrade.current_version(nodeId, signal),
      connect(callbacks) {
        const connection = useWebSocket('/api/upgrade/perform', false, {
          immediate: false,
          autoClose: false,
          onConnected: callbacks.onConnected,
          onMessage: (_socket, event) => callbacks.onMessage(event.data),
          onDisconnected: callbacks.onDisconnected,
          onError: callbacks.onError,
        }, {
          'x_node_id': String(nodeId),
          'X-Secure-Session-ID': secureSessionId,
        })
        connection.open()
        return connection.close
      },
    })
  }

  return { authorize, perform }
}
