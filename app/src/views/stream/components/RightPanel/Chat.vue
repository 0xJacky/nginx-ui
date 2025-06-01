<script setup lang="ts">
import ChatGPT from '@/components/ChatGPT'
import { useStreamEditorStore } from '../../store'

const store = useStreamEditorStore()
const { configText, filepath } = storeToRefs(store)
</script>

<template>
  <div>
    <ChatGPT
      :content="configText"
      :path="filepath"
    />
  </div>
</template>
