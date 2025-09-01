<script setup lang="ts">
import ChatMessage from './ChatMessage.vue'
import { useLLMStore } from './llm'

// Use LLM store
const llmStore = useLLMStore()
const { messages, editingIdx, editValue, loading } = storeToRefs(llmStore)

function handleEdit(index: number) {
  llmStore.startEdit(index)
}

async function handleSave() {
  llmStore.saveEdit()
  await nextTick()
  llmStore.request()
}

function handleCancel() {
  llmStore.cancelEdit()
}

async function handleRegenerate(index: number) {
  llmStore.regenerate(index)
}
</script>

<template>
  <div class="message-list-container">
    <AList
      class="llm-log"
      item-layout="horizontal"
      :data-source="messages"
    >
      <template #renderItem="{ item, index }">
        <ChatMessage
          :edit-value="editValue"
          :message="item"
          :index="index"
          :is-editing="editingIdx === index"
          :loading="loading"
          @edit="handleEdit"
          @save="handleSave"
          @cancel="handleCancel"
          @regenerate="handleRegenerate"
        />
      </template>
    </AList>
  </div>
</template>

<style lang="less" scoped>
.message-list-container {
  overflow-y: auto;
  height: 100%;

  .llm-log {
    :deep(.ant-list-item) {
      padding: 0 12px;
    }

    :deep(.ant-comment-content) {
      width: 100%;
    }

    :deep(.ant-comment) {
      width: 100%;
    }

    :deep(.ant-comment-content-detail) {
      width: 100%;

      p {
        margin-bottom: 10px;
      }
    }

    :deep(.ant-list-item:first-child) {
      display: none;
    }
  }
}
</style>
