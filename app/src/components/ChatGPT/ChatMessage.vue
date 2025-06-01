<script setup lang="ts">
import type { ChatComplicationMessage } from '@/api/openai'
import { useChatGPTStore } from './chatgpt'
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

const chatGPTStore = useChatGPTStore()
const { streamingMessageIndex } = storeToRefs(chatGPTStore)

function updateEditValue(value: string) {
  chatGPTStore.editValue = value
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
    && streamingMessageIndex.value === props.index
})

// High-performance typewriter animation using RAF
function startTypewriterAnimation(targetContent: string) {
  if (animationFrame.value) {
    cancelAnimationFrame(animationFrame.value)
  }

  const transformedContent = getTransformedContent(targetContent)

  // Skip if content hasn't changed
  if (displayText.value === transformedContent) {
    isTyping.value = false
    return
  }

  // Start from current display text length
  const startLength = displayText.value.length
  const targetLength = transformedContent.length

  // If content is shorter (like editing), immediately set to target
  if (targetLength < startLength) {
    displayText.value = transformedContent
    isTyping.value = false
    return
  }

  isTyping.value = true
  let currentIndex = startLength
  let lastTime = performance.now()

  // Characters per second (adjustable for speed)
  const charactersPerSecond = 120 // Similar to VScode speed
  const msPerCharacter = 1000 / charactersPerSecond

  function animate(currentTime: number) {
    const deltaTime = currentTime - lastTime

    // Check if enough time has passed to show next character(s)
    if (deltaTime >= msPerCharacter) {
      // Calculate how many characters to show based on elapsed time
      const charactersToAdd = Math.floor(deltaTime / msPerCharacter)
      currentIndex = Math.min(currentIndex + charactersToAdd, targetLength)

      displayText.value = transformedContent.substring(0, currentIndex)
      lastTime = currentTime

      // Check if we've reached the end
      if (currentIndex >= targetLength) {
        isTyping.value = false
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
      isTyping.value = false
    }
  },
  { immediate: true },
)

// Watch for streaming state changes
watch(
  shouldUseTypewriter,
  newValue => {
    if (!newValue) {
      // If no longer streaming, immediately show full content
      displayText.value = getTransformedContent(props.message.content)
      isTyping.value = false
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

  :deep(code) {
    font-size: 12px;
  }

  :deep(.hljs) {
    border-radius: 5px;
  }

  :deep(blockquote) {
    display: block;
    opacity: 0.6;
    margin: 0.5em 0;
    padding-left: 1em;
    border-left: 3px solid #ccc;
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
