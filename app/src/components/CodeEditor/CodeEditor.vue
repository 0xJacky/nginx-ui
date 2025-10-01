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
  noBorderRadius?: boolean
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
      borderRadius: props.noBorderRadius ? '0' : '5px',
    }"
    :readonly="props.readonly"
    :placeholder="props.placeholder"
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

/* Loading spinner for code completion */
:deep(.completion-loading-spinner) {
  position: relative;
  background: transparent;
}

:deep(.completion-loading-spinner):before {
  content: '';
  position: absolute;
  top: 50%;
  left: 0;
  width: 12px;
  height: 12px;
  margin-top: -6px;
  border: 2px solid #6a737d;
  border-top: 2px solid transparent;
  border-radius: 50%;
  animation: completion-spin 1s linear infinite;
  z-index: 10;
}

@keyframes completion-spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* Dark theme support */
.dark :deep(.completion-loading-spinner):before {
  border-color: #8b949e;
  border-top-color: transparent;
}
</style>
