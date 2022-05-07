import $gettext from '@/lib/translate/gettext'
import store from '@/lib/store'
import Vue from 'vue'
const unparse = (text, config) => {
    // http_listen_port: /listen (.*);/i,
    // https_listen_port: /listen (.*) ssl/i,
    const reg = {
        server_name: /server_name[\s](.*);/ig,
        index: /index[\s](.*);/i,
        root: /root[\s](.*);/i,
        ssl_certificate: /ssl_certificate[\s](.*);/i,
        ssl_certificate_key: /ssl_certificate_key[\s](.*);/i
    }
    text = text.replace(/listen[\s](.*);/i, 'listen\t'
        + config['http_listen_port'] + ';')
    text = text.replace(/listen[\s](.*) ssl/i, 'listen\t'
        + config['https_listen_port'] + ' ssl')

    text = text.replace(/listen(.*):(.*);/i, 'listen\t[::]:'
        + config['http_listen_port'] + ';')
    text = text.replace(/listen(.*):(.*) ssl/i, 'listen\t[::]:'
        + config['https_listen_port'] + ' ssl')

    for (let k in reg) {
        text = text.replace(new RegExp(reg[k]), k + '\t' +
            (config[k] !== undefined ? config[k] : ' ') + ';')
    }

    return text
}

const issue_cert = (server_name, callback) => {
    Vue.prototype.$message.info($gettext('Note: The server_name in the current configuration must be the domain name you need to get the certificate.'), 15)
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
}

export {unparse, issue_cert}
