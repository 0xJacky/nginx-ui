<script setup lang="ts">
import { BulbOutlined, LoadingOutlined, SendOutlined } from '@antdv-next/icons'
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

  if (!askBuffer.value.trim())
    return
  llmStore.send(askBuffer.value, currentLanguage.value, props.osInfo)
}

function handleButtonClick() {
  if (!askBuffer.value.trim())
    return
  llmStore.send(askBuffer.value, currentLanguage.value, props.osInfo)
}
</script>

<template>
  <div ref="inputContainerRef" class="input-msg">
    <div
      class="control-btn"
      :class="{ 'has-options': modelOptions.length > 1 || availableThinkingLevels.length }"
    >
      <ASpace
        v-if="modelOptions.length > 1 || availableThinkingLevels.length"
        size="small"
        wrap
      >
        <ASelect
          v-if="modelOptions.length > 1"
          :value="activeModel?.name"
          :options="modelOptions"
          :popup-match-select-width="false"
          :title="$gettext('Model')"
          size="small"
          variant="borderless"
          class="max-w-44"
          @update:value="value => selectedModel = value as string"
        />
        <ASelect
          v-if="availableThinkingLevels.length"
          :value="currentThinkingLevel"
          :options="thinkingOptions"
          :popup-match-select-width="false"
          :title="$gettext('Thinking')"
          size="small"
          variant="borderless"
          @update:value="value => thinkingLevel = value as typeof thinkingLevel"
        >
          <template #prefix>
            <BulbOutlined />
          </template>
        </ASelect>
      </ASpace>
      <ASpace v-show="!loading">
        <APopconfirm
          :cancel-text="$gettext('No')"
          :ok-text="$gettext('OK')"
          :title="$gettext('Are you sure you want to clear the record of chat?')"
          @confirm="llmStore.clearRecord()"
        >
          <AButton type="text">
            {{ $gettext('Clear') }}
          </AButton>
        </APopconfirm>
        <AButton
          type="text"
          @click="llmStore.regenerate(messagesLength - 1, currentLanguage, props.osInfo)"
        >
          {{ $gettext('Regenerate response') }}
        </AButton>
      </ASpace>
    </div>
    <ATextarea
      v-model:value="askBuffer"
      :auto-size="{ minRows: 1, maxRows: 6 }"
      :placeholder="$gettext('Type your message here...')"
      @press-enter="handleSend"
    />
    <div class="send-btn">
      <AButton
        size="small"
        type="text"
        :disabled="loading || !askBuffer"
        @click="handleButtonClick"
      >
        <LoadingOutlined v-if="loading" spin />
        <SendOutlined v-else />
      </AButton>
    </div>
  </div>
</template>

<style lang="less" scoped>
.input-msg {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  padding: 16px;
  border-top: 1px solid var(--ant-color-border);
  width: 100%;
  box-sizing: border-box;
  z-index: 100;
  box-shadow: 0 0 10px rgba(0, 0, 0, 0.04);

  .control-btn {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 4px;

    &.has-options {
      justify-content: space-between;
    }
  }

  :deep(.ant-input) {
    padding-right: 50px; // 为发送按钮预留空间
    resize: none;
    min-height: 32px;
    line-height: 1.5;
  }

  :deep(.ant-input-textarea) {
    .ant-input {
      min-height: 32px !important;
    }
  }

  .send-btn {
    position: absolute;
    right: 16px;
    bottom: 19px;
  }
}

.dark {
  .input-msg {
    background: rgba(30, 30, 30, 0.8);
    border-top: 1px solid #333;
  }
}
</style>
