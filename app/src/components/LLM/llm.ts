import type { ChatComplicationMessage } from '@/api/llm'
import llm from '@/api/llm'
import { animationCoordinator } from './animationCoordinator'
import { ChatService } from './chatService'
import { useLLMSessionStore } from './sessionStore'

export const useLLMStore = defineStore('llm', () => {
  // State
  const path = ref('')
  const nginxConfig = ref('')
  const currentSessionId = ref<string | null>(null)
  const messages = ref<ChatComplicationMessage[]>([])
  const messageContainerRef = ref<HTMLDivElement>()
  const loading = ref(false)
  const editingIdx = ref(-1)
  const editValue = ref('')
  const askBuffer = ref('')
  const streamingMessageIndex = ref(-1)
  const userScrolledUp = ref(false)
  const messageTypingCompleted = ref(false)

  // Getters
  const isEditing = computed(() => editingIdx.value !== -1)
  const currentEditingMessage = computed(() => {
    if (editingIdx.value !== -1 && messages.value[editingIdx.value]) {
      return messages.value[editingIdx.value]
    }
    return null
  })
  const hasMessages = computed(() => messages.value.length > 0)

  // Actions
  // Initialize messages for a specific file path
  async function initMessages(filePath?: string) {
    messages.value = []
    if (filePath) {
      try {
        const record = await llm.get_messages(filePath)
        messages.value = record.content || []
      }
      catch (error) {
        console.error('Failed to load chat record:', error)
      }
      path.value = filePath
    }
  }

  // Switch to a specific session
  async function switchSession(sessionId: string) {
    try {
      currentSessionId.value = sessionId
      const session = await llm.get_session(sessionId)
      messages.value = session.messages || []
      // Only update path if it's not already set to terminal assistant
      if (path.value !== '__terminal_assistant__') {
        path.value = session.path || ''
      }
      cancelEdit()
    }
    catch (error) {
      console.error('Failed to switch session:', error)
    }
  }

  // Save current messages to session
  async function saveSession() {
    if (!currentSessionId.value)
      return

    try {
      const validMessages = messages.value.filter(msg => msg.content.trim() !== '')
      await llm.update_session(currentSessionId.value, { messages: validMessages })
    }
    catch (error) {
      console.error('Failed to save session:', error)
    }
  }

  // Start editing a message at the specified index
  function startEdit(index: number) {
    if (index >= 0 && index < messages.value.length) {
      editingIdx.value = index
      editValue.value = messages.value[index].content
    }
  }

  // Save the edited message
  function saveEdit() {
    if (editingIdx.value !== -1 && messages.value[editingIdx.value]) {
      messages.value[editingIdx.value].content = editValue.value
      editingIdx.value = -1
      editValue.value = ''
    }
  }

  // Cancel editing and reset state
  function cancelEdit() {
    editingIdx.value = -1
    editValue.value = ''
  }

  // Add a new user message
  function addUserMessage(content: string) {
    messages.value.push({
      role: 'user',
      content,
    })
  }

  // Add a new assistant message
  function addAssistantMessage(content: string = '') {
    messages.value.push({
      role: 'assistant',
      content,
    })
  }

  // Update the last assistant message content (for streaming)
  function updateLastAssistantMessage(content: string) {
    const lastMessage = messages.value[messages.value.length - 1]
    if (lastMessage && lastMessage.role === 'assistant') {
      lastMessage.content = content
    }
  }

  // Remove messages after the specified index for regeneration
  function prepareRegenerate(index: number) {
    messages.value = messages.value.slice(0, index)
    cancelEdit()
  }

  // Clear all messages
  function clearMessages() {
    messages.value = []
    cancelEdit()
  }

  // Store chat record to server
  async function storeRecord() {
    // Prefer session storage over legacy file storage
    if (currentSessionId.value) {
      await saveSession()
    }
    else if (path.value) {
      try {
        const validMessages = messages.value.filter(msg => msg.content.trim() !== '')
        await llm.store_messages({
          file_name: path.value,
          messages: validMessages,
        })
      }
      catch (error) {
        console.error('Failed to store chat record:', error)
      }
    }
  }

  // Current assistant type
  const assistantType = ref<string>('nginx')

  // Set assistant type for the current session
  function setType(type: 'terminal') {
    if (type === 'terminal') {
      assistantType.value = 'terminal'
    }
  }

  // Clear chat record on server
  async function clearRecord() {
    if (!path.value)
      return

    try {
      await llm.store_messages({
        file_name: path.value,
        messages: [],
      })
      clearMessages()
    }
    catch (error) {
      console.error('Failed to clear chat record:', error)
    }
  }

  // Set loading state
  function setLoading(loadingState: boolean) {
    loading.value = loadingState
  }

  // Set ask buffer
  function setAskBuffer(buffer: string) {
    askBuffer.value = buffer
  }

  // Clear ask buffer
  function clearAskBuffer() {
    askBuffer.value = ''
  }

  // Auto-scroll state management
  const isAutoScrolling = ref(false)
  const scrollObserver = ref<ResizeObserver | null>(null)
  const mutationObserver = ref<MutationObserver | null>(null)
  const lastScrollHeight = ref(0)

  // Check if container is at bottom
  function isAtBottom() {
    if (!messageContainerRef.value)
      return true

    const container = messageContainerRef.value
    const threshold = 10 // Very strict threshold for precision
    const scrollBottom = container.scrollHeight - container.scrollTop - container.clientHeight
    return scrollBottom <= threshold
  }

  // Smooth scroll to bottom with high precision
  function scrollToBottom(force = false) {
    if (!messageContainerRef.value)
      return

    const container = messageContainerRef.value

    // Always scroll if forced, or if user hasn't scrolled up
    if (!force && userScrolledUp.value) {
      return
    }

    // Mark as auto-scrolling to prevent user scroll detection
    isAutoScrolling.value = true

    // Immediate scroll
    container.scrollTop = container.scrollHeight

    // Reset auto-scroll flag after a short delay
    setTimeout(() => {
      isAutoScrolling.value = false
      userScrolledUp.value = false // Reset user scroll state
    }, 50)
  }

  // Enhanced scroll position detection
  function checkScrollPosition() {
    if (!messageContainerRef.value || isAutoScrolling.value)
      return

    const wasAtBottom = isAtBottom()
    userScrolledUp.value = !wasAtBottom
  }

  // Start real-time scroll tracking for typewriter animations
  function startScrollTracking() {
    if (!messageContainerRef.value)
      return

    const container = messageContainerRef.value

    // Stop any existing observers
    stopScrollTracking()

    // Track size changes using ResizeObserver for real-time response
    scrollObserver.value = new ResizeObserver(entries => {
      for (const entry of entries) {
        const newHeight = entry.target.scrollHeight
        if (newHeight !== lastScrollHeight.value) {
          lastScrollHeight.value = newHeight

          // Only auto-scroll if user hasn't scrolled up
          if (!userScrolledUp.value) {
            scrollToBottom()
          }
        }
      }
    })

    // Start observing the container
    scrollObserver.value.observe(container)

    // Also observe content changes using MutationObserver
    mutationObserver.value = new MutationObserver(mutations => {
      let shouldScroll = false

      for (const mutation of mutations) {
        if (mutation.type === 'childList' || mutation.type === 'characterData') {
          shouldScroll = true
          break
        }
      }

      if (shouldScroll && !userScrolledUp.value) {
        // Use RAF for smooth scrolling
        requestAnimationFrame(() => {
          scrollToBottom()
        })
      }
    })

    // Observe all content changes
    mutationObserver.value.observe(container, {
      childList: true,
      subtree: true,
      characterData: true,
    })

    // Initial scroll to bottom
    scrollToBottom(true)
  }

  // Stop scroll tracking
  function stopScrollTracking() {
    if (scrollObserver.value) {
      scrollObserver.value.disconnect()
      scrollObserver.value = null
    }

    if (mutationObserver.value) {
      mutationObserver.value.disconnect()
      mutationObserver.value = null
    }
  }

  // Set streaming message index
  function setStreamingMessageIndex(index: number) {
    streamingMessageIndex.value = index
  }

  // Clear streaming message index
  function clearStreamingMessageIndex() {
    streamingMessageIndex.value = -1
  }

  // Request: Send messages to server using chat service
  async function request(language?: string) {
    setLoading(true)
    animationCoordinator.reset() // Reset all animation states
    animationCoordinator.setMessageStreaming(true)

    // Set the streaming message index to the last message (assistant message)
    setStreamingMessageIndex(messages.value.length - 1)

    // Start real-time scroll tracking for typewriter animation
    startScrollTracking()

    try {
      const chatService = new ChatService()
      const assistantMessage = await chatService.request(
        assistantType.value,
        messages.value.slice(0, -1), // Exclude the empty assistant message
        message => {
          // Update the current assistant message in real-time
          updateLastAssistantMessage(message.content)
        },
        language,
        nginxConfig.value,
      )

      // Update the final content
      updateLastAssistantMessage(assistantMessage.content)

      // If no typing animation starts within a reasonable time, end streaming
      // This handles cases where content is too short for typewriter effect
      setTimeout(() => {
        if (animationCoordinator.getState().value.messageStreaming) {
          animationCoordinator.setMessageStreaming(false)
        }
      }, 200)

      // Ensure content is rendered before scrolling
      await nextTick()
      await nextTick() // Double nextTick for complex content
      scrollToBottom(true) // Force scroll when message is complete
    }
    catch (error) {
      console.error('Chat request failed:', error)
      // Remove the empty assistant message on error
      if (messages.value.length > 0 && messages.value[messages.value.length - 1].content === '') {
        messages.value.pop()
      }
    }
    finally {
      // Don't clear streaming index immediately - let typewriter animation complete first

      // Ensure all DOM updates are complete before final scroll
      await nextTick()
      await nextTick()
      scrollToBottom(true) // Force scroll after loading completes

      await storeRecord()

      // Title animation will be triggered by coordinator when message animation completes

      // Wait for all animations to complete before ending loading state
      setTimeout(async () => {
        await animationCoordinator.waitForAllAnimationsComplete()

        // Now clear streaming state after all animations are done
        clearStreamingMessageIndex()

        // Stop scroll tracking when all animations are done
        stopScrollTracking()

        setLoading(false)

        // Final scroll after everything is truly complete
        scrollToBottom(true)

        // Generate session title after everything is complete
        await tryGenerateSessionTitle()
      }, 100)
    }
  }

  // Send: Add user message into messages then call request
  async function send(content: string, currentLanguage?: string) {
    // Add user message directly without embedding file content
    addUserMessage(content)

    // Clear ask buffer
    clearAskBuffer()

    // Add empty assistant message for real-time updates
    addAssistantMessage('')

    await request(currentLanguage)
  }

  // Regenerate: Removes messages after index and re-request the answer
  async function regenerate(index: number, currentLanguage?: string) {
    prepareRegenerate(index)

    // Add empty assistant message for real-time updates
    addAssistantMessage('')

    await request(currentLanguage)
  }

  // Auto-generate title for sessions with user messages
  async function tryGenerateSessionTitle() {
    if (!currentSessionId.value) {
      return
    }

    // Check if there are user messages in the conversation
    const hasUserMessages = messages.value.some(msg => msg.role === 'user')
    if (!hasUserMessages) {
      return
    }
    // Wait for message animation to complete before starting title animation
    await animationCoordinator.waitForMessageAnimationComplete()

    try {
      const sessionStore = useLLMSessionStore()
      await sessionStore.generateSessionTitle(currentSessionId.value)
    }
    catch (error) {
      console.error('Failed to auto-generate session title:', error)
    }
  }

  function setNginxConfig(config: string) {
    nginxConfig.value = config
  }

  // Listen for title animation trigger from coordinator
  onMounted(() => {
    window.addEventListener('startTitleAnimation', () => {
      tryGenerateSessionTitle()
    })
  })

  onUnmounted(() => {
    window.removeEventListener('startTitleAnimation', () => {
      tryGenerateSessionTitle()
    })

    // Clean up observers when component unmounts
    stopScrollTracking()
  })

  // Set up manual scroll detection when container ref changes
  watch(messageContainerRef, newContainer => {
    if (newContainer) {
      // Simple scroll listener for detecting user manual scrolls
      const handleScroll = () => {
        // Only detect scroll if not auto-scrolling
        if (!isAutoScrolling.value) {
          checkScrollPosition()
        }
      }

      newContainer.addEventListener('scroll', handleScroll, { passive: true })

      // Initial check
      checkScrollPosition()
    }
  })
  // Return all state, getters, and actions
  return {
    // State
    path,
    nginxConfig,
    currentSessionId,
    messages,
    loading,
    editingIdx,
    editValue,
    askBuffer,
    messageContainerRef,
    streamingMessageIndex,
    userScrolledUp,
    messageTypingCompleted,
    assistantType,

    // Getters
    isEditing,
    currentEditingMessage,
    hasMessages,

    // Actions
    initMessages,
    setNginxConfig,
    switchSession,
    saveSession,
    startEdit,
    saveEdit,
    cancelEdit,
    addUserMessage,
    addAssistantMessage,
    updateLastAssistantMessage,
    prepareRegenerate,
    clearMessages,
    storeRecord,
    setType,
    clearRecord,
    setLoading,
    setAskBuffer,
    clearAskBuffer,
    setStreamingMessageIndex,
    clearStreamingMessageIndex,
    request,
    send,
    regenerate,
    scrollToBottom,
    tryGenerateSessionTitle,
    checkScrollPosition,
    startScrollTracking,
    stopScrollTracking,
  }
})
