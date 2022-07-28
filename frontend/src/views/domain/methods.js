import $gettext from '@/lib/translate/gettext'
import store from '@/lib/store'
import Vue from 'vue'

const issue_cert = (server_name, callback) => {
    Vue.prototype.$message.info($gettext('Getting the certificate, please wait...'), 15)
    const ws = new WebSocket(Vue.prototype.getWebSocketRoot() + '/cert/issue/' + server_name
        + '?token=' + btoa(store.state.user.token))

    ws.onopen = () => {
        ws.send('go')
    }

    ws.onmessage = m => {
        const r = JSON.parse(m.data)
        switch (r.status) {
            case 'success':
                Vue.prototype.$message.success(r.message, 10)
                break
            case 'info':
                Vue.prototype.$message.info(r.message, 10)
                break
            case 'error':
                Vue.prototype.$message.error(r.message, 10)
                break
        }

        if (r.status === 'success' && r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
            callback(r.ssl_certificate, r.ssl_certificate_key)
        }
    }
    // setTimeout(() => {
    //     callback('a', 'b')
    // }, 10000)
}

export {issue_cert}
