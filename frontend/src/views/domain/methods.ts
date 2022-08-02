import gettext from '@/gettext'
import websocket from '@/lib/websocket'
import ReconnectingWebSocket from 'reconnecting-websocket'
import {message} from 'ant-design-vue'

const {$gettext} = gettext

const issue_cert = async (server_name: string, callback: Function) => {
    // message.info($gettext('Getting the certificate, please wait...'), 15)
    //
    // const ws: ReconnectingWebSocket = websocket('/api/cert/issue/' + server_name)
    //
    // ws.onopen = () => {
    //     ws.send('go')
    // }
    //
    // ws.onmessage = m => {
    //     const r = JSON.parse(m.data)
    //     switch (r.status) {
    //         case 'success':
    //             message.success(r.message, 10)
    //             break
    //         case 'info':
    //             message.info(r.message, 10)
    //             break
    //         case 'error':
    //             message.error(r.message, 10)
    //             break
    //     }
    //
    //     if (r.status === 'success' && r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
    //         callback(r.ssl_certificate, r.ssl_certificate_key)
    //     }
    // }
    setTimeout(() => {
        callback('a', 'b')
    }, 10000)
}

export {issue_cert}
