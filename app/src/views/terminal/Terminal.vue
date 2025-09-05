<script setup lang="ts">
import type { TerminalSessionCallbacks } from '@/composables/useTerminalSession'
import { CloseOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { theme } from 'ant-design-vue'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useTerminalSession } from '@/composables/useTerminalSession'
import { useTerminalStore } from '@/pinia'
import TerminalRightPanel from './components/TerminalRightPanel.vue'
import TerminalStatusBar from './components/TerminalStatusBar.vue'
import '@xterm/xterm/css/xterm.css'

const terminalStore = useTerminalStore()
const otpModal = use2FAModal()
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
    const secureSessionId = await otpModal.open()

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
    const secureSessionId = await otpModal.open()
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
          <div class="terminal-header">
            <div class="terminal-tabs">
              <div class="tabs-scroll">
                <div
                  v-for="tab in terminalStore.tabs"
                  :key="tab.id"
                  class="terminal-tab"
                  :class="{ active: tab.id === terminalStore.activeTabId }"
                  @click="switchTab(tab.id)"
                >
                  <span class="tab-name">{{ tab.name }}</span>
                  <CloseOutlined
                    v-if="terminalStore.tabs.length > 1"
                    class="tab-close"
                    @click.stop="closeTab(tab.id)"
                  />
                </div>
              </div>

              <div class="tab-actions-group">
                <AButton
                  type="text"
                  size="small"
                  class="add-tab-btn"
                  @click="createNewTerminal"
                >
                  <template #icon>
                    <PlusOutlined />
                  </template>
                </AButton>
              </div>
            </div>
            <div class="header-actions">
              <AButton
                type="text"
                size="small"
                @click="toggleRightPanel"
              >
                {{ terminalStore.llm_panel_visible ? $gettext('Hide Assistant') : $gettext('Show Assistant') }}
              </AButton>
            </div>
          </div>
          <div class="terminals-container">
            <div
              v-for="tab in terminalStore.tabs"
              :key="tab.id"
              class="terminal-session"
              :class="{ active: tab.id === terminalStore.activeTabId }"
            >
              <!-- Session-specific connection alert -->
              <AAlert
                v-if="getSessionConnectionStatus(tab.id).lostConnection"
                class="session-alert"
                type="error"
                show-icon
                size="small"
                :message="$gettext('Connection lost for this terminal. Please refresh if needed.')"
                banner
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

              <!-- Terminal container -->
              <div
                :id="getTerminalContainerId(tab.id)"
                class="console"
              />
            </div>
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

.terminal-header {
  background: rgba(30, 30, 30, 0.8);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  border-bottom: 1px solid #333;
  box-shadow: 0 0 10px rgba(0, 0, 0, 0.04);
  display: flex;
  justify-content: space-between;

  .terminal-tabs {
    flex: 1;
    display: flex;
    align-items: center;
    height: 47px;
    padding: 0 12px;
    width: 100%;
    box-sizing: border-box;

    .tabs-scroll {
      flex: 1;
      display: flex;
      overflow-x: auto;
      overflow-y: hidden;
      gap: 0;
      min-width: 0;
      background: transparent;
      position: relative;

      &::-webkit-scrollbar {
        height: 0;
      }
    }

    .terminal-tab {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      padding: 8px 8px;
      cursor: pointer;
      transition: all 0.15s ease;
      background: transparent;
      max-width: 120px;
      min-width: 80px;
      position: relative;
      box-sizing: border-box;
      border-radius: 6px;

      &:hover:not(.active) {
        .tab-name {
          color: rgba(255, 255, 255, 0.9);
        }

        .tab-close {
          opacity: 1;
          transform: scale(1);
        }
      }

      &.active {
        color: #ffffff;
        z-index: 2;
        position: relative;

        .tab-name {
          font-weight: 500;
          color: #ffffff;
        }

        .tab-close {
          opacity: 1;
          transform: scale(1);
        }
      }

      .tab-name {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: 13px;
        color: rgba(255, 255, 255, 0.7);
        transition: color 0.15s ease;
      }

      .tab-close {
        width: 22px;
        height: 22px;
        padding: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 4px;
        border: none;
        background: transparent;
        color: rgba(255, 255, 255, 0.6);
        margin-left: 4px;
        opacity: 0;
        transform: scale(0.8);
        transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);

        &:hover {
          background: rgba(239, 68, 68, 0.2);
          color: #ef4444;
          transform: scale(1);
        }

        :deep(.anticon) {
          font-size: 12px;
        }
      }
    }
  }

  .tab-actions-group {
    flex-shrink: 0;
    display: flex;
    align-items: center;

    .add-tab-btn {
      width: 24px;
      height: 24px;
      padding: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      border: none;
      background: transparent;
      color: rgba(255, 255, 255, 0.6);
      transition: color 0.15s ease;

      &:hover:not(:disabled) {
        color: rgba(255, 255, 255, 0.9);
      }

      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      :deep(.anticon) {
        font-size: 12px;
      }
    }
  }

  .header-actions {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 0 12px 0 0;

    .icon {
      font-size: 16px;
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
}

.terminals-container {
  flex: 1;
  position: relative;
  height: calc(100% - 48px - 40px);
  overflow: hidden;
}

.terminal-session {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: none;
  flex-direction: column;
  overflow: hidden;

  &.active {
    display: flex;
  }
}

.console {
  flex: 1;
  width: 100%;
  overflow: hidden;

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
    min-height: 44px;

    :deep(.ant-btn) {
      font-size: 12px;
      padding: 4px 8px;
    }
  }
}
</style>
