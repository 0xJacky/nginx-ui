// Animation Coordinator - Centralized state management for all animations and scrolling
import { readonly, ref, watch } from 'vue'

export interface AnimationState {
  messageStreaming: boolean
  messageTyping: boolean
  titleAnimating: boolean
  scrolling: boolean
}

class AnimationCoordinator {
  private state = ref<AnimationState>({
    messageStreaming: false,
    messageTyping: false,
    titleAnimating: false,
    scrolling: false,
  })

  private callbacks: {
    onMessageTypingComplete?: () => void
    onTitleAnimationComplete?: () => void
    onAllAnimationsComplete?: () => void
  } = {}

  // Get current state (readonly)
  getState() {
    return readonly(this.state)
  }

  // Check if any animation is in progress
  isAnyAnimationActive() {
    const s = this.state.value
    return s.messageStreaming || s.messageTyping || s.titleAnimating || s.scrolling
  }

  // Check if message-related animations are complete
  isMessageAnimationComplete() {
    const s = this.state.value
    return !s.messageStreaming && !s.messageTyping
  }

  // Set message streaming state
  setMessageStreaming(streaming: boolean) {
    if (this.state.value.messageStreaming === streaming)
      return

    this.state.value.messageStreaming = streaming

    if (!streaming) {
      // When streaming stops, message typing might still be active
      this.checkTransitions()
    }
  }

  // Set message typing state
  setMessageTyping(typing: boolean) {
    // Prevent redundant state changes
    if (this.state.value.messageTyping === typing)
      return

    this.state.value.messageTyping = typing

    if (!typing) {
      this.callbacks.onMessageTypingComplete?.()
      this.checkTransitions()
    }
  }

  // Set title animation state
  setTitleAnimating(animating: boolean) {
    if (this.state.value.titleAnimating === animating)
      return

    this.state.value.titleAnimating = animating

    if (!animating) {
      this.callbacks.onTitleAnimationComplete?.()
      this.checkTransitions()
    }
  }

  // Set scrolling state
  setScrolling(scrolling: boolean) {
    this.state.value.scrolling = scrolling

    if (!scrolling) {
      this.checkTransitions()
    }
  }

  // Set callbacks
  setCallbacks(callbacks: Partial<typeof this.callbacks>) {
    Object.assign(this.callbacks, callbacks)
  }

  private titleAnimationTriggered = false

  // Check for state transitions and trigger appropriate actions
  private checkTransitions() {
    const s = this.state.value

    // If message animation is complete and title is not animating, we can start title animation
    if (this.isMessageAnimationComplete() && !s.titleAnimating && !this.titleAnimationTriggered) {
      this.titleAnimationTriggered = true

      // Small delay before starting title animation
      setTimeout(() => {
        if (this.isMessageAnimationComplete() && !this.state.value.titleAnimating) {
          this.triggerTitleAnimation()
        }
      }, 200)
    }

    // If all animations are complete
    if (!this.isAnyAnimationActive()) {
      this.callbacks.onAllAnimationsComplete?.()
    }
  }

  // Trigger title animation (to be called by external code)
  private triggerTitleAnimation() {
    // This will be handled by the LLM store
    window.dispatchEvent(new CustomEvent('startTitleAnimation'))
  }

  // Reset all states (useful when starting a new conversation)
  reset() {
    this.state.value = {
      messageStreaming: false,
      messageTyping: false,
      titleAnimating: false,
      scrolling: false,
    }
    this.titleAnimationTriggered = false
  }

  // Wait for message animation to complete
  async waitForMessageAnimationComplete(): Promise<void> {
    return new Promise(resolve => {
      if (this.isMessageAnimationComplete()) {
        resolve()
        return
      }

      const unwatch = watch(
        () => this.isMessageAnimationComplete(),
        complete => {
          if (complete) {
            unwatch()
            resolve()
          }
        },
      )
    })
  }

  // Wait for all animations to complete
  async waitForAllAnimationsComplete(): Promise<void> {
    return new Promise(resolve => {
      if (!this.isAnyAnimationActive()) {
        resolve()
        return
      }

      const unwatch = watch(
        () => this.isAnyAnimationActive(),
        active => {
          if (!active) {
            unwatch()
            resolve()
          }
        },
      )
    })
  }
}

// Global singleton instance
export const animationCoordinator = new AnimationCoordinator()

// Composable for using in components
export function useAnimationCoordinator() {
  return {
    coordinator: animationCoordinator,
    state: animationCoordinator.getState(),
    isAnyAnimationActive: () => animationCoordinator.isAnyAnimationActive(),
    isMessageAnimationComplete: () => animationCoordinator.isMessageAnimationComplete(),
  }
}
