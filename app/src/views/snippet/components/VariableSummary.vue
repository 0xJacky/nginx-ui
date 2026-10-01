<script setup lang="ts">
import type { VariableRow, VariableType } from '../variables'
import { localize, useSnippetDescription } from '../description'
import { colorOf, colorSlots } from '../template'

const props = defineProps<{
  rows: VariableRow[]
  // Also shows the key and the default value of each variable.
  detailed?: boolean
}>()

const { current: language } = useSnippetDescription()
const slots = computed(() => colorSlots(props.rows))

const typeLabels = computed<Record<VariableType, string>>(() => ({
  string: $gettext('Text'),
  boolean: $gettext('Switch'),
  select: $gettext('Select'),
}))

function defaultOf(row: VariableRow) {
  if (row.type === 'boolean')
    return row.value ? $gettext('On') : $gettext('Off')
  if (row.type === 'select') {
    const option = row.options.find(o => o.value === row.value)
    return option ? localize(option.labels, language.value) || option.value : ''
  }
  return String(row.value)
}
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <div
      v-for="row in rows"
      :key="row.id"
      class="variable"
      :style="{ '--c': colorOf(slots, row.key) }"
    >
      <span class="flex min-w-0 flex-1 flex-col">
        <span class="truncate">{{ localize(row.names, language) || row.key }}</span>
        <span
          v-if="detailed"
          class="key truncate font-mono text-xs"
        >.{{ row.key }}</span>
      </span>
      <span class="flex shrink-0 items-center gap-1">
        <span class="hint text-xs">{{ typeLabels[row.type] }}</span>
        <span
          v-if="detailed && defaultOf(row)"
          class="hint max-w-28 truncate text-xs"
        >{{ defaultOf(row) }}</span>
      </span>
    </div>
  </div>
</template>

<style scoped lang="less">
.variable {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px;
  border: 1px solid var(--ant-color-border-secondary);
  border-left: 3px solid var(--c);
  border-radius: 6px;
  background: var(--ant-color-bg-container);
}

.key {
  color: color-mix(in srgb, var(--c) 75%, var(--ant-color-text));
}

.hint {
  color: var(--ant-color-text-secondary);
}
</style>
