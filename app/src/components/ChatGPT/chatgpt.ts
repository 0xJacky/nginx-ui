import type { ChatComplicationMessage } from '@/api/openai'
import openai from '@/api/openai'
import { ChatService } from './chatService'

export const useChatGPTStore = defineStore('chatgpt', () => {
  // State
  const path = ref<string>('') // Path to the chat record file
  const messages = ref<ChatComplicationMessage[]>([])
  const messageContainerRef = ref<HTMLDivElement>()
  const loading = ref(false)
  const editingIdx = ref(-1)
  const editValue = ref('')
  const askBuffer = ref('')
  const streamingMessageIndex = ref(-1) // Track which message is currently streaming

  // Getters
  const isEditing = computed(() => editingIdx.value !== -1)
  const currentEditingMessage = computed(() => {
    if (editingIdx.value !== -1 && messages.value[editingIdx.value]) {
      return messages.value[editingIdx.value]
    }
    return null
  })
  const hasMessages = computed(() => messages.value.length > 0)
  const shouldShowStartButton = computed(() => messages.value.length === 0)

  // Actions
  // Initialize messages for a specific file path
  async function initMessages(filePath?: string) {
    messages.value = []
    if (filePath) {
      try {
        const record = await openai.get_record(filePath)
        messages.value = record.content || []
      }
      catch (error) {
        console.error('Failed to load chat record:', error)
      }
      path.value = filePath
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
    if (!path.value)
      return

    try {
      // Filter out empty messages before storing
      const validMessages = messages.value.filter(msg => msg.content.trim() !== '')
      await openai.store_record({
        file_name: path.value,
        messages: validMessages,
      })
    }
    catch (error) {
      console.error('Failed to store chat record:', error)
    }
  }

  // Clear chat record on server
  async function clearRecord() {
    if (!path.value)
      return

    try {
      await openai.store_record({
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

  // scroll to bottom
  function scrollToBottom() {
    messageContainerRef.value?.scrollTo({
      top: messageContainerRef.value.scrollHeight,
      behavior: 'smooth',
    })
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
  async function request() {
    setLoading(true)

    // Set the streaming message index to the last message (assistant message)
    setStreamingMessageIndex(messages.value.length - 1)

    try {
      const chatService = new ChatService()
      const assistantMessage = await chatService.request(
        path.value,
        messages.value.slice(0, -1), // Exclude the empty assistant message
        message => {
          // Update the current assistant message in real-time
          updateLastAssistantMessage(message.content)
        },
      )

      // Update the final content
      updateLastAssistantMessage(assistantMessage.content)

      // Auto scroll to bottom after response
      await nextTick()
      scrollToBottom()
    }
    catch (error) {
      console.error('Chat request failed:', error)
      // Remove the empty assistant message on error
      if (messages.value.length > 0 && messages.value[messages.value.length - 1].content === '') {
        messages.value.pop()
      }
    }
    finally {
      setLoading(false)
      clearStreamingMessageIndex() // Clear streaming state
      await storeRecord()
    }
  }

  // Send: Add user message into messages then call request
  async function send(content: string, currentLanguage?: string) {
    if (messages.value.length === 0) {
      // The first message
      addUserMessage(`${content}\n\nCurrent Language Code: ${currentLanguage}`)
    }
    else {
      // Append user's new message
      addUserMessage(askBuffer.value)
      clearAskBuffer()
    }

    // Add empty assistant message for real-time updates
    addAssistantMessage('')

    await request()
  }

  // Regenerate: Removes messages after index and re-request the answer
  async function regenerate(index: number) {
    prepareRegenerate(index)

    // Add empty assistant message for real-time updates
    addAssistantMessage('')

    await request()
  }

  watch(messages, () => {
    scrollToBottom()
  }, { immediate: true })
  // Return all state, getters, and actions
  return {
    // State
    messages,
    loading,
    editingIdx,
    editValue,
    askBuffer,
    messageContainerRef,
    streamingMessageIndex,

    // Getters
    isEditing,
    currentEditingMessage,
    hasMessages,
    shouldShowStartButton,

    // Actions
    initMessages,
    startEdit,
    saveEdit,
    cancelEdit,
    addUserMessage,
    addAssistantMessage,
    updateLastAssistantMessage,
    prepareRegenerate,
    clearMessages,
    storeRecord,
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
  }
})
