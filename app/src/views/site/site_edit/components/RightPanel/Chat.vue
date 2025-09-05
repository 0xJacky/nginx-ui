<script setup lang="ts">
import LLM from '@/components/LLM'
import { useSiteEditorStore } from '../SiteEditor/store'

interface Props {
  chatHeight: string
}

defineProps<Props>()
const editorStore = useSiteEditorStore()
const {
  configText,
  filepath,
} = storeToRefs(editorStore)
</script>

<template>
  <div class="mt--6">
    <LLM
      :nginx-config="configText"
      :path="filepath"
      :height="chatHeight"
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

// LLM组件高度覆盖
:deep(.llm-wrapper) {
  height: calc(100vh - 260px);
}

// LLM组件滚动设置 - 与 TerminalRightPanel 保持一致
:deep(.llm-container) {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  position: relative; // 确保定位上下文
}

:deep(.session-header) {
  flex-shrink: 0;
}

:deep(.message-container) {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0; // 重要：允许 flex 子元素缩小
  position: relative; // 为绝对定位提供参考点
}

// 消息列表容器可滚动，为输入框预留空间
:deep(.message-list-container) {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

// 输入框绝对定位在底部
:deep(.input-msg) {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 10;
}
</style>
