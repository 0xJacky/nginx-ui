<script setup lang="ts">
import { VAceEditor } from 'vue3-ace-editor'
import 'ace-builds/src-noconflict/mode-nginx'
import ace from 'ace-builds'
import 'ace-builds/src-noconflict/theme-monokai'
import extSearchboxUrl from 'ace-builds/src-noconflict/ext-searchbox?url'
import { computed } from 'vue'

const props = defineProps<{
  content?: string
  defaultHeight?: string
  readonly?: boolean
  placeholder?: string
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

ace.config.setModuleUrl('ace/ext/searchbox', extSearchboxUrl)
</script>

<template>
  <VAceEditor
    ref="aceRef"
    v-model:value="value"
    lang="nginx"
    theme="monokai"
    :style="{
      minHeight: defaultHeight || '100vh',
      borderRadius: '5px',
    }"
    :readonly="readonly"
    :placeholder="placeholder"
  />
</template>

<style scoped>
:deep(.ace_placeholder) {
  z-index: 1;
  position: relative;
}
</style>
