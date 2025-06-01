<script setup lang="ts">
import BaseEditor from '@/components/BaseEditor'
import ConfigLeftPanel from '@/views/config/components/ConfigLeftPanel.vue'
import ConfigRightPanel from '@/views/config/components/ConfigRightPanel.vue'

// Use Vue 3.4+ useTemplateRef to get reference to left panel
const leftPanelRef = useTemplateRef<InstanceType<typeof ConfigLeftPanel>>('leftPanel')
</script>

<template>
  <BaseEditor :loading="leftPanelRef?.loading">
    <template #left>
      <ConfigLeftPanel ref="leftPanel" />
    </template>

    <template #right>
      <ConfigRightPanel
        v-if="leftPanelRef"
        v-model:data="leftPanelRef.data"
        :add-mode="leftPanelRef.addMode || false"
        :new-path="leftPanelRef.newPath || ''"
        :modified-at="leftPanelRef.modifiedAt || ''"
        :orig-name="leftPanelRef.origName || ''"
      />
    </template>
  </BaseEditor>
</template>

<style lang="less" scoped>
.col-right {
  position: sticky;
  top: 78px;

  :deep(.ant-card-body) {
    max-height: 100vh;
    overflow-y: scroll;
  }
}

:deep(.ant-collapse-ghost > .ant-collapse-item > .ant-collapse-content > .ant-collapse-content-box) {
  padding: 0;
}

:deep(.ant-collapse > .ant-collapse-item > .ant-collapse-header) {
  padding: 0 0 10px 0;
}

.overwrite {
  margin-right: 15px;

  span {
    color: #9b9b9b;
  }
}

.node-deploy-control {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
  align-items: center;
}

:deep(.ant-card-body) {
  padding: 0;
}
</style>
