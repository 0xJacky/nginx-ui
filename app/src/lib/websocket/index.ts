import ReconnectingWebSocket from 'reconnecting-websocket'
import { storeToRefs } from 'pinia'
import { useSettingsStore, useUserStore } from '@/pinia'
import { urlJoin } from '@/lib/helper'

function ws(url: string, reconnect: boolean = true): ReconnectingWebSocket | WebSocket {
  const user = useUserStore()
  const settings = useSettingsStore()
  const { token } = storeToRefs(user)

  const protocol = location.protocol === 'https:' ? 'wss://' : 'ws://'

  const node_id = (settings.environment.id > 0) ? (`&x_node_id=${settings.environment.id}`) : ''

  const _url = urlJoin(protocol + window.location.host, window.location.pathname,
    url, `?token=${btoa(token.value)}`, node_id)

  if (reconnect)
    return new ReconnectingWebSocket(_url, undefined, { maxRetries: 10 })

  return new WebSocket(_url)
}

export default ws
