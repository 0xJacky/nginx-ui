<script setup lang="ts">
import type { TerminalTab } from '@/pinia/moudule/terminal'
import { ReloadOutlined } from '@ant-design/icons-vue'

interface Props {
  tab: TerminalTab
  isActive: boolean
  lostConnection: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  refresh: []
}>()

function getTerminalContainerId(tabId: string): string {
  return `container-${tabId}`
}

function handleRefresh() {
  emit('refresh')
}
</script>

<template>
  <div
    class="terminal-session"
    :class="{ active: isActive }"
  >
    <!-- Session-specific connection alert -->
    <AAlert
      v-if="lostConnection"
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
          @click="handleRefresh"
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
</template>

<style lang="less" scoped>
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
</style>
