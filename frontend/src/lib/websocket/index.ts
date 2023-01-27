import ReconnectingWebSocket from 'reconnecting-websocket'
import {useUserStore} from '@/pinia'
import {storeToRefs} from 'pinia'
import {urlJoin} from '@/lib/helper'


function ws(url: string, reconnect: boolean = true): ReconnectingWebSocket | WebSocket {
    const user = useUserStore()
    const {token} = storeToRefs(user)

    const protocol = location.protocol === 'https:' ? 'wss://' : 'ws://'

    const _url = urlJoin(protocol + window.location.host, window.location.pathname,
        url, '?token=' + btoa(token.value))

    if (reconnect) {
        return new ReconnectingWebSocket(_url)
    }

    return new WebSocket(_url)

}

export default ws
