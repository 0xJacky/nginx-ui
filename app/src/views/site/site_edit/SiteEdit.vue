<script setup lang="ts">
import BaseEditor from '@/components/BaseEditor'
import RightSettings from '@/views/site/site_edit/components/RightPanel/RightPanel.vue'
import SiteEditor from '@/views/site/site_edit/components/SiteEditor'
import { useSiteEditorStore } from './components/SiteEditor/store'

const editorStore = useSiteEditorStore()
const { loading } = storeToRefs(editorStore)
</script>

<template>
  <div class="site-container">
    <BaseEditor :loading>
      <template #left>
        <SiteEditor />
      </template>

      <template #right>
        <RightSettings />
      </template>
    </BaseEditor>
  </div>
</template>

<style lang="less" scoped>
:deep(.ant-card) {
  box-shadow: unset;

  .ant-tabs-content {
    max-height: calc(100vh - 260px);
  }
}
</style>
