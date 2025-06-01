<script setup lang="ts">
import type { Config } from '@/api/config'
import ChatGPT from '@/components/ChatGPT'

const data = defineModel<Config>('data', { required: true })
</script>

<template>
  <div class="mt--6">
    <ChatGPT
      :content="data.content"
      :path="data.filepath"
    />
  </div>
</template>
