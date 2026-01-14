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
      :bordered="false"
      :loading
    >
      <ATabs
        v-model:active-key="activeKey"
        size="small"
      >
        <ATabPane key="basic" :tab="$gettext('Basic')">
          <Basic />
        </ATabPane>
        <ATabPane key="dns" :tab="$gettext('DNS')">
          <DNS />
        </ATabPane>
        <ATabPane
          v-if="!advanceMode"
          key="config-template"
          :tab="$gettext('Config Template')"
        >
          <ConfigTemplate />
        </ATabPane>
        <ATabPane key="chat" :tab="$gettext('Chat')">
          <Chat :chat-height="chatHeight" />
        </ATabPane>
        <ATabPane key="port-scanner" :tab="$gettext('Port Scanner')">
          <PortScannerCompact />
        </ATabPane>
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

  :deep(.ant-card-body) {
    padding: 0;
  }

  :deep(.ant-tabs-nav) {
    margin: 0;
    padding: 0 24px;
    height: 55px;
  }
}

:deep(.ant-tabs-content) {
  padding-top: 24px;
  overflow-y: auto;
}
</style>
