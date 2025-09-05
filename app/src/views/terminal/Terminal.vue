<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { throttle } from 'lodash'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import ws from '@/lib/websocket'
import TerminalRightPanel from './components/TerminalRightPanel.vue'
import TerminalStatusBar from './components/TerminalStatusBar.vue'
import '@xterm/xterm/css/xterm.css'

let term: Terminal | null
let ping: undefined | ReturnType<typeof setTimeout>

const router = useRouter()
const websocket = shallowRef<ReconnectingWebSocket | WebSocket>()
const lostConnection = ref(false)
const insecureConnection = ref(false)
const isWebSocketReady = ref(false)
const isRightPanelVisible = ref(false)
const rightPanelRef = ref<InstanceType<typeof TerminalRightPanel>>()

// Keep ref for terminal layout
const terminalLayoutRef = ref<HTMLElement>()

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

    // Monitor terminal input for LLM context
    handleTerminalInput(key)

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

function refreshTerminal() {
  window.location.reload()
}

function toggleRightPanel() {
  isRightPanelVisible.value = !isRightPanelVisible.value
}

// Monitor terminal input to provide context to LLM
function handleTerminalInput(data: string) {
  if (rightPanelRef.value && data.includes('\r')) {
    // Extract command when Enter is pressed
    const command = data.replace('\r', '').trim()
    if (command) {
      rightPanelRef.value.updateCurrentCommand(command)
    }
  }
}
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
      action
    >
      <template #action>
        <AButton
          size="small"
          type="text"
          @click="refreshTerminal"
        >
          <template #icon>
            <ReloadOutlined />
          </template>
        </AButton>
      </template>
    </AAlert>
    <div ref="terminalLayoutRef" class="terminal-layout">
      <div class="terminal-container">
        <div class="terminal-header">
          <div class="header-actions">
            <AButton
              type="text"
              size="small"
              @click="toggleRightPanel"
            >
              {{ isRightPanelVisible ? $gettext('Hide Assistant') : $gettext('Show Assistant') }}
            </AButton>
          </div>
        </div>
        <div
          id="terminal"
          class="console"
        />
        <TerminalStatusBar />
      </div>

      <TerminalRightPanel
        ref="rightPanelRef"
        :is-visible="isRightPanelVisible"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
.terminal-layout {
  display: flex;
  min-height: max(585px, calc(100vh - 200px));
  border-radius: 5px;
  overflow: hidden;
  background: #000;
  position: relative;
  width: 100%;

  @media (max-width: 512px) {
    border-radius: 0;
  }
}

.terminal-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  transition: all 0.3s ease;
  background: #000;
}

.terminal-header {
  background: #1a1a1a;
  border-bottom: 1px solid #333;
  padding: 8px 12px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  min-height: 40px;

  .header-actions {
    display: flex;
    gap: 8px;
    align-items: center;

    .icon {
      font-size: 16px;
    }
  }

  :deep(.ant-btn) {
    color: #e0e0e0;
    border: 1px solid #444;
    background: transparent;

    &:hover {
      color: #4a9eff;
      border-color: #4a9eff;
      background: rgba(74, 158, 255, 0.1);
    }
  }
}

.console {
  flex: 1;

  :deep(.terminal) {
    padding: 10px;
    height: 100%;
  }

  :deep(.xterm-viewport) {
    border-radius: 0;
  }
}

@media (max-width: 768px) {
  .terminal-header {
    padding: 6px 8px;
    min-height: 36px;

    :deep(.ant-btn) {
      font-size: 12px;
      padding: 4px 8px;
    }
  }
}
</style>
