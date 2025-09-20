<script setup lang="ts">
import type { Config } from '@/api/config'
import { useElementSize } from '@vueuse/core'
import Basic from './Basic.vue'
import Chat from './Chat.vue'

interface ConfigRightPanelProps {
  addMode: boolean
  newPath: string
  modifiedAt: string
  origName: string
}

defineProps<ConfigRightPanelProps>()
const data = defineModel<Config>('data', { required: true })

const activeKey = ref('basic')

// Get container height for Chat component
const containerRef = ref<HTMLElement>()
const { height: containerHeight } = useElementSize(containerRef)

// Calculate chat height (container height - tabs nav height - padding)
const chatHeight = computed(() => {
  const tabsNavHeight = 55
  const padding = 48 // top and bottom padding
  return `${containerHeight.value - tabsNavHeight - padding}px`
})
</script>

<template>
  <div ref="containerRef" class="right-settings-container">
    <ACard
      class="right-settings"
      :bordered="false"
    >
      <ATabs
        v-model:active-key="activeKey"
        size="small"
      >
        <ATabPane key="basic" :tab="$gettext('Basic')">
          <Basic
            v-model:data="data"
            :add-mode
            :new-path
            :modified-at
            :orig-name
          />
        </ATabPane>
        <ATabPane key="chat" :tab="$gettext('Chat')">
          <Chat v-model:data="data" :chat-height />
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

  :deep(.ant-tabs-nav) {
    margin: 0;
    height: 55px;
    padding: 0 24px;
  }
}

:deep(.ant-tabs-content) {
  padding-top: 24px;
  overflow-y: auto;
}

:deep(.ant-card) {
  box-shadow: unset;

  .ant-tabs-content {
    max-height: calc(100vh - 260px);
  }
}
</style>
