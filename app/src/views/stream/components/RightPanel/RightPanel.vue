<script setup lang="ts">
import Basic from './Basic.vue'
import Chat from './Chat.vue'

const activeKey = ref('basic')
</script>

<template>
  <div class="right-settings-container">
    <ACard
      class="right-settings"
      :bordered="false"
    >
      <ATabs
        v-model:active-key="activeKey"
        class="mb-24px"
        size="small"
      >
        <ATabPane key="basic" :tab="$gettext('Basic')">
          <Basic />
        </ATabPane>
        <ATabPane key="chat" :tab="$gettext('Chat')">
          <Chat />
        </ATabPane>
      </ATabs>
    </ACard>
  </div>
</template>

<style scoped lang="less">
.right-settings-container {
  position: relative;

  .right-settings {
    max-height: calc(100vh - 323px);
    overflow-y: scroll;
    position: relative;
  }

  :deep(.ant-card-body) {
    padding: 19.5px 24px;
  }

  :deep(.ant-tabs-nav) {
    margin: 0;
  }
}

:deep(.ant-tabs-content) {
  padding-top: 24px;
  max-height: calc(100vh - 425px);
  overflow-y: scroll;
}
</style>
