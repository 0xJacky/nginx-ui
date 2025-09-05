<script setup lang="ts">
import type { Config } from '@/api/config'
import LLM from '@/components/LLM'

interface Props {
  chatHeight: string
}

defineProps<Props>()
const data = defineModel<Config>('data', { required: true })
</script>

<template>
  <div class="mt--6">
    <LLM
      :content="data.content"
      :path="data.filepath"
      :height="chatHeight"
    />
  </div>
</template>
