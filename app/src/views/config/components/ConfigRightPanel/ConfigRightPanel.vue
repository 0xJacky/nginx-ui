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
      variant="borderless"
      :styles="{ root: { boxShadow: 'unset' } }"
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
            maxHeight: 'calc(100vh - 260px)',
          },
        }"
      >
        <template #contentRender="{ item }">
          <Basic
            v-if="item.key === 'basic'"
            v-model:data="data"
            :add-mode
            :new-path
            :modified-at
            :orig-name
          />
          <Chat
            v-else
            v-model:data="data"
            :chat-height
          />
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
