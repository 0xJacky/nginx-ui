<script setup lang="ts">
import type { ChatComplicationMessage } from '@/api/llm'
import { useAnimationCoordinator } from './animationCoordinator'
import { useLLMStore } from './llm'
import { marked } from './markdown'
import { transformText } from './utils'

interface Props {
  message: ChatComplicationMessage
  index: number
  isEditing: boolean
  loading: boolean
  editValue: string
}

const props = defineProps<Props>()

defineEmits<{
  edit: [index: number]
  save: [index: number]
  cancel: []
  regenerate: [index: number]
}>()

const llmStore = useLLMStore()
const { streamingMessageIndex } = storeToRefs(llmStore)
const { coordinator } = useAnimationCoordinator()

function updateEditValue(value: string) {
  llmStore.editValue = value
}

// Typewriter effect state
const displayText = ref('')
const isTyping = ref(false)
const animationFrame = ref<number | null>(null)

// Cache for transformed content to avoid re-processing
let lastRawContent = ''
let lastTransformedContent = ''

// Get transformed content with caching
function getTransformedContent(content: string): string {
  if (content === lastRawContent) {
    return lastTransformedContent
  }
  lastRawContent = content
  lastTransformedContent = transformText(content)
  return lastTransformedContent
}

// Check if current message should use typewriter effect
const shouldUseTypewriter = computed(() => {
  return props.message.role === 'assistant'
    && !props.isEditing
    && (streamingMessageIndex.value === props.index || isTyping.value)
})

// High-performance typewriter animation using RAF
function startTypewriterAnimation(targetContent: string) {
  const transformedContent = getTransformedContent(targetContent)

  // Skip if content hasn't changed
  if (displayText.value === transformedContent) {
    if (isTyping.value) {
      isTyping.value = false
      coordinator.setMessageTyping(false)
    }
    return
  }

  // For streaming content, just update the target without restarting animation
  if (isTyping.value && animationFrame.value) {
    // Animation is already running, just update the target content
    // The animation will automatically pick up the new content
    return
  }

  // Start from current display text length
  const startLength = displayText.value.length
  const targetLength = transformedContent.length

  // If content is shorter (like editing), immediately set to target
  if (targetLength < startLength) {
    displayText.value = transformedContent
    if (isTyping.value) {
      isTyping.value = false
      coordinator.setMessageTyping(false)
    }
    return
  }

  // Only start new animation if not already typing
  if (!isTyping.value) {
    isTyping.value = true
    coordinator.setMessageTyping(true)

    let currentIndex = startLength
    let lastTime = performance.now()

    // Characters per second (adjustable for speed)
    const charactersPerSecond = 120 // Similar to VScode speed
    const msPerCharacter = 1000 / charactersPerSecond

    function animate(currentTime: number) {
      // Get the latest transformed content (in case it changed during animation)
      const latestContent = getTransformedContent(props.message.content)
      const latestLength = latestContent.length

      const deltaTime = currentTime - lastTime

      // Check if enough time has passed to show next character(s)
      if (deltaTime >= msPerCharacter) {
        // Calculate how many characters to show based on elapsed time
        const charactersToAdd = Math.floor(deltaTime / msPerCharacter)
        currentIndex = Math.min(currentIndex + charactersToAdd, latestLength)

        displayText.value = latestContent.substring(0, currentIndex)
        lastTime = currentTime

        // Check if we've reached the end
        if (currentIndex >= latestLength) {
          isTyping.value = false
          coordinator.setMessageTyping(false)
          coordinator.setMessageStreaming(false) // End streaming when typing completes
          animationFrame.value = null
          return
        }
      }

      // Continue animation
      animationFrame.value = requestAnimationFrame(animate)
    }

    // Start the animation
    animationFrame.value = requestAnimationFrame(animate)
  }
}

// Stop animation when component unmounts
onUnmounted(() => {
  if (animationFrame.value) {
    cancelAnimationFrame(animationFrame.value)
  }
})

// Watch for content changes
watch(
  () => props.message.content,
  newContent => {
    if (shouldUseTypewriter.value) {
      // Only use typewriter effect for streaming messages
      startTypewriterAnimation(newContent)
    }
    else {
      // For user messages, non-streaming messages, or when editing, show immediately
      displayText.value = getTransformedContent(newContent)
      if (isTyping.value) {
        isTyping.value = false
        coordinator.setMessageTyping(false)
      }
    }
  },
  { immediate: true },
)

// Watch for streaming state changes
watch(
  shouldUseTypewriter,
  (newValue, oldValue) => {
    if (!newValue && oldValue) {
      // Don't interrupt if typewriter is still animating
      if (isTyping.value) {
        return
      }

      // If no longer streaming and not typing, immediately show full content
      displayText.value = getTransformedContent(props.message.content)
      if (animationFrame.value) {
        cancelAnimationFrame(animationFrame.value)
        animationFrame.value = null
      }
    }
  },
)

