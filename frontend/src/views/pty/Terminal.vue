<template>
    <a-card :title="$gettext('Terminal')">
        <div class="console" id="terminal"></div>
    </a-card>
</template>

<script>
import ReconnectingWebSocket from 'reconnecting-websocket'
import 'xterm/css/xterm.css'
import {Terminal} from 'xterm'
import {FitAddon} from 'xterm-addon-fit'

const _ = require('lodash')

export default {
    name: 'Terminal',
    data() {
        return {
            term: null,
            ping: null
        }
    },
    created() {
        this.websocket = new ReconnectingWebSocket(this.getWebSocketRoot() + '/pty?token='
            + btoa(this.$store.state.user.token))
        this.websocket.onmessage = this.wsOnMessage
        this.websocket.onopen = this.wsOnOpen
    },
    mounted() {
        this.initTerm()
    },
    destroyed() {
        window.removeEventListener('resize', this.fit)
        clearInterval(this.ping)
        this.ping = null
        this.term.close()
        this.websocket.close()
    },
    methods: {
        fit: _.throttle(function () {
            this.fitAddon.fit()
        }, 50),
        initTerm() {
            const term = new Terminal({
                rendererType: 'canvas',
                convertEol: true,
                fontSize: 14,
                cursorStyle: 'block',
                scrollback: 1000,
                theme: {
                    background: 'rgba(3,14,32,0.7)'
                },
            })
            const fitAddon = new FitAddon()
            term.loadAddon(fitAddon)
            this.fitAddon = fitAddon
            term.open(document.getElementById('terminal'))
            setTimeout(() => {
                fitAddon.fit()
            }, 60)
            window.addEventListener('resize', this.fit)
            term.focus()

            let that = this

            term.onData(function (key) {
                let order = {
                    Data: key,
                    Type: 1
                }
                that.sendMessage(order)
            })
            term.onBinary(data => {
                that.sendMessage({Type: 1, Data: data})
            })
            term.onResize(data => {
                that.sendMessage({Type: 2, Data: {Cols: data.cols, Rows: data.rows}})
            })
            this.term = term
        },
        wsOnMessage(msg) {
            this.term.write(msg.data)
        },
        wsOnOpen() {
            const that = this
            this.ping = setInterval(function () {
                that.sendMessage({Type: 3})
            }, 30000)
        },
        sendMessage(data) {
            this.websocket.send(JSON.stringify(data))
        }
    }
}
</script>

<style lang="less" scoped>
.console {
    min-height: calc(100vh - 300px);
}
</style>
