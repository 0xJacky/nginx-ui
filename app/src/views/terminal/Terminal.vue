<script setup lang="ts">
import twoFA from '@/api/2fa'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import ws from '@/lib/websocket'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import _ from 'lodash'
import '@xterm/xterm/css/xterm.css'

let term: Terminal | null
let ping: NodeJS.Timeout

const router = useRouter()
const websocket = shallowRef()
const lostConnection = ref(false)

onMounted(() => {
  twoFA.secure_session_status()

  const otpModal = use2FAModal()

  otpModal.open().then(secureSessionId => {
    websocket.value = ws(`/api/pty?X-Secure-Session-ID=${secureSessionId}`, false)

    nextTick(() => {
      initTerm()
      websocket.value.onmessage = wsOnMessage
      websocket.value.onopen = wsOnOpen
      websocket.value.onerror = () => {
        lostConnection.value = true
      }
      websocket.value.onclose = () => {
        lostConnection.value = true
      }
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
  websocket.value.send(JSON.stringify(data))
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
  websocket.value?.close()
})
</script>

<template>
  <ACard :title="$gettext('Terminal')">
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
