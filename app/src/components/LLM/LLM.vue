<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useAnimationCoordinator } from './animationCoordinator'
import ChatMessageInput from './ChatMessageInput.vue'
import ChatMessageList from './ChatMessageList.vue'
import { useLLMStore } from './llm'
import LLMSessionTabs from './LLMSessionTabs.vue'
import { useLLMSessionStore } from './sessionStore'

const props = defineProps<{
  path?: string
  nginxConfig?: string
  type?: 'terminal' | 'nginx'
  height?: string
  osInfo?: string
}>()

// Use LLM store and session store
const llmStore = useLLMStore()
const sessionStore = useLLMSessionStore()
const { activeSessionId, sortedSessions } = storeToRefs(sessionStore)

// Animation coordinator
const { state: animationState, isMessageAnimationComplete } = useAnimationCoordinator()

// Message list ref for scrolling
const messageListRef = ref<InstanceType<typeof ChatMessageList>>()

// Get input height for padding calculation
const chatInputRef = ref<InstanceType<typeof ChatMessageInput>>()
const inputHeight = computed(() => chatInputRef.value?.inputHeight || 0)

// Initialize sessions and handle type changes
watch(() => props.type, async () => {
  // Set assistant type
  if (props.type === 'terminal') {
    llmStore.setType('terminal')
  }

  // Load sessions for current type
  if (props.type) {
    await sessionStore.loadSessions(props.type, true) // true indicates it's a type, not a path
  }
  else {
    // Fallback to path-based loading if no type specified
    await sessionStore.loadSessions(props.path)
  }

  // Check if we have sessions available
  if (sortedSessions.value.length > 0 && !activeSessionId.value) {
    // Use the most recent session
    const latestSession = sortedSessions.value[0]
    await llmStore.switchSession(latestSession.session_id)
    sessionStore.setActiveSession(latestSession.session_id)
  }
  else if (sortedSessions.value.length === 0) {
    // No sessions exist for this type/path, create a new one automatically
    let title = $gettext('New Chat')
    if (props.type === 'terminal') {
      title = $gettext('Terminal Assistant')
    }
    else if (props.path) {
      title = $gettext('Chat for %{path}', { path: props.path.split('/').pop() || '' })
    }
    try {
      const session = await sessionStore.createSession(title, props.path, props.type)
      await llmStore.switchSession(session.session_id)
      // Auto-initialization removed - no initial message sent
    }
    catch (error) {
      console.error('Failed to create initial session:', error)
      // Fallback to legacy mode
      await llmStore.initMessages(props.path)
      // Auto-initialization removed - no initial message sent
    }
  }
}, { immediate: true })

// Handle new session creation
async function handleNewSessionCreated() {
  // Reload sessions to update the list
  if (props.type) {
    await sessionStore.loadSessions(props.type, true)
  }
  else {
    await sessionStore.loadSessions(props.path)
  }
  await nextTick()
  // Auto-initialization removed - no initial message sent
}

// Handle when all sessions are cleared
function handleSessionCleared() {
  // Reset to initial state - could create a welcome message or just stay empty
}

// Handle scrolling when messages change (immediate scroll for new messages)
watch(
  () => llmStore.messages.length,
  () => {
    // Reset scroll state for new messages
    if (messageListRef.value) {
      messageListRef.value.resetScrollState()
    }

    nextTick(() => {
      if (messageListRef.value) {
        messageListRef.value.scrollToBottom()
      }
    })
  },
)

// Watch animation state and scroll when all message animations complete
watch(animationState, (_, oldState) => {
  // When message animations complete, scroll to bottom with a final force scroll
  if (oldState && (oldState.messageStreaming || oldState.messageTyping) && isMessageAnimationComplete()) {
    nextTick(() => {
      if (messageListRef.value) {
        messageListRef.value.scrollToBottom()

        // Final force scroll after a small delay to ensure everything is rendered
        setTimeout(() => {
          if (messageListRef.value) {
            messageListRef.value.scrollToBottom()
          }
        }, 200)
      }
    })
  }
}, { deep: true })

// Also scroll during typing animation to keep up with content changes
watch(
  () => animationState.value.messageTyping,
  (isTyping, _wasTyping) => {
    if (isTyping) {
      // During typing, continuously scroll to bottom with a small delay
      const scrollInterval = setInterval(() => {
        if (!animationState.value.messageTyping) {
          clearInterval(scrollInterval)
          // Final scroll when typing stops
          setTimeout(() => {
            if (messageListRef.value) {
              messageListRef.value.scrollToBottom()
            }
          }, 100)
          return
        }
        if (messageListRef.value) {
          messageListRef.value.scrollToBottom()
        }
      }, 100)
    }
  },
)

watch(() => props.nginxConfig, v => {
  llmStore.setNginxConfig(v || '')
}, { immediate: true })
</script>

<template>
  <div
    class="llm-wrapper"
  >
    <div class="llm-container">
      <!-- Session Tabs -->
      <div class="session-header">
        <LLMSessionTabs
          :path="path"
          :type="type"
          @new-session-created="handleNewSessionCreated"
          @session-cleared="handleSessionCleared"
        />
      </div>

      <!-- Message List -->
      <div class="message-container">
        <ChatMessageList
          ref="messageListRef"
          :input-height="inputHeight"
          :os-info="props.osInfo"
        />
      </div>

      <!-- Input Container -->
      <div class="input-container">
        <ChatMessageInput
          ref="chatInputRef"
          :os-info="props.osInfo"
        />
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.llm-wrapper {
  width: 100%;
  height: max(600px, calc(100vh - 200px));
}

.llm-container {
  width: 100%;
  height: 100%;
  position: relative;
}

.session-header {
  border-bottom: 1px solid var(--ant-color-border);
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 48px;
}

.message-container {
  height: calc(100% - 48px - 56px); // Total height - header height - default input height
  overflow: hidden;
  position: relative;

  :deep(.message-list-container) {
    height: 100%;
  }
}

.input-container {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  min-height: 56px;
  background: var(--ant-color-bg-container);
  border-top: 1px solid var(--ant-color-border);
}

.dark {
  .session-header {
    border-bottom: 1px solid #333;
  }

  .input-container {
    border-top: 1px solid #333;
  }
}
</style>
