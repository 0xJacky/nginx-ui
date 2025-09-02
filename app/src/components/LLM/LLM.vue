<script setup lang="ts">
import { useElementVisibility } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/pinia'
import ChatMessageInput from './ChatMessageInput.vue'
import ChatMessageList from './ChatMessageList.vue'
import { buildLLMContext } from './contextBuilder'
import { useLLMStore } from './llm'
import LLMSessionTabs from './LLMSessionTabs.vue'
import { useLLMSessionStore } from './sessionStore'

const props = defineProps<{
  content: string
  path?: string
}>()

const { language: current } = storeToRefs(useSettingsStore())

// Use LLM store and session store
const llmStore = useLLMStore()
const sessionStore = useLLMSessionStore()
const { messageContainerRef } = storeToRefs(llmStore)
const { activeSessionId, sortedSessions } = storeToRefs(sessionStore)

// Initialize sessions and handle path changes
watch(() => props.path, async () => {
  // Load sessions for current path
  await sessionStore.loadSessions(props.path)

  // Check if we have sessions available
  if (sortedSessions.value.length > 0 && !activeSessionId.value) {
    // Use the most recent session
    const latestSession = sortedSessions.value[0]
    await llmStore.switchSession(latestSession.session_id)
    sessionStore.setActiveSession(latestSession.session_id)
  }
  else if (sortedSessions.value.length === 0) {
    // No sessions exist for this path, create a new one automatically
    const title = props.path ? `Chat for ${props.path.split('/').pop()}` : 'New Chat'
    try {
      const session = await sessionStore.createSession(title, props.path)
      await llmStore.switchSession(session.session_id)

      // Initialize with first message
      await nextTick()
      await sendFirstMessage()
    }
    catch (error) {
      console.error('Failed to create initial session:', error)
      // Fallback to legacy mode
      await llmStore.initMessages(props.path)
      await nextTick()
      if (llmStore.messages.length === 0) {
        await sendFirstMessage()
      }
    }
  }
}, { immediate: true })

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

// Handle new session creation
async function handleNewSessionCreated() {
  // Reload sessions to update the list
  await sessionStore.loadSessions(props.path)
  await nextTick()

  // Auto-send first message if no messages exist
  if (llmStore.messages.length === 0) {
    await sendFirstMessage()
  }
}

// Handle when all sessions are cleared
function handleSessionCleared() {
  // Reset to initial state - could create a welcome message or just stay empty
}

const isVisible = useElementVisibility(messageContainerRef)

watch(isVisible, visible => {
  if (visible) {
    llmStore.scrollToBottom()
  }
}, { immediate: true })
</script>

<template>
  <div class="llm-container">
    <div class="session-header">
      <LLMSessionTabs
        :content="props.content"
        :path="props.path"
        @new-session-created="handleNewSessionCreated"
        @session-cleared="handleSessionCleared"
      />
    </div>

    <div
      ref="messageContainerRef"
      class="message-container"
    >
      <ChatMessageList />
      <ChatMessageInput />
    </div>
  </div>
</template>

<style lang="less" scoped>
.llm-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
  position: relative;

  // 为 backdrop-filter 提供背景内容
  &::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: linear-gradient(135deg,
      rgba(0, 0, 0, 0.02) 0%,
      rgba(255, 255, 255, 0.01) 50%,
      rgba(0, 0, 0, 0.02) 100%);
    pointer-events: none;
    z-index: 0;
  }
}

.session-header {
  flex-shrink: 0;
  position: relative;
  z-index: 1;
}

.message-container {
  flex: 1;
  margin: 0 auto;
  width: 100%;
  max-width: 800px;
  max-height: calc(100vh - 332px);
  overflow-y: auto;
  overflow-x: hidden;
  position: relative;
  z-index: 1;
  background: linear-gradient(to bottom,
    rgba(0, 0, 0, 0.01) 0%,
    rgba(255, 255, 255, 0.005) 30%,
    rgba(0, 0, 0, 0.01) 60%,
    rgba(255, 255, 255, 0.01) 100%);
}

.dark {
  .llm-container {
    &::before {
      background: linear-gradient(135deg,
        rgba(255, 255, 255, 0.02) 0%,
        rgba(0, 0, 0, 0.01) 50%,
        rgba(255, 255, 255, 0.02) 100%);
    }
  }

  .message-container {
    background: linear-gradient(to bottom,
      rgba(255, 255, 255, 0.01) 0%,
      rgba(0, 0, 0, 0.005) 30%,
      rgba(255, 255, 255, 0.01) 60%,
      rgba(0, 0, 0, 0.01) 100%);
  }
}
</style>
