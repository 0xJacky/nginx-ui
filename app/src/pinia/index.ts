import { useAppStore } from './moudule/app'
import { useDnsStore } from './moudule/dns'
import { useGlobalStore } from './moudule/global'
import { useProxyAvailabilityStore } from './moudule/proxyAvailability'
import { useSettingsStore } from './moudule/settings'
import { useTerminalStore } from './moudule/terminal'
import { useUserStore } from './moudule/user'
import { useWebSocketEventBusStore } from './moudule/websocketEventBus'

// Re-export types
export type { EventHandler, EventSubscription, WebSocketMessage } from './moudule/websocketEventBus'

export {
  useAppStore,
  useDnsStore,
  useGlobalStore,
  useProxyAvailabilityStore,
  useSettingsStore,
  useTerminalStore,
  useUserStore,
  useWebSocketEventBusStore,
}
