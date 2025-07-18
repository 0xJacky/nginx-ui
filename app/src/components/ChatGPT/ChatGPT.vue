<script setup lang="ts">
import Icon from '@ant-design/icons-vue'
import { useElementVisibility } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import ChatGPT_logo from '@/assets/svg/ChatGPT_logo.svg?component'
import { useSettingsStore } from '@/pinia'
import { useChatGPTStore } from './chatgpt'
import ChatMessageInput from './ChatMessageInput.vue'
import ChatMessageList from './ChatMessageList.vue'

const props = defineProps<{
  content: string
  path?: string
}>()

const { language: current } = storeToRefs(useSettingsStore())

// Use ChatGPT store
const chatGPTStore = useChatGPTStore()
const { messageContainerRef, loading, shouldShowStartButton } = storeToRefs(chatGPTStore)

// Initialize messages when path changes
watch(() => props.path, async () => {
  await chatGPTStore.initMessages(props.path)
  await nextTick()
}, { immediate: true })

// Send message handler
async function handleSend() {
  await chatGPTStore.send(props.content, current.value)
}

const isVisible = useElementVisibility(messageContainerRef)

watch(isVisible, visible => {
  if (visible) {
    chatGPTStore.scrollToBottom()
  }
}, { immediate: true })
</script>

<template>
  <div
    v-if="shouldShowStartButton"
    class="chat-start m-4"
  >
    <AButton
      :loading="loading"
      @click="handleSend"
    >
      <Icon
        v-if="!loading"
        :component="ChatGPT_logo"
      />
      {{ $gettext('Ask ChatGPT for Help') }}
    </AButton>
  </div>

  <div
    v-else
    ref="messageContainerRef"
    class="message-container"
  >
    <ChatMessageList />

    <ChatMessageInput />
  </div>
</template>

<style lang="less" scoped>
.message-container {
  margin: 0 auto;
  max-width: 800px;
  max-height: calc(100vh - 260px);
  overflow-y: auto;
}
</style>
