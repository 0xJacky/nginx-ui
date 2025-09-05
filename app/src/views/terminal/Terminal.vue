<script setup lang="ts">
import type { TerminalSessionCallbacks } from '@/composables/useTerminalSession'
import { theme } from 'ant-design-vue'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useTerminalSession } from '@/composables/useTerminalSession'
import { useTerminalStore } from '@/pinia'
import TerminalHeader from './components/TerminalHeader.vue'
import TerminalRightPanel from './components/TerminalRightPanel.vue'
import TerminalSessionContent from './components/TerminalSessionContent.vue'
import TerminalStatusBar from './components/TerminalStatusBar.vue'
import '@xterm/xterm/css/xterm.css'

const terminalStore = useTerminalStore()
const { open: openOtpModal, contextHolder } = use2FAModal()
const {
  createSession,
  destroySession,
  focusSession,
  resizeAllSessions,
  getSessionConnectionStatus,
} = useTerminalSession()

// Create theme config for AConfigProvider
const terminalTheme = computed(() => {
  return {
    algorithm: theme.darkAlgorithm,
  }
})

const insecureConnection = ref(false)
const rightPanelRef = ref<InstanceType<typeof TerminalRightPanel>>()
const terminalLayoutRef = ref<HTMLElement>()

function checkSecureConnection() {
  const hostname = window.location.hostname
  const protocol = window.location.protocol

  if ((hostname !== 'localhost' && hostname !== '127.0.0.1') && protocol !== 'https:') {
    insecureConnection.value = true
  }
}

const sessionCallbacks: TerminalSessionCallbacks = {
  onInput: (_tabId: string, data: string) => {
    if (rightPanelRef.value && data.includes('\r')) {
      const command = data.replace('\r', '').trim()
      if (command) {
        rightPanelRef.value.updateCurrentCommand(command)
      }
    }
  },
  onConnectionLost: (_tabId: string) => {
    // Connection status is now managed within the session itself
  },
  onConnectionReady: (_tabId: string) => {
    // Connection status is now managed within the session itself
  },
}

async function createNewTerminal() {
  const tab = terminalStore.createTab()

  try {
    const secureSessionId = await openOtpModal()

    // Wait for DOM to update before creating session
    await nextTick()

    await createSession(tab, getTerminalContainerId(tab.id), secureSessionId, sessionCallbacks)
    nextTick(() => {
      focusSession(tab.id)
    })
  }
  catch (error) {
    console.error('Failed to create terminal session:', error)
    terminalStore.closeTab(tab.id)
  }
}

function getTerminalContainerId(tabId: string): string {
  return `container-${tabId}`
}

function switchTab(tabId: string) {
  terminalStore.setActiveTab(tabId)
  nextTick(() => {
    focusSession(tabId)
  })
}

function closeTab(tabId: string) {
  destroySession(tabId)
  terminalStore.closeTab(tabId)
}

onMounted(() => {
  checkSecureConnection()

  if (!terminalStore.hasActiveTabs) {
    createNewTerminal()
  }
})

onUnmounted(() => {
  terminalStore.tabs.forEach(tab => {
    destroySession(tab.id)
  })
})

async function refreshTerminal() {
  // Get the current active tab
  const activeTab = terminalStore.activeTab
  if (!activeTab)
    return

  try {
    // Close the current session
    destroySession(activeTab.id)

    // Recreate the session
    const secureSessionId = await openOtpModal()
    await nextTick()

    await createSession(activeTab, getTerminalContainerId(activeTab.id), secureSessionId, sessionCallbacks)
    nextTick(() => {
      focusSession(activeTab.id)
    })
  }
  catch (error) {
    console.error('Failed to refresh terminal session:', error)
  }
}

watch(() => terminalStore.activeTabId, (newTabId, oldTabId) => {
  if (oldTabId && newTabId !== oldTabId) {
    nextTick(() => {
      if (newTabId) {
        focusSession(newTabId)
      }
    })
  }
})

function toggleRightPanel() {
  terminalStore.toggle_llm_panel()
  nextTick(() => {
    setTimeout(() => {
      resizeAllSessions()
    }, 300)
  })
}
</script>

<template>
  <div>
    <component :is="contextHolder" />
    <AConfigProvider :theme="terminalTheme">
      <AAlert
        v-if="insecureConnection"
        class="mb-6"
        type="warning"
        show-icon
        :message="$gettext('You are accessing this terminal over an insecure HTTP connection on a non-localhost domain. This may expose sensitive information.')"
      />
      <div ref="terminalLayoutRef" class="terminal-layout">
        <div class="terminal-container">
          <TerminalHeader
            :tabs="terminalStore.tabs"
            :active-tab-id="terminalStore.activeTabId"
            :llm-panel-visible="terminalStore.llm_panel_visible"
            @switch-tab="switchTab"
            @close-tab="closeTab"
            @create-new-terminal="createNewTerminal"
            @toggle-right-panel="toggleRightPanel"
          />
          <div class="terminals-container">
            <TerminalSessionContent
              v-for="tab in terminalStore.tabs"
              :key="tab.id"
              :tab="tab"
              :is-active="tab.id === terminalStore.activeTabId"
              :lost-connection="getSessionConnectionStatus(tab.id).lostConnection"
              @refresh="refreshTerminal"
            />
          </div>
          <TerminalStatusBar />
        </div>

        <TerminalRightPanel
          ref="rightPanelRef"
          :is-visible="terminalStore.llm_panel_visible"
        />
      </div>
    </AConfigProvider>
  </div>
</template>

<style lang="less" scoped>
.terminal-layout {
  display: flex;
  height: max(600px, calc(100vh - 200px));
  border: 1px solid #333;
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

.terminals-container {
  flex: 1;
  position: relative;
  height: calc(100% - 48px - 40px);
  overflow: hidden;
}
</style>