// Reset when switching between messages
watch(
  () => [props.index, props.isEditing],
  () => {
    if (!shouldUseTypewriter.value) {
      displayText.value = getTransformedContent(props.message.content)
      isTyping.value = false
      if (animationFrame.value) {
        cancelAnimationFrame(animationFrame.value)
        animationFrame.value = null
      }
    }
  },
)

// Initialize display text
onMounted(() => {
  if (shouldUseTypewriter.value) {
    displayText.value = ''
    startTypewriterAnimation(props.message.content)
  }
  else {
    displayText.value = getTransformedContent(props.message.content)
  }
})
</script>

<template>
  <AListItem>
    <AComment :author="message.role === 'assistant' ? $gettext('Assistant') : $gettext('User')">
      <template #content>
        <div
          v-if="message.role === 'assistant' || !isEditing"
          class="content"
          :class="{ typing: isTyping }"
        >
          <div
            v-dompurify-html="marked.parse(displayText)"
            class="message-content"
          />
        </div>
        <AInput
          v-else
          :value="editValue"
          class="pa-0"
          :bordered="false"
          @update:value="updateEditValue"
        />
      </template>
      <template #actions>
        <span
          v-if="message.role === 'user' && !isEditing"
          @click="$emit('edit', index)"
        >
          {{ $gettext('Modify') }}
        </span>
        <template v-else-if="isEditing">
          <span @click="$emit('save', index + 1)">{{ $gettext('Save') }}</span>
          <span @click="$emit('cancel')">{{ $gettext('Cancel') }}</span>
        </template>
        <span
          v-else-if="!loading"
          @click="$emit('regenerate', index)"
        >
          {{ $gettext('Reload') }}
        </span>
      </template>
    </AComment>
  </AListItem>
</template>

<style lang="less" scoped>
.content {
  width: 100%;
  position: relative;

  .message-content {
    width: 100%;
  }

  &.typing {
    .message-content {
      // Very subtle glow during typing
      animation: typing-glow 3s ease-in-out infinite;
    }
  }

  .message-content :deep(h1) {
    font-size: 1.5em;
    font-weight: 600;
    margin: 1em 0 0.5em 0;
    line-height: 1.3;
  }

  .message-content :deep(h2) {
    font-size: 1.3em;
    font-weight: 600;
    margin: 0.8em 0 0.4em 0;
    line-height: 1.3;
  }

  .message-content :deep(h3) {
    font-size: 1.15em;
    font-weight: 600;
    margin: 0.7em 0 0.3em 0;
    line-height: 1.3;
  }

  .message-content :deep(h4) {
    font-size: 1.05em;
    font-weight: 600;
    margin: 0.6em 0 0.3em 0;
    line-height: 1.3;
  }

  .message-content :deep(h5), .message-content :deep(h6) {
    font-size: 1em;
    font-weight: 600;
    margin: 0.5em 0 0.2em 0;
    line-height: 1.3;
  }

  .message-content :deep(p) {
    margin: 0.5em 0;
    line-height: 1.6;
  }

  .message-content :deep(ul), .message-content :deep(ol) {
    margin: 0.5em 0 1em 0;
    padding-left: 1.5em;
  }

  .message-content :deep(li) {
    margin: 0.2em 0;
    line-height: 1.5;
  }

  .message-content :deep(ul li) {
    list-style-type: disc;
  }

  .message-content :deep(ol li) {
    list-style-type: decimal;
  }

  .message-content :deep(ul ul), .message-content :deep(ol ol), .message-content :deep(ul ol), .message-content :deep(ol ul) {
    margin: 0.2em 0;
  }

  :deep(code) {
    font-size: 12px;
  }

  :deep(.hljs) {
    border-radius: 5px;
  }

  .message-content :deep(blockquote) {
    display: block;
    opacity: 0.8;
    margin: 1em 0;
    padding: 0.5em 0 0.5em 1em;
    border-left: 4px solid #d0d7de;
    background-color: rgba(208, 215, 222, 0.1);
  }

  .message-content :deep(blockquote p) {
    margin: 0;
  }

  .message-content :deep(table) {
    border-collapse: collapse;
    margin: 1em 0;
    width: 100%;
  }

  .message-content :deep(th), .message-content :deep(td) {
    border: 1px solid #d0d7de;
    padding: 0.5em;
    text-align: left;
  }

  .message-content :deep(th) {
    background-color: #f6f8fa;
    font-weight: 600;
  }

  .message-content :deep(hr) {
    border: none;
    border-top: 1px solid #d0d7de;
    margin: 1.5em 0;
  }
}

@keyframes typing-glow {
  0%, 100% {
    filter: brightness(1) contrast(1);
  }
  50% {
    filter: brightness(1.01) contrast(1.01);
  }
}

// Dark mode adjustments (if applicable)
@media (prefers-color-scheme: dark) {
  .content {
    .typing-indicator {
      background-color: #40a9ff;
    }

    &.typing .message-content {
      animation: typing-glow-dark 3s ease-in-out infinite;
    }
  }
}

@keyframes typing-glow-dark {
  0%, 100% {
    filter: brightness(1) contrast(1);
  }
  50% {
    filter: brightness(1.05) contrast(1.02);
  }
}
</style>
