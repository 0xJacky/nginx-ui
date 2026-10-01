<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { Snippet } from '@/api/snippet'
import { CheckOutlined, CloudSyncOutlined, CopyOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@antdv-next/icons'
import { breakpointsAntDesign, createReusableTemplate, useBreakpoints, useClipboard } from '@vueuse/core'
import dayjs from 'dayjs'
import snippet from '@/api/snippet'
import ReactiveFromNow from '@/components/ReactiveFromNow'
import { formatDateTime } from '@/lib/helper'
import SnippetEditor from './components/SnippetEditor.vue'
import SnippetSync from './components/SnippetSync.vue'
import SnippetUsage from './components/SnippetUsage.vue'
import { useSnippetDescription } from './description'

const { message, modal } = App.useApp()
const { describe } = useSnippetDescription()
const { copy, copied, isSupported: canCopy } = useClipboard({ legacy: true, copiedDuring: 1500 })
const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('md')

const snippets = ref<Snippet[]>([])
const syncNodeCount = ref(0)
const isLoading = ref(false)
const isEditorOpen = ref(false)
const isSyncOpen = ref(false)
const editingFile = ref<string>()
const copiedFile = ref('')
const filterText = ref('')

const columns = computed<TableColumnsType<Snippet>>(() => isNarrow.value
  ? [{ title: () => $gettext('Snippet'), key: 'compact' }]
  : [
      { title: () => $gettext('Name'), key: 'name' },
      { title: () => $gettext('File'), key: 'file', width: 280 },
      { title: () => $gettext('Used By'), key: 'used_by', width: 300 },
      { title: () => $gettext('Updated at'), key: 'modified_at', width: 170, responsive: ['lg'] },
      { title: () => $gettext('Actions'), key: 'actions', width: 130, fixed: 'right' },
    ])

const filteredSnippets = computed(() => {
  const text = filterText.value.trim().toLowerCase()
  if (!text)
    return snippets.value
  return snippets.value.filter(s => s.name.toLowerCase().includes(text)
    || s.file.toLowerCase().includes(text)
    || describe(s.description).toLowerCase().includes(text))
})

function variableCount(record: Snippet) {
  return Object.keys(record.variables).length
}

async function loadData() {
  isLoading.value = true
  try {
    const [list, sync] = await Promise.all([snippet.getList(), snippet.getSync()])
    snippets.value = list.data ?? []
    syncNodeCount.value = sync.sync_node_ids?.length ?? 0
  }
  finally {
    isLoading.value = false
  }
}

onMounted(loadData)

function openCreate() {
  editingFile.value = undefined
  isEditorOpen.value = true
}

function openEdit(file: string) {
  editingFile.value = file
  isEditorOpen.value = true
}

// A click anywhere on a row opens the snippet, except on its own controls.
function rowProps(record: Snippet) {
  return {
    class: 'cursor-pointer',
    onClick: (event: MouseEvent) => {
      if (!(event.target as HTMLElement).closest('button, a, .ant-tag'))
        openEdit(record.file)
    },
  }
}

async function copyInclude(record: Snippet) {
  await copy(record.include)
  copiedFile.value = record.file
  message.success($gettext('Copied %{include}', { include: record.include }))
}

function confirmDelete(record: Snippet) {
  modal.confirm({
    title: $gettext('Delete snippet %{name}?', { name: record.name }),
    content: syncNodeCount.value > 0
      ? $gettext('snippets/%{file} will be removed here and on the synchronized nodes.', { file: record.file })
      : $gettext('snippets/%{file} will be removed.', { file: record.file }),
    okText: $gettext('Delete'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    async onOk() {
      await snippet.delete(record.file)
      message.success($gettext('Snippet %{name} deleted', { name: record.name }))
      await loadData()
    },
  })
}

// Cell fragments shared by the full and the compact layout.
const [DefineName, NameCell] = createReusableTemplate<{ record: Snippet }>()
const [DefineFile, FileCell] = createReusableTemplate<{ record: Snippet }>()
const [DefineUsage, UsageCell] = createReusableTemplate<{ record: Snippet }>()
const [DefineActions, ActionsCell] = createReusableTemplate<{ record: Snippet }>()
</script>

<template>
  <ACard :title="$gettext('Snippets')">
    <template #extra>
      <ASpace wrap>
        <AButton
          :loading="isLoading"
          :aria-label="$gettext('Reload')"
          @click="loadData"
        >
          <template #icon>
            <ReloadOutlined />
          </template>
        </AButton>
        <AButton
          :aria-label="$gettext('Synchronize')"
          @click="isSyncOpen = true"
        >
          <template #icon>
            <CloudSyncOutlined />
          </template>
          <span class="max-sm:hidden">
            {{ syncNodeCount > 0
              ? $ngettext('Synchronized to %{count} node', 'Synchronized to %{count} nodes', syncNodeCount, { count: String(syncNodeCount) })
              : $gettext('Synchronize') }}
          </span>
        </AButton>
        <AButton
          type="primary"
          :aria-label="$gettext('Create Snippet')"
          @click="openCreate"
        >
          <template #icon>
            <PlusOutlined />
          </template>
          <span class="max-sm:hidden">{{ $gettext('Create Snippet') }}</span>
        </AButton>
      </ASpace>
    </template>

    <DefineName v-slot="{ record }">
      <div class="flex min-w-0 flex-col gap-0.5">
        <div class="flex flex-wrap items-center gap-2">
          <span class="font-medium">{{ record.name }}</span>
          <ATooltip
            v-if="variableCount(record) > 0"
            :title="$gettext('Fill in the variables by inserting the snippet from the config template panel of the site editor.')"
          >
            <ATag
              color="gold"
              :bordered="false"
              class="m-0"
            >
              {{ $ngettext('%{count} variable', '%{count} variables', variableCount(record), { count: String(variableCount(record)) }) }}
            </ATag>
          </ATooltip>
        </div>
        <span
          v-if="describe(record.description)"
          class="text-sm text-gray-500 dark:text-gray-400"
        >
          {{ describe(record.description) }}
        </span>
      </div>
    </DefineName>

    <DefineFile v-slot="{ record }">
      <div class="flex min-w-0 items-center gap-1">
        <span class="truncate font-mono text-xs text-gray-500 dark:text-gray-400">snippets/{{ record.file }}</span>
        <ATooltip
          v-if="canCopy && variableCount(record) === 0"
          :title="$gettext('Copy the include directive')"
        >
          <AButton
            type="text"
            size="small"
            :aria-label="$gettext('Copy the include directive')"
            @click="copyInclude(record)"
          >
            <template #icon>
              <CheckOutlined
                v-if="copied && copiedFile === record.file"
                class="text-green-500"
              />
              <CopyOutlined v-else />
            </template>
          </AButton>
        </ATooltip>
      </div>
    </DefineFile>

    <DefineUsage v-slot="{ record }">
      <SnippetUsage :used-by="record.used_by" />
    </DefineUsage>

    <DefineActions v-slot="{ record }">
      <ASpace :size="0">
        <AButton
          type="link"
          size="small"
          @click="openEdit(record.file)"
        >
          {{ $gettext('Edit') }}
        </AButton>
        <ATooltip
          :title="record.used_by.length > 0
            ? $gettext('Remove the snippet from the files that include it first.')
            : undefined"
        >
          <AButton
            type="link"
            size="small"
            danger
            :disabled="record.used_by.length > 0"
            @click="confirmDelete(record)"
          >
            {{ $gettext('Delete') }}
          </AButton>
        </ATooltip>
      </ASpace>
    </DefineActions>

    <p class="mt-0 mb-4 text-gray-500 dark:text-gray-400">
      {{ $gettext('A snippet keeps a piece of Nginx configuration in one place. Include it in a site, or insert it from the config template panel of the site editor; changing an included snippet updates every site that uses it.') }}
    </p>

    <AInput
      v-if="snippets.length > 0"
      v-model:value="filterText"
      :placeholder="$gettext('Search snippets')"
      allow-clear
      class="mb-4 max-w-80"
    >
      <template #prefix>
        <SearchOutlined />
      </template>
    </AInput>

    <ATable
      :columns="columns"
      :data-source="filteredSnippets"
      :loading="isLoading"
      :pagination="false"
      :scroll="isNarrow ? undefined : { x: 900 }"
      :custom-row="rowProps"
      row-key="file"
    >
      <template #emptyText>
        <AEmpty
          v-if="!filterText"
          :description="$gettext('No snippets yet')"
        >
          <AButton
            type="primary"
            @click="openCreate"
          >
            <template #icon>
              <PlusOutlined />
            </template>
            {{ $gettext('Create Snippet') }}
          </AButton>
        </AEmpty>
        <AEmpty
          v-else
          :description="$gettext('No snippet matches the search')"
        />
      </template>

      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'compact'">
          <div class="flex flex-col gap-2">
            <div class="flex items-start justify-between gap-2">
              <NameCell :record />
              <ActionsCell :record />
            </div>
            <FileCell :record />
            <UsageCell :record />
          </div>
        </template>

        <template v-else-if="column.key === 'name'">
          <NameCell :record />
        </template>

        <template v-else-if="column.key === 'file'">
          <FileCell :record />
        </template>

        <template v-else-if="column.key === 'used_by'">
          <UsageCell :record />
        </template>

        <template v-else-if="column.key === 'modified_at'">
          <ATooltip :title="formatDateTime(record.modified_at)">
            <ReactiveFromNow :time="dayjs(record.modified_at).unix()" />
          </ATooltip>
        </template>

        <template v-else-if="column.key === 'actions'">
          <ActionsCell :record />
        </template>
      </template>
    </ATable>

    <SnippetEditor
      v-model:open="isEditorOpen"
      :file="editingFile"
      @saved="loadData"
    />
    <SnippetSync
      v-model:open="isSyncOpen"
      @saved="loadData"
    />
  </ACard>
</template>
