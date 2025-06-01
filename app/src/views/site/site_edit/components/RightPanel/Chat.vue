<script setup lang="ts">
import ChatGPT from '@/components/ChatGPT'
import { useSiteEditorStore } from '../SiteEditor/store'

const editorStore = useSiteEditorStore()
const {
  configText,
  filepath,
} = storeToRefs(editorStore)
</script>

<template>
  <div class="mt--4">
    <ChatGPT
      :content="configText"
      :path="filepath"
    />
  </div>
</template>

<style scoped lang="less">
:deep(.ant-collapse-ghost > .ant-collapse-item > .ant-collapse-content > .ant-collapse-content-box) {
  padding: 0;
}

:deep(.ant-collapse > .ant-collapse-item > .ant-collapse-header) {
  padding: 0 0 10px 0;
}
</style>
