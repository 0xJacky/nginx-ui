<script setup lang="ts">
import { useSettingsStore } from '@/pinia'
import ChatMessage from './ChatMessage.vue'
import { useLLMStore } from './llm'

// Props
const props = defineProps<{
  inputHeight?: number
  osInfo?: string
}>()

// Use LLM store
const llmStore = useLLMStore()
const { messages, editingIdx, editValue, loading } = storeToRefs(llmStore)

// Get current language
const { language: currentLanguage } = storeToRefs(useSettingsStore())

// Message list container ref for scrolling
const messageListContainer = ref<HTMLElement>()

// Track user scroll state
const userScrolledUp = ref(false)
const isAutoScrolling = ref(false)
const lastScrollTop = ref(0)
const scrollDirection = ref<'up' | 'down' | 'none'>('none')

// Check if user is at bottom
function isAtBottom() {
  if (!messageListContainer.value)
    return false
  const container = messageListContainer.value
  const threshold = 10 // Allow larger margin for padding/border issues
  const distanceFromBottom = container.scrollHeight - container.scrollTop - container.clientHeight

  return distanceFromBottom <= threshold
}

// Scroll to bottom method
function scrollToBottom() {
  if (messageListContainer.value) {
    isAutoScrolling.value = true

    // Scroll to absolute bottom to account for padding
    const container = messageListContainer.value
    const targetScrollTop = container.scrollHeight - container.clientHeight
    container.scrollTop = targetScrollTop

    // Update tracking values
    lastScrollTop.value = targetScrollTop
    userScrolledUp.value = false

    setTimeout(() => {
      isAutoScrolling.value = false
    }, 100)
  }
}

// Handle scroll events to detect user scrolling
function handleScroll() {
  if (isAutoScrolling.value)
    return

  const container = messageListContainer.value
  if (!container)
    return

  const currentScrollTop = container.scrollTop
  const previousScrollTop = lastScrollTop.value

  // Determine scroll direction
  if (currentScrollTop > previousScrollTop) {
    scrollDirection.value = 'down'
  }
  else if (currentScrollTop < previousScrollTop) {
    scrollDirection.value = 'up'
  }
  else {
    scrollDirection.value = 'none'
  }

  // Only mark as user scrolled up if they actively scrolled up AND are not at bottom
  const wasAtBottom = isAtBottom()
  if (scrollDirection.value === 'up' && !wasAtBottom) {
    userScrolledUp.value = true
  }
  else if (wasAtBottom) {
    // If user is at bottom, allow auto scroll
    userScrolledUp.value = false
  }

  lastScrollTop.value = currentScrollTop
}

// Auto-scroll only if user hasn't scrolled up
function autoScrollToBottom() {
  if (!userScrolledUp.value) {
    scrollToBottom()
  }
}

// Setup scroll listener
onMounted(() => {
  nextTick(() => {
    if (messageListContainer.value) {
      messageListContainer.value.addEventListener('scroll', handleScroll, { passive: true })
    }
  })
})

onUnmounted(() => {
  if (messageListContainer.value) {
    messageListContainer.value.removeEventListener('scroll', handleScroll)
  }
})

// Reset scroll state (call when new messages arrive)
function resetScrollState() {
  userScrolledUp.value = false
}

// Expose scroll method
defineExpose({
  scrollToBottom: autoScrollToBottom,
  resetScrollState,
})

function handleEdit(index: number) {
  llmStore.startEdit(index)
}

async function handleSave(index: number) {
  llmStore.saveEdit()
  await nextTick()
  llmStore.regenerate(index, currentLanguage.value, props.osInfo)
}

function handleCancel() {
  llmStore.cancelEdit()
}

async function handleRegenerate(index: number) {
  llmStore.regenerate(index, currentLanguage.value, props.osInfo)
}
</script>

<template>
  <div
    ref="messageListContainer"
    class="message-list-container"
    :style="{ paddingBottom: props.inputHeight ? `${props.inputHeight + 32}px` : '32px' }"
  >
    <AList
      class="llm-log pt-12"
      item-layout="horizontal"
    >
      <ChatMessage
        v-for="(item, index) in messages"
        :key="index"
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
    </AList>
  </div>
</template>

<style lang="less" scoped>
.message-list-container {
  width: 100%;
  overflow-y: auto;
  overflow-x: hidden;

  :deep(.ant-list-empty-text) {
    display: none;
  }

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

  }
}
</style>
