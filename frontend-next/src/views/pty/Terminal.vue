<script setup lang="ts">
import ReconnectingWebSocket from 'reconnecting-websocket'
import 'xterm/css/xterm.css'
import {Terminal} from 'xterm'
import {FitAddon} from 'xterm-addon-fit'
import {onMounted, onUnmounted} from "vue"
import {useUserStore} from "@/pinia/user"
import {storeToRefs} from "pinia"
import _ from 'lodash'

const user = useUserStore()
const {token} = storeToRefs(user)

let term: Terminal | null
let ping: null | NodeJS.Timer

const protocol = location.protocol === 'https:' ? 'wss://' : 'ws://'

const websocket = new ReconnectingWebSocket(
    protocol + window.location.host + '/api/pty?token='
    + btoa(token.value))

onMounted(() => {
    initTerm()

    websocket.onmessage = wsOnMessage
    websocket.onopen = wsOnOpen
})

interface Message {
    Type: Number,
    Data: any | null
}

const fitAddon = new FitAddon()

const fit = _.throttle(function () {
    fitAddon.fit()
}, 50)

function initTerm() {
    term = new Terminal({
        rendererType: 'canvas',
        convertEol: true,
        fontSize: 14,
        cursorStyle: 'block',
        scrollback: 1000,
        theme: {
            background: 'rgba(3,14,32,0.7)'
        },
    })

    term.loadAddon(fitAddon)
    // this.fitAddon = fitAddon
    term.open(document.getElementById('terminal')!)
    setTimeout(() => {
        fitAddon.fit()
    }, 60)
    window.addEventListener('resize', fit)
    term.focus()

    term.onData(function (key) {
        let order: Message = {
            Data: key,
            Type: 1
        }
        sendMessage(order)
    })
    term.onBinary(data => {
        sendMessage({Type: 1, Data: data})
    })
    term.onResize(data => {
        sendMessage({Type: 2, Data: {Cols: data.cols, Rows: data.rows}})
    })
}

function sendMessage(data: Message) {
    websocket.send(JSON.stringify(data))
}

function wsOnMessage(msg: { data: any }) {
    term!.write(msg.data)
}

function wsOnOpen() {
    ping = setInterval(function () {
        sendMessage({Type: 3, Data: null})
    }, 30000)
}

onUnmounted(() => {
    window.removeEventListener('resize', fit)
    clearInterval(ping!)
    ping = null
    websocket.close()
})

</script>

<template>
    <a-card :title="$gettext('Terminal')">
        <div class="console" id="terminal"></div>
    </a-card>
</template>

<style lang="less" scoped>
.console {
    min-height: calc(100vh - 300px);
}
</style>
