<script setup lang="ts">
import type { Config } from '@/api/config'
import LLM from '@/components/LLM'

const data = defineModel<Config>('data', { required: true })
</script>

<template>
  <div class="mt--6">
    <LLM
      :content="data.content"
      :path="data.filepath"
    />
  </div>
</template>
