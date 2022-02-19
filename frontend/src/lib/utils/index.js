import scrollPosition from './scroll-position'

export default {
    // eslint-disable-next-line no-unused-vars
    install(Vue, options) {
        Vue.prototype.extend = (target, source) => {
            for (let obj in source) {
                target[obj] = source[obj]
            }
            return target
        }

        Vue.prototype.getClientWidth = () => {
            return document.body.clientWidth
        }

        Vue.prototype.collapse = () => {
            return !(Vue.prototype.getClientWidth() > 768 || Vue.prototype.getClientWidth() < 512)
        }

        Vue.prototype.bytesToSize = (bytes) => {
            if (bytes === 0) return '0 B'

            const k = 1024

            const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']

            const i = Math.floor(Math.log(bytes) / Math.log(k))
            return (bytes / Math.pow(k, i)).toPrecision(3) + ' ' + sizes[i]
        }

        Vue.prototype.scrollPosition = scrollPosition

        Vue.prototype.getWebSocketRoot = () => {
            const protocol = location.protocol === 'https:' ? 'wss://' : 'ws://'
            if (process.env.NODE_ENV === 'development' && process.env['VUE_APP_API_WSS_ROOT']) {
                return process.env['VUE_APP_API_WSS_ROOT']
            }
            return protocol + location.host + process.env['VUE_APP_API_WSS_ROOT']
        }
    }
}
