<script setup lang="ts">
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import { useStreamEditorStore } from '../../store'

const store = useStreamEditorStore()
const { configText, filepath, historyChatgptRecord } = storeToRefs(store)
</script>

<template>
  <div>
    <ChatGPT
      v-model:history-messages="historyChatgptRecord"
      :content="configText"
      :path="filepath"
    />
  </div>
</template>
