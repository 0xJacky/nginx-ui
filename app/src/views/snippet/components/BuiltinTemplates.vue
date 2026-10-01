<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { BuiltinTemplate } from '@/api/snippet'
import { CopyOutlined } from '@antdv-next/icons'
import { breakpointsAntDesign, useBreakpoints } from '@vueuse/core'
import { useSnippetDescription } from '../description'
import SnippetPeek from './SnippetPeek.vue'

const props = defineProps<{
  templates: BuiltinTemplate[]
  loading: boolean
  filterText: string
}>()

const router = useRouter()
const { describe } = useSnippetDescription()
const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('md')

const columns = computed<TableColumnsType<BuiltinTemplate>>(() => isNarrow.value
  ? [{ title: () => $gettext('Template'), key: 'compact' }]
  : [
      { title: () => $gettext('Name'), key: 'name' },
      { title: () => $gettext('Author'), key: 'author', width: 180, responsive: ['lg'] },
      { title: () => $gettext('Actions'), key: 'actions', width: 220, fixed: 'right' },
    ])

const filtered = computed(() => {
  const text = props.filterText.trim().toLowerCase()
  if (!text)
    return props.templates
  return props.templates.filter(t => t.name.toLowerCase().includes(text)
    || t.filename.toLowerCase().includes(text)
    || describe(t.description).toLowerCase().includes(text))
})

function variableCount(template: BuiltinTemplate) {
  return Object.keys(template.variables ?? {}).length
}

function view(template: BuiltinTemplate) {
  router.push(`/sites/snippets/templates/${encodeURIComponent(template.filename)}`)
}

// A click anywhere on a row opens the template, except on its own controls.
function rowProps(record: BuiltinTemplate) {
  return {
    onClick: (event: MouseEvent) => {
      if (!(event.target as HTMLElement).closest('button, a'))
        view(record)
    },
  }
}

function copyAsSnippet(template: BuiltinTemplate) {
  router.push({ path: '/sites/snippets/add', query: { from: template.filename } })
}
</script>

<template>
  <ATable
    :columns="columns"
    :data-source="filtered"
    :loading="loading"
    :pagination="false"
    :on-row="rowProps"
    row-class-name="cursor-pointer"
    row-key="filename"
  >
    <template #emptyText>
      <AEmpty :description="$gettext('No template matches the search')" />
    </template>

    <template #expandedRowRender="{ record }">
      <SnippetPeek
        :file="record.filename"
        builtin
      />
    </template>

    <template #bodyCell="{ column, record }">
      <div
        v-if="column.key === 'name' || column.key === 'compact'"
        class="flex min-w-0 flex-col gap-0.5"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="font-medium">{{ record.name || record.filename }}</span>
          <ATag
            v-if="variableCount(record) > 0"
            color="gold"
            :bordered="false"
            class="m-0"
          >
            {{ $ngettext('%{count} variable', '%{count} variables', variableCount(record), { count: String(variableCount(record)) }) }}
          </ATag>
        </div>
        <span
          v-if="describe(record.description)"
          class="hint text-sm"
        >
          {{ describe(record.description) }}
        </span>
        <div
          v-if="column.key === 'compact'"
          class="mt-1"
        >
          <AButton
            size="small"
            @click="copyAsSnippet(record as BuiltinTemplate)"
          >
            <template #icon>
              <CopyOutlined />
            </template>
            {{ $gettext('Copy as Snippet') }}
          </AButton>
        </div>
      </div>
      <span
        v-else-if="column.key === 'author'"
        class="hint"
      >{{ record.author }}</span>
      <ASpace
        v-else-if="column.key === 'actions'"
        :size="0"
      >
        <AButton
          type="link"
          size="small"
          @click="view(record as BuiltinTemplate)"
        >
          {{ $gettext('View') }}
        </AButton>
        <AButton
          type="link"
          size="small"
          @click="copyAsSnippet(record as BuiltinTemplate)"
        >
          {{ $gettext('Copy as Snippet') }}
        </AButton>
      </ASpace>
    </template>
  </ATable>
</template>

<style scoped lang="less">
.hint {
  color: var(--ant-color-text-secondary);
}
</style>
