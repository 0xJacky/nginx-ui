<script setup lang="ts">
import { useChatGPTStore } from './chatgpt'
import ChatMessage from './ChatMessage.vue'

// Use ChatGPT store
const chatGPTStore = useChatGPTStore()
const { messages, editingIdx, editValue, loading } = storeToRefs(chatGPTStore)

const messageListRef = useTemplateRef('messageList')
let scrollTimeoutId: number | null = null

function scrollToBottom() {
  // Debounce scroll operations for better performance
  if (scrollTimeoutId) {
    clearTimeout(scrollTimeoutId)
  }

  scrollTimeoutId = window.setTimeout(() => {
    requestAnimationFrame(() => {
      if (messageListRef.value) {
        let element = messageListRef.value.parentElement
        while (element) {
          const style = window.getComputedStyle(element)
          if (style.overflowY === 'auto' || style.overflowY === 'scroll') {
            element.scrollTo({
              top: element.scrollHeight,
              behavior: 'smooth',
            })
            return
          }
          element = element.parentElement
        }
      }
    })
  }, 50) // 50ms debounce
}

// Watch for messages changes and auto scroll - with debouncing
watch(() => messages.value, () => {
  scrollToBottom()
}, { deep: true, flush: 'post' })

// Auto scroll when messages are loaded
onMounted(() => {
  scrollToBottom()
})

// Clean up on unmount
onUnmounted(() => {
  if (scrollTimeoutId) {
    clearTimeout(scrollTimeoutId)
  }
})

// Expose scroll function for parent component
defineExpose({
  scrollToBottom,
})

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
  <div ref="messageList" class="message-list-container">
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
