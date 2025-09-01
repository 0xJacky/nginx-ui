<script setup lang="ts">
import { useElementVisibility } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/pinia'
import ChatMessageInput from './ChatMessageInput.vue'
import ChatMessageList from './ChatMessageList.vue'
import { buildLLMContext } from './contextBuilder'
import { useLLMStore } from './llm'

const props = defineProps<{
  content: string
  path?: string
}>()

const { language: current } = storeToRefs(useSettingsStore())

// Use LLM store
const llmStore = useLLMStore()
const { messageContainerRef } = storeToRefs(llmStore)

// Initialize messages when path changes
watch(() => props.path, async () => {
  await llmStore.initMessages(props.path)
  await nextTick()

  // Auto-send first message if no messages exist
  if (llmStore.messages.length === 0) {
    await sendFirstMessage()
  }
  else {
    // Check if we need to enhance the first message with include context
    checkAndEnhanceFirstMessage()
  }
}, { immediate: true })

// Check if first message needs context enhancement
async function checkAndEnhanceFirstMessage() {
  if (llmStore.messages.length > 0 && props.path) {
    const firstMessage = llmStore.messages[0]
    // Check if the first message already contains included files info
    if (firstMessage.role === 'user' && !firstMessage.content.includes('--- INCLUDED FILES ---')) {
      try {
        // Build complete context including included files
        const context = await buildLLMContext(props.path, props.content)

        if (context.includedFiles.length > 0) {
          // Update the first message with enhanced context
          const enhancedContent = `${context.contextText}\n\nCurrent Language Code: ${current.value}`
          llmStore.messages[0].content = enhancedContent
          await llmStore.storeRecord()
        }
      }
      catch (error) {
        console.error('Failed to enhance first message:', error)
      }
    }
  }
}

// Build context and send first message
async function sendFirstMessage() {
  if (!props.path) {
    // If no path, use original content only
    await llmStore.send(props.content, current.value, props.content)
    return
  }

  try {
    // Build complete context including included files
    const context = await buildLLMContext(props.path, props.content)

    // Send with enhanced context
    await llmStore.send(props.content, current.value, context.contextText)
  }
  catch (error) {
    console.error('Failed to build enhanced context, falling back to original:', error)
    // Fallback to original behavior
    await llmStore.send(props.content, current.value, props.content)
  }
}

const isVisible = useElementVisibility(messageContainerRef)

watch(isVisible, visible => {
  if (visible) {
    llmStore.scrollToBottom()
  }
}, { immediate: true })
</script>

<template>
  <div
    ref="messageContainerRef"
    class="message-container"
  >
    <ChatMessageList />

    <ChatMessageInput />
  </div>
</template>

<style lang="less" scoped>
.message-container {
  margin: 0 auto;
  max-width: 800px;
  max-height: calc(100vh - 260px);
  overflow-y: auto;
}
</style>
