<script setup lang="ts">
import type { BuiltinTemplate, Snippet } from '@/api/snippet'
import snippet from '@/api/snippet'
import { colorSlots } from '../template'
import { toRows } from '../variables'
import CodeView from './CodeView.vue'
import VariableSummary from './VariableSummary.vue'

const props = defineProps<{
  file: string
  // The file names a block template built into Nginx UI.
  builtin?: boolean
}>()

const detail = ref<Pick<Snippet | BuiltinTemplate, 'content' | 'variables'>>()
const isLoading = ref(true)

onMounted(async () => {
  try {
    detail.value = props.builtin ? await snippet.getBuiltin(props.file) : await snippet.get(props.file)
  }
  finally {
    isLoading.value = false
  }
})

const rows = computed(() => toRows(detail.value?.variables))
const slots = computed(() => colorSlots(rows.value))
</script>

<template>
  <ASpin :spinning="isLoading">
    <div
      v-if="detail"
      class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_16rem]"
    >
      <CodeView
        :content="detail.content ?? ''"
        :slots="slots"
        class="max-h-80 overflow-y-auto"
      />
      <VariableSummary
        v-if="rows.length > 0"
        :rows="rows"
      />
    </div>
  </ASpin>
</template>
