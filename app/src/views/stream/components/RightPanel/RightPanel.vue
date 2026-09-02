<script setup lang="ts">
import { useElementSize } from '@vueuse/core'
import { PortScannerCompact } from '@/components/PortScanner'
import Basic from './Basic.vue'
import Chat from './Chat.vue'

const activeKey = ref('basic')

// Get container height for Chat component
const containerRef = ref<HTMLElement>()
const { height: containerHeight } = useElementSize(containerRef)

// Calculate chat height
const chatHeight = computed(() => {
  const tabsNavHeight = 55
  const padding = 48
  return `${containerHeight.value - tabsNavHeight - padding}px`
})
</script>

<template>
  <div ref="containerRef" class="right-settings-container">
    <ACard
      class="right-settings"
      variant="borderless"
    >
      <ATabs
        v-model:active-key="activeKey"
        size="small"
        :items="[
          {
            key: 'basic',
            label: $gettext('Basic'),
          },
          {
            key: 'chat',
            label: $gettext('Chat'),
          },
          {
            key: 'port-scanner',
            label: $gettext('Port Scanner'),
          },
        ]"
        :styles="{
          header: {
            margin: '0',
            height: '55px',
            padding: '0 24px',
          },
          content: {
            paddingTop: '24px',
            overflowY: 'auto',
          },
        }"
      >
        <template #contentRender="{ item }">
          <Basic v-if="item.key === 'basic'" />
          <Chat
            v-else-if="item.key === 'chat'"
            :chat-height="chatHeight"
          />
          <PortScannerCompact v-else />
        </template>
      </ATabs>
    </ACard>
  </div>
</template>

<style scoped lang="less">
.right-settings-container {
  position: relative;

  .right-settings {
    position: relative;
  }
}
</style>
