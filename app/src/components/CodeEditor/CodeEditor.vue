<script setup lang="ts">
import type { Editor } from 'ace-builds'
import ace from 'ace-builds'
import extSearchboxUrl from 'ace-builds/src-noconflict/ext-searchbox?url'
import { VAceEditor } from 'vue3-ace-editor'
import useCodeCompletion from './CodeCompletion'
import 'ace-builds/src-noconflict/mode-nginx'
import 'ace-builds/src-noconflict/theme-monokai'

const props = defineProps<{
  defaultHeight?: string
  readonly?: boolean
  placeholder?: string
  disableCodeCompletion?: boolean
}>()

const content = defineModel<string>('content', { default: '' })

onMounted(() => {
  try {
    ace.config.setModuleUrl('ace/ext/searchbox', extSearchboxUrl)
  }
  catch (error) {
    console.error(`Failed to initialize Ace editor: ${error}`)
  }
})

const codeCompletion = useCodeCompletion()

async function init(editor: Editor) {
  if (props.readonly || props.disableCodeCompletion) {
    return
  }
  codeCompletion.init(editor)
}

onUnmounted(() => {
  codeCompletion.cleanUp()
})
</script>

<template>
  <VAceEditor
    v-model:value="content"
    lang="nginx"
    theme="monokai"
    :style="{
      minHeight: defaultHeight || '100vh',
      borderRadius: '5px',
    }"
    :readonly
    :placeholder
    @init="init"
  />
</template>

<style lang="less" scoped>
:deep(.ace_placeholder) {
  z-index: 1;
  position: relative;
}

:deep(.ace_ghost-text) {
  color: #6a737d;
  opacity: 0.8;
}
</style>
