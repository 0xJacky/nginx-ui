<script setup lang="ts">
import LLM from '@/components/LLM'
import { useStreamEditorStore } from '../../store'

const store = useStreamEditorStore()
const { configText, filepath } = storeToRefs(store)
</script>

<template>
  <div class="mt--6">
    <LLM
      :content="configText"
      :path="filepath"
    />
  </div>
</template>
