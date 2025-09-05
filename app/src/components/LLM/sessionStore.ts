import type { ChatComplicationMessage, LLMSessionResponse } from '@/api/llm'
import llm from '@/api/llm'
import { animationCoordinator } from './animationCoordinator'

export const useLLMSessionStore = defineStore('llm-session', () => {
  // State
  const sessions = ref<LLMSessionResponse[]>([])
  const activeSessionId = ref<string | null>(null)
  const loading = ref(false)
  const sessionDrawerVisible = ref(false)
  const typewriterInProgress = ref(new Set<string>())

  // Getters
  const activeSession = computed(() => {
    if (!activeSessionId.value)
      return null
    return sessions.value.find(s => s.session_id === activeSessionId.value) || null
  })

  const sortedSessions = computed(() => {
    return [...sessions.value].sort((a, b) => {
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
    })
  })

  const hasActiveSession = computed(() => activeSessionId.value !== null)

  // Actions
  async function loadSessions(pathOrType?: string, isType?: boolean) {
    loading.value = true
    try {
      const response = await llm.get_sessions(pathOrType, isType)
      sessions.value = response
    }
    catch (error) {
      console.error('Failed to load sessions:', error)
    }
    finally {
      loading.value = false
    }
  }

  async function createSession(title: string, path?: string, type?: string) {
    try {
      const sessionData: { title: string, path?: string, type?: string } = { title }

      // For terminal type, don't pass path
      if (type === 'terminal') {
        sessionData.type = type
      }
      else if (path) {
        sessionData.path = path
      }

      const response = await llm.create_session(sessionData)
      sessions.value.unshift(response)
      activeSessionId.value = response.session_id
      return response
    }
    catch (error) {
      console.error('Failed to create session:', error)
      throw error
    }
  }

  async function updateSession(sessionId: string, data: { title?: string, messages?: ChatComplicationMessage[], is_active?: boolean }) {
    try {
      const response = await llm.update_session(sessionId, data)
      const index = sessions.value.findIndex(s => s.session_id === sessionId)
      if (index !== -1) {
        // If typewriter is in progress for this session, preserve the current title
        if (typewriterInProgress.value.has(sessionId) && data.title) {
          const currentTitle = sessions.value[index].title
          sessions.value[index] = { ...response, title: currentTitle }
        }
        else {
          sessions.value[index] = response
        }
      }
      return response
    }
    catch (error) {
      console.error('Failed to update session:', error)
      throw error
    }
  }

  async function deleteSession(sessionId: string) {
    try {
      await llm.delete_session(sessionId)
      sessions.value = sessions.value.filter(s => s.session_id !== sessionId)

      // If deleting active session, clear it
      if (activeSessionId.value === sessionId) {
        activeSessionId.value = null
      }
    }
    catch (error) {
      console.error('Failed to delete session:', error)
      throw error
    }
  }

  async function duplicateSession(sessionId: string) {
    try {
      const response = await llm.duplicate_session(sessionId)
      sessions.value.unshift(response)
      activeSessionId.value = response.session_id
      return response
    }
    catch (error) {
      console.error('Failed to duplicate session:', error)
      throw error
    }
  }

  async function generateSessionTitle(sessionId: string) {
    try {
      // Skip if typewriter is already in progress for this session
      if (typewriterInProgress.value.has(sessionId)) {
        return
      }

      const response = await llm.generate_session_title(sessionId)

      // Update the session in the local store with typewriter effect
      const index = sessions.value.findIndex(s => s.session_id === sessionId)
      if (index !== -1) {
        await typewriterEffect(sessionId, response.title)
      }

      return response
    }
    catch (error) {
      console.error('Failed to generate session title:', error)
      throw error
    }
  }

  // Typewriter effect for session title
  async function typewriterEffect(sessionId: string, newTitle: string) {
    const index = sessions.value.findIndex(s => s.session_id === sessionId)
    if (index === -1) {
      return
    }

    const session = sessions.value[index]

    // Mark typewriter as in progress
    typewriterInProgress.value.add(sessionId)
    animationCoordinator.setTitleAnimating(true)

    try {
      let currentText = ''

      // Clear the current title first
      session.title = ''

      // Type out the new title character by character
      for (let i = 0; i <= newTitle.length; i++) {
        // Double-check session still exists (in case of concurrent operations)
        const currentIndex = sessions.value.findIndex(s => s.session_id === sessionId)
        if (currentIndex === -1) {
          break
        }

        currentText = newTitle.substring(0, i)
        sessions.value[currentIndex].title = currentText

        // Ensure Vue updates the DOM
        await nextTick()

        // Wait between each character (adjust speed as needed)
        await new Promise(resolve => setTimeout(resolve, 20))
      }

      // Ensure final title is set correctly
      const finalIndex = sessions.value.findIndex(s => s.session_id === sessionId)
      if (finalIndex !== -1) {
        sessions.value[finalIndex].title = newTitle
      }
    }
    finally {
      // Always remove from progress tracking when done
      typewriterInProgress.value.delete(sessionId)
      animationCoordinator.setTitleAnimating(false)
    }
  }

  function setActiveSession(sessionId: string | null) {
    activeSessionId.value = sessionId
  }

  async function updateSessionActiveStatus(sessionId: string, isActive: boolean) {
    try {
      const response = await llm.update_session(sessionId, { is_active: isActive })
      const index = sessions.value.findIndex(s => s.session_id === sessionId)
      if (index !== -1) {
        sessions.value[index].is_active = isActive
      }
      return response
    }
    catch (error) {
      console.error('Failed to update session active status:', error)
      throw error
    }
  }

  function toggleSessionDrawer() {
    sessionDrawerVisible.value = !sessionDrawerVisible.value
  }

  function showSessionDrawer() {
    sessionDrawerVisible.value = true
  }

  function hideSessionDrawer() {
    sessionDrawerVisible.value = false
  }

  // Initialize (will be called with path from parent component)
  // onMounted(() => {
  //   loadSessions()
  // })

  return {
    // State
    sessions,
    activeSessionId,
    loading,
    sessionDrawerVisible,
    typewriterInProgress,

    // Getters
    activeSession,
    sortedSessions,
    hasActiveSession,

    // Actions
    loadSessions,
    createSession,
    updateSession,
    deleteSession,
    duplicateSession,
    generateSessionTitle,
    setActiveSession,
    updateSessionActiveStatus,
    toggleSessionDrawer,
    showSessionDrawer,
    hideSessionDrawer,
  }
})
