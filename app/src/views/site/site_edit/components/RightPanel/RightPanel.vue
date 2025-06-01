<script setup lang="ts">
import { PortScannerCompact } from '@/components/PortScanner'
import { useSiteEditorStore } from '../SiteEditor/store'
import Basic from './Basic.vue'
import Chat from './Chat.vue'
import ConfigTemplate from './ConfigTemplate.vue'

const activeKey = ref('basic')

const editorStore = useSiteEditorStore()
const { advanceMode, loading } = storeToRefs(editorStore)

watch(advanceMode, val => {
  if (val) {
    activeKey.value = 'basic'
  }
})
</script>

<template>
  <div class="right-settings-container">
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
        <ATabPane
          v-if="!advanceMode"
          key="config-template"
          :tab="$gettext('Config Template')"
        >
          <ConfigTemplate />
        </ATabPane>
        <ATabPane key="chat" :tab="$gettext('Chat')">
          <Chat />
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
