import ReconnectingWebSocket from 'reconnecting-websocket'
import {useUserStore} from '@/pinia/user'
import {storeToRefs} from 'pinia'


function ws(url: string): ReconnectingWebSocket {
    const user = useUserStore()
    const {token} = storeToRefs(user)

    const protocol = location.protocol === 'https:' ? 'wss://' : 'ws://'

    return new ReconnectingWebSocket(
        protocol + window.location.host + url + '?token=' + btoa(token.value))
}

export default ws
