<script setup lang="ts">
import 'xterm/css/xterm.css'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { onMounted, onUnmounted } from 'vue'
import _ from 'lodash'
import { useGettext } from 'vue3-gettext'
import ws from '@/lib/websocket'

const { $gettext } = useGettext()

let term: Terminal | null
let ping: number

const websocket = ws('/api/pty')

onMounted(() => {
  initTerm()

  websocket.onmessage = wsOnMessage
  websocket.onopen = wsOnOpen
})

interface Message {
  Type: number
  Data: string | null | { Cols: number; Rows: number }
}

const fitAddon = new FitAddon()

const fit = _.throttle(() => {
  fitAddon.fit()
}, 50)

function initTerm() {
  term = new Terminal({
    convertEol: true,
    fontSize: 14,
    cursorStyle: 'block',
    scrollback: 1000,
    theme: {
      background: '#000',
    },
  })

  term.loadAddon(fitAddon)
  term.open(document.getElementById('terminal')!)
  setTimeout(() => {
    fitAddon.fit()
  }, 60)
  window.addEventListener('resize', fit)
  term.focus()

  term.onData(key => {
    const order: Message = {
      Data: key,
      Type: 1,
    }

    sendMessage(order)
  })
  term.onBinary(data => {
    sendMessage({ Type: 1, Data: data })
  })
  term.onResize(data => {
    sendMessage({ Type: 2, Data: { Cols: data.cols, Rows: data.rows } })
  })
}

function sendMessage(data: Message) {
  websocket.send(JSON.stringify(data))
}

function wsOnMessage(msg: { data: string | Uint8Array }) {
  term!.write(msg.data)
}

function wsOnOpen() {
  ping = setInterval(() => {
    sendMessage({ Type: 3, Data: null })
  }, 30000)
}

onUnmounted(() => {
  window.removeEventListener('resize', fit)
  clearInterval(ping)
  term?.dispose()
  ping = 0
  websocket.close()
})

</script>

<template>
  <ACard :title="$gettext('Terminal')">
    <div
      id="terminal"
      class="console"
    />
  </ACard>
</template>

<style lang="less" scoped>
.console {
  min-height: calc(100vh - 300px);

  :deep(.terminal) {
    padding: 10px;
  }

  :deep(.xterm-viewport) {
    border-radius: 5px;
  }
}
</style>
