<script setup lang="ts">
import { useElementSize } from '@vueuse/core'
import { PortScannerCompact } from '@/components/PortScanner'
import { useSiteEditorStore } from '../SiteEditor/store'
import Basic from './Basic.vue'
import Chat from './Chat.vue'
import ConfigTemplate from './ConfigTemplate.vue'
import DNS from './DNS.vue'

const activeKey = ref('basic')

const editorStore = useSiteEditorStore()
const { advanceMode, loading } = storeToRefs(editorStore)

// Get container height for Chat component
const containerRef = ref<HTMLElement>()
const { height: containerHeight } = useElementSize(containerRef)

// Calculate chat height
const chatHeight = computed(() => {
  const tabsNavHeight = 55
  const padding = 48
  return `${containerHeight.value - tabsNavHeight - padding}px`
})

watch(advanceMode, val => {
  if (val) {
    activeKey.value = 'basic'
  }
})
</script>

<template>
  <div ref="containerRef" class="right-settings-container">
    <ACard
      class="right-settings"
      variant="borderless"
      :loading
      :styles="{ body: { padding: 0 } }"
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
            key: 'dns',
            label: $gettext('DNS'),
          },
          ...(!advanceMode
            ? [{
              key: 'config-template',
              label: $gettext('Config Template'),
            }]
            : []),
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
            padding: '0 24px',
            height: '55px',
          },
          content: {
            paddingTop: '24px',
            overflowY: 'auto',
          },
        }"
      >
        <template #contentRender="{ item }">
          <Basic v-if="item.key === 'basic'" />
          <DNS v-else-if="item.key === 'dns'" />
          <ConfigTemplate v-else-if="item.key === 'config-template'" />
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
