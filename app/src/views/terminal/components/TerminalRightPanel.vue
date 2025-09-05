<script setup lang="ts">
import type { AnalyticInit } from '@/api/analytic'
import analytic from '@/api/analytic'
import LLM from '@/components/LLM/LLM.vue'
import { useTerminalStore } from '@/pinia'

interface Props {
  isVisible: boolean
}

defineProps<Props>()

const terminalStore = useTerminalStore()

// Get current terminal command context per tab
const tabCommands = ref<Map<string, string>>(new Map())
const systemInfo = ref<AnalyticInit | null>(null)

// No longer need to calculate height since we have CSS min-height

// Fetch system information
async function fetchSystemInfo() {
  try {
    systemInfo.value = await analytic.init()
  }
  catch (error) {
    console.error('Failed to fetch system info:', error)
  }
}

// Build OS information string
const osInfoString = computed(() => {
  if (!systemInfo.value?.host) {
    return ''
  }

  const host = systemInfo.value.host
  const parts: string[] = []

  if (host.os)
    parts.push(`OS: ${host.os}`)
  if (host.platform)
    parts.push(`Platform: ${host.platform}`)
  if (host.platformVersion)
    parts.push(`Version: ${host.platformVersion}`)
  if (host.kernelVersion)
    parts.push(`Kernel: ${host.kernelVersion}`)
  if (host.kernelArch)
    parts.push(`Architecture: ${host.kernelArch}`)

  return parts.join(', ')
})

// Build terminal context with system information
const terminalContext = computed(() => {
  let context = ''

  if (systemInfo.value?.host?.platformVersion) {
    context += `System: ${systemInfo.value.host.platformVersion}\n\n`
  }

  const activeTabId = terminalStore.activeTabId
  const currentCommand = activeTabId ? tabCommands.value.get(activeTabId) || '' : ''

  if (activeTabId && terminalStore.activeTab) {
    context += `Active terminal tab: ${terminalStore.activeTab.name}\n`
  }

  if (currentCommand) {
    context += `Current terminal command: ${currentCommand}\n\n`
    context += 'Please help me with this command or terminal operation.'
  }
  else {
    context += 'I need assistance with terminal operations and commands.'
  }

  return context
})

// Initialize system info when component mounts
onMounted(() => {
  fetchSystemInfo()
})

// Function to update command for active tab
function updateCurrentCommand(command: string) {
  const activeTabId = terminalStore.activeTabId
  if (activeTabId) {
    tabCommands.value.set(activeTabId, command)
  }
}

defineExpose({
  updateCurrentCommand,
})
</script>

<template>
  <div
    v-if="isVisible"
    class="terminal-right-panel dark"
  >
    <div v-if="isVisible" class="panel-content">
      <LLM
        :content="terminalContext"
        type="terminal"
        theme="dark"
        :os-info="osInfoString"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
.terminal-right-panel {
  width: 400px;
  min-height: calc(100vh - 200px);
  border-left: 1px solid #333;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;

  @media (max-width: 1200px) {
    width: 350px;
  }

  @media (max-width: 992px) {
    width: 300px;
  }

  @media (max-width: 768px) {
    position: fixed;
    right: 0;
    top: 0;
    width: 100%;
    height: 100vh;
    z-index: 1000;
    border-left: none;
  }
}

.panel-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;

  :deep(.llm-container) {
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100%;
    position: relative; // 确保定位上下文
  }

  :deep(.session-header) {
    flex-shrink: 0;
  }

  :deep(.message-container) {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0; // 重要：允许 flex 子元素缩小
    position: relative; // 为绝对定位提供参考点
  }

  // 消息列表容器可滚动，为输入框预留空间
  :deep(.message-list-container) {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }

  // 输入框绝对定位在底部
  :deep(.input-msg) {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    background: rgba(30, 30, 30, 0.95);
    backdrop-filter: blur(10px);
    z-index: 10;
  }
}
</style>
