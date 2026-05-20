<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { computed } from 'vue'

interface Props {
  code: string
  language?: string
  title?: string
}

const props = withDefaults(defineProps<Props>(), {
  language: 'shell',
  title: '',
})

const { copy, copied } = useClipboard()
const trimmed = computed(() => props.code.trimEnd())
</script>

<template>
  <div class="rounded border border-slate-200 dark:border-slate-700 overflow-hidden">
    <div class="flex items-center justify-between px-3 py-2 bg-slate-50 dark:bg-slate-800">
      <span class="text-xs font-medium text-slate-600 dark:text-slate-300">
        {{ title || language }}
      </span>
      <AButton
        size="small"
        :type="copied ? 'primary' : 'default'"
        @click="copy(trimmed)"
      >
        {{ copied ? $gettext('Copied') : $gettext('Copy') }}
      </AButton>
    </div>
    <pre class="m-0 p-3 overflow-x-auto text-sm leading-snug"><code>{{ trimmed }}</code></pre>
  </div>
</template>
