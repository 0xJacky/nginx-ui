import type { Ref } from 'vue'

interface TypewriterOptions {
  baseSpeed?: number
  fastSpeed?: number
  scrollInterval?: number
}

export function useTypewriter(options: TypewriterOptions = {}) {
  const {
    baseSpeed = 35,
    fastSpeed = 25,
    scrollInterval = 150,
  } = options

  let isTyping = false
  let typeQueue: string[] = []
  let scrollTimer: number | null = null
  let rafId: number | null = null

  const typeText = async (
    content: string,
    targetRef: Ref<string>,
    onScroll?: () => void,
    isFastMode = false,
  ): Promise<void> => {
    return new Promise(resolve => {
      if (isTyping) {
        typeQueue.push(content)
        resolve()
        return
      }

      isTyping = true
      const chars = content.split('')
      let charIndex = 0

      const typeNextChars = () => {
        if (charIndex >= chars.length) {
          isTyping = false

          // Process queued content
          if (typeQueue.length > 0) {
            const nextContent = typeQueue.shift()!
            typeText(nextContent, targetRef, onScroll, isFastMode).then(resolve)
          }
          else {
            resolve()
          }
          return
        }

        // Add character by character
        targetRef.value += chars[charIndex]
        charIndex++

        // Throttled scrolling - reduce scroll frequency
        if (onScroll && !scrollTimer) {
          scrollTimer = window.setTimeout(() => {
            onScroll()
            scrollTimer = null
          }, scrollInterval)
        }

        // Dynamic speed based on mode
        const delay = isFastMode ? fastSpeed : baseSpeed

        rafId = requestAnimationFrame(() => {
          setTimeout(typeNextChars, delay)
        })
      }

      typeNextChars()
    })
  }

  const resetTypewriter = () => {
    isTyping = false
    typeQueue = []

    if (scrollTimer) {
      clearTimeout(scrollTimer)
      scrollTimer = null
    }

    if (rafId) {
      cancelAnimationFrame(rafId)
      rafId = null
    }
  }

  const pauseTypewriter = () => {
    if (rafId) {
      cancelAnimationFrame(rafId)
      rafId = null
    }
  }

  const resumeTypewriter = () => {
    // Typewriter will resume automatically when next content is queued
  }

  return {
    typeText,
    resetTypewriter,
    pauseTypewriter,
    resumeTypewriter,
    isTyping: readonly(ref(isTyping)),
  }
}
