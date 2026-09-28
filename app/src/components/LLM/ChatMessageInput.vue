<script setup lang="ts">
import { ArrowUpOutlined, BulbOutlined, ClearOutlined, RedoOutlined } from '@antdv-next/icons'
import { useElementSize } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { thinkingLevelLabel } from '@/constants/llm'
import { useSettingsStore } from '@/pinia'
import { useLLMStore } from './llm'

const props = defineProps<{
  nginxConfig?: string
  osInfo?: string
}>()
const llmStore = useLLMStore()
const {
  loading,
  askBuffer,
  messages,
  chatModels,
  selectedModel,
  thinkingLevel,
  activeModel,
  availableThinkingLevels,
} = storeToRefs(llmStore)

onMounted(llmStore.loadChatModels)

const modelOptions = computed(() => chatModels.value.map(model => ({
  label: model.name,
  value: model.name,
})))

const thinkingOptions = computed(() => [
  { label: thinkingLevelLabel(''), value: '' },
  ...availableThinkingLevels.value.map(level => ({ label: thinkingLevelLabel(level), value: level })),
])

// The composer draws the border and the focus ring (on :focus-within), so
// the textarea drops its own, including antd's keyboard focus outline
const bareInput = { border: 'none', boxShadow: 'none', outline: 'none', background: 'transparent' }
const composerInputStyles = {
  root: bareInput,
  textarea: { ...bareInput, padding: '10px 12px 4px', lineHeight: 1.5, resize: 'none' as const },
}

// A level the model does not offer falls back to the provider default
const currentThinkingLevel = computed(() => availableThinkingLevels.value.includes(thinkingLevel.value as never)
  ? thinkingLevel.value
  : '')
const { language: currentLanguage } = storeToRefs(useSettingsStore())

// Get input container height for spacer
const inputContainerRef = ref<HTMLElement>()
const { height: inputHeight } = useElementSize(inputContainerRef)

// Expose the height so parent can use it
defineExpose({
  inputHeight,
})

// Watch height changes to force parent updates
watch(inputHeight, () => {
  // Force reactivity by triggering a re-render
  nextTick()
})

const messagesLength = computed(() => messages.value?.length ?? 0)

function handleSend(event?: KeyboardEvent) {
  // If it's a keyboard event and shift is pressed, allow default (new line)
  if (event && event.shiftKey) {
    return
  }

  // Prevent default Enter behavior when not shift+enter
  if (event) {
    event.preventDefault()
  }

  if (loading.value || !askBuffer.value.trim())
    return
  llmStore.send(askBuffer.value, currentLanguage.value, props.osInfo)
}

function handleButtonClick() {
  if (loading.value || !askBuffer.value.trim())
    return
  llmStore.send(askBuffer.value, currentLanguage.value, props.osInfo)
}
</script>

<template>
  <div ref="inputContainerRef" class="input-msg">
    <div class="composer">
      <ATextarea
        v-model:value="askBuffer"
        :auto-size="{ minRows: 1, maxRows: 8 }"
        :placeholder="$gettext('Type your message here...')"
        :aria-label="$gettext('Message')"
        variant="borderless"
        :styles="composerInputStyles"
        @press-enter="handleSend"
      />
      <div class="composer-toolbar">
        <div class="composer-options">
          <ASelect
            v-if="modelOptions.length > 1"
            :value="activeModel?.name"
            :options="modelOptions"
            :popup-match-select-width="false"
            :aria-label="$gettext('Model')"
            size="small"
            variant="filled"
            class="composer-model"
            @update:value="value => selectedModel = value as string"
          />
          <ASelect
            v-if="availableThinkingLevels.length"
            :value="currentThinkingLevel"
            :options="thinkingOptions"
            :popup-match-select-width="false"
            :aria-label="$gettext('Thinking')"
            size="small"
            variant="filled"
            @update:value="value => thinkingLevel = value as typeof thinkingLevel"
          >
            <template #prefix>
              <BulbOutlined class="composer-option-icon" />
            </template>
          </ASelect>
        </div>
        <div class="composer-actions">
          <ATooltip :title="$gettext('Regenerate response')">
            <AButton
              type="text"
              size="small"
              shape="circle"
              :disabled="loading || !messagesLength"
              :aria-label="$gettext('Regenerate response')"
              @click="llmStore.regenerate(messagesLength - 1, currentLanguage, props.osInfo)"
            >
              <template #icon>
                <RedoOutlined />
              </template>
            </AButton>
          </ATooltip>
          <APopconfirm
            :cancel-text="$gettext('No')"
            :ok-text="$gettext('OK')"
            :title="$gettext('Are you sure you want to clear the record of chat?')"
            :disabled="loading || !messagesLength"
            @confirm="llmStore.clearRecord()"
          >
            <ATooltip :title="$gettext('Clear')">
              <AButton
                type="text"
                size="small"
                shape="circle"
                :disabled="loading || !messagesLength"
                :aria-label="$gettext('Clear')"
              >
                <template #icon>
                  <ClearOutlined />
                </template>
              </AButton>
            </ATooltip>
          </APopconfirm>
          <AButton
            type="primary"
            size="small"
            shape="circle"
            class="composer-send"
            :loading="loading"
            :disabled="!loading && !askBuffer.trim()"
            :aria-label="$gettext('Send')"
            @click="handleButtonClick"
          >
            <template #icon>
              <ArrowUpOutlined />
            </template>
          </AButton>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
// The panel darkens antd's own components without switching the page
// theme, so the composer carries its own palette for both.
.input-msg {
  --composer-area-bg: #ffffff;
  --composer-bg: #ffffff;
  --composer-border: #d9dde4;
  --composer-border-hover: #b8bfca;
  --composer-focus: #1677ff;
  --composer-focus-ring: rgba(22, 119, 255, 0.14);
  --composer-shadow: 0 1px 2px rgba(16, 24, 40, 0.05);

  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  width: 100%;
  box-sizing: border-box;
  // Messages fade out under the composer instead of meeting a hard edge
  padding: 20px 12px 12px;
  background: linear-gradient(to bottom, transparent, var(--composer-area-bg) 20px);
}

.composer {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--composer-border);
  border-radius: 12px;
  background: var(--composer-bg);
  box-shadow: var(--composer-shadow);
  transition: border-color .15s ease-out, box-shadow .15s ease-out;

  &:hover {
    border-color: var(--composer-border-hover);
  }

  &:focus-within {
    border-color: var(--composer-focus);
    box-shadow: 0 0 0 3px var(--composer-focus-ring);
  }
}

.composer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 8px 8px;
}

.composer-options {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

// A step below the 14px label so the icon does not outweigh the text
.composer-option-icon {
  font-size: 12px;
  opacity: .65;
}

.composer-model {
  max-width: 11rem;
}

.composer-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-left: auto;
}

.composer-send {
  margin-left: 4px;
}

.dark .input-msg {
  --composer-area-bg: #141414;
  --composer-bg: #1d1d1f;
  --composer-border: #34363b;
  --composer-border-hover: #4a4d54;
  --composer-focus: #3c89e8;
  --composer-focus-ring: rgba(60, 137, 232, 0.22);
  --composer-shadow: none;
}

@media (prefers-reduced-motion: reduce) {
  .composer {
    transition: none;
  }
}
</style>
