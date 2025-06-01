<script setup lang="ts">
import { useChatGPTStore } from './chatgpt'
import ChatMessage from './ChatMessage.vue'

// Use ChatGPT store
const chatGPTStore = useChatGPTStore()
const { messages, editingIdx, editValue, loading } = storeToRefs(chatGPTStore)

function handleEdit(index: number) {
  chatGPTStore.startEdit(index)
}

async function handleSave() {
  chatGPTStore.saveEdit()
  await nextTick()
  chatGPTStore.request()
}

function handleCancel() {
  chatGPTStore.cancelEdit()
}

async function handleRegenerate(index: number) {
  chatGPTStore.regenerate(index)
}
</script>

<template>
  <div class="message-list-container">
    <AList
      class="chatgpt-log"
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

  .chatgpt-log {
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
