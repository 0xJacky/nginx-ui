<script setup lang="ts">
import LLM from '@/components/LLM'
import { useStreamEditorStore } from '../../store'

interface Props {
  chatHeight: string
}

defineProps<Props>()
const store = useStreamEditorStore()
const { configText, filepath } = storeToRefs(store)
</script>

<template>
  <div class="mt--6">
    <LLM
      :content="configText"
      :path="filepath"
      :height="chatHeight"
    />
  </div>
</template>
