<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { throttle } from 'lodash'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import ws from '@/lib/websocket'
import '@xterm/xterm/css/xterm.css'

let term: Terminal | null
let ping: undefined | ReturnType<typeof setTimeout>

const router = useRouter()
const websocket = shallowRef<ReconnectingWebSocket | WebSocket>()
const lostConnection = ref(false)
const insecureConnection = ref(false)
const isWebSocketReady = ref(false)

// Check if using HTTP in a non-localhost environment
function checkSecureConnection() {
  const hostname = window.location.hostname
  const protocol = window.location.protocol

  // Check if it's not localhost and not HTTPS
  if ((hostname !== 'localhost' && hostname !== '127.0.0.1') && protocol !== 'https:') {
    insecureConnection.value = true
  }
}

onMounted(() => {
  // Check connection security
  checkSecureConnection()

  const otpModal = use2FAModal()

  otpModal.open().then(secureSessionId => {
    websocket.value = ws(`/api/pty?X-Secure-Session-ID=${secureSessionId}`, false)

    nextTick(() => {
      websocket.value!.onmessage = wsOnMessage
      websocket.value!.onopen = wsOnOpen
      websocket.value!.onerror = () => {
        lostConnection.value = true
        isWebSocketReady.value = false
      }
      websocket.value!.onclose = () => {
        lostConnection.value = true
        isWebSocketReady.value = false
      }

      // Initialize terminal only after WebSocket is ready
      initTerm()
    })
  }).catch(() => {
    if (window.history.length > 1)
      router.go(-1)
    else
      router.push('/')
  })
})

interface Message {
  Type: number
  Data: string | null | { Cols: number, Rows: number }
}

const fitAddon = new FitAddon()

const fit = throttle(() => {
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

  // Only set up event handlers, but don't send messages until WebSocket is ready
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
  // Only send if WebSocket is ready
  if (websocket.value && isWebSocketReady.value) {
    websocket.value.send(JSON.stringify(data))
  }
}

function wsOnMessage(msg: { data: string | Uint8Array }) {
  term!.write(msg.data)
}

function wsOnOpen() {
  isWebSocketReady.value = true
  ping = setInterval(() => {
    sendMessage({ Type: 3, Data: null })
  }, 30000)
}

onUnmounted(() => {
  window.removeEventListener('resize', fit)
  clearInterval(ping)
  term?.dispose()
  websocket.value?.close()
})
</script>

<template>
  <div>
    <AAlert
      v-if="insecureConnection"
      class="mb-6"
      type="warning"
      show-icon
      :message="$gettext('You are accessing this terminal over an insecure HTTP connection on a non-localhost domain. This may expose sensitive information.')"
    />
    <AAlert
      v-if="lostConnection"
      class="mb-6"
      type="error"
      show-icon
      :message="$gettext('Connection lost, please refresh the page.')"
    />
    <div
      id="terminal"
      class="console"
    />
  </div>
</template>

<style lang="less" scoped>
.console {
  min-height: calc(100vh - 200px);

  :deep(.terminal) {
    padding: 10px;
  }

  :deep(.xterm-viewport) {
    border-radius: 5px;
    @media (max-width: 512px) {
      border-radius: 0;
    }
  }
}
</style>
