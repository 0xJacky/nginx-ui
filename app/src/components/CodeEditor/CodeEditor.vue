<script setup lang="ts">
import { VAceEditor } from 'vue3-ace-editor'
import 'ace-builds/src-noconflict/mode-nginx'
import 'ace-builds/src-noconflict/theme-monokai'
import { computed } from 'vue'

const props = defineProps<{
  content?: string
  defaultHeight?: string
  readonly?: boolean
}>()

const emit = defineEmits(['update:content'])

const value = computed({
  get() {
    return props.content ?? ''
  },
  set(v) {
    emit('update:content', v)
  },
})
</script>

<template>
  <VAceEditor
    v-model:value="value"
    lang="nginx"
    theme="monokai"
    :style="{
      minHeight: defaultHeight || '100vh',
      borderRadius: '5px',
    }"
    :readonly="readonly"
  />
</template>

<style scoped>

</style>
