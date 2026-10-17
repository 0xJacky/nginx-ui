<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { BuiltinTemplate, Snippet } from '@/api/snippet'
import { CheckOutlined, CloudSyncOutlined, CopyOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@antdv-next/icons'
import { breakpointsAntDesign, createReusableTemplate, useBreakpoints, useClipboard } from '@vueuse/core'
import dayjs from 'dayjs'
import snippet from '@/api/snippet'
import ReactiveFromNow from '@/components/ReactiveFromNow'
import { formatDateTime } from '@/lib/helper'
import BuiltinTemplates from './components/BuiltinTemplates.vue'
import SnippetPeek from './components/SnippetPeek.vue'
import SnippetSync from './components/SnippetSync.vue'
import SnippetUsage from './components/SnippetUsage.vue'
import { localize, useSnippetDescription } from './description'

const { message, modal } = App.useApp()
const { current: language, describe } = useSnippetDescription()
const { copy, copied, isSupported: canCopy } = useClipboard({ legacy: true, copiedDuring: 1500 })
const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('md')

const snippets = ref<Snippet[]>([])
const syncNodeCount = ref(0)
const isLoading = ref(false)
const isSyncOpen = ref(false)
const route = useRoute()
const router = useRouter()
const builtins = ref<BuiltinTemplate[]>([])
const isBuiltinLoading = ref(false)
// The tab is kept in the address, so going back from a copy returns to it.
const tab = computed({
  get: () => route.query.tab === 'builtin' ? 'builtin' : 'snippets',
  set: value => router.replace({ query: value === 'builtin' ? { tab: 'builtin' } : {} }),
})
const tabOptions = computed(() => [
  { label: `${$gettext('My Snippets')} ${snippets.value.length}`, value: 'snippets' },
  { label: `${$gettext('Templates')} ${builtins.value.length || ''}`.trim(), value: 'builtin' },
])
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
  return snippets.value.filter(s => nameOf(s).toLowerCase().includes(text)
    || s.file.toLowerCase().includes(text)
    || describe(s.description).toLowerCase().includes(text))
})

function nameOf(record: Snippet) {
  return localize(record.name_i18n, language.value) || record.name
}

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

async function loadBuiltins() {
  isBuiltinLoading.value = true
  try {
    builtins.value = (await snippet.getBuiltins()).data ?? []
  }
  finally {
    isBuiltinLoading.value = false
  }
}

onMounted(() => {
  loadData()
  loadBuiltins()
})

function openCreate() {
  router.push('/sites/snippets/add')
}

function openEdit(file: string) {
  router.push(`/sites/snippets/${encodeURIComponent(file)}`)
}

// A click anywhere on a row opens the snippet, except on its own controls.
function rowProps(record: Snippet) {
  return {
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
    title: $gettext('Delete snippet %{name}?', { name: nameOf(record) }),
    content: syncNodeCount.value > 0
      ? $gettext('snippets/%{file} will be removed here and on the synchronized nodes.', { file: record.file })
      : $gettext('snippets/%{file} will be removed.', { file: record.file }),
    okText: $gettext('Delete'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    async onOk() {
      await snippet.delete(record.file)
      message.success($gettext('Snippet %{name} deleted', { name: nameOf(record) }))
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
          <span class="font-medium">{{ nameOf(record) }}</span>
          <ATooltip
            :title="variableCount(record) > 0
              ? $ngettext(
                'Has %{count} variable. Sites insert a filled in copy from the config template panel of the site editor.',
                'Has %{count} variables. Sites insert a filled in copy from the config template panel of the site editor.',
                variableCount(record),
                { count: String(variableCount(record)) },
              )
              : $gettext('Sites can include this snippet and follow its later changes.')"
          >
            <ATag
              :color="variableCount(record) > 0 ? 'gold' : 'green'"
              :bordered="false"
              class="m-0"
            >
              {{ variableCount(record) > 0 ? $gettext('Insert Only') : $gettext('Includable') }}
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

    <div class="mb-4 flex flex-wrap items-center gap-3">
      <ASegmented
        v-model:value="tab"
        :options="tabOptions"
      />
      <AInput
        v-model:value="filterText"
        :placeholder="tab === 'builtin' ? $gettext('Search templates') : $gettext('Search snippets')"
        allow-clear
        class="max-w-80"
      >
        <template #prefix>
          <SearchOutlined />
        </template>
      </AInput>
    </div>

    <BuiltinTemplates
      v-if="tab === 'builtin'"
      :templates="builtins"
      :loading="isBuiltinLoading"
      :filter-text="filterText"
    />

    <ATable
      v-else
      :columns="columns"
      :data-source="filteredSnippets"
      :loading="isLoading"
      :pagination="false"
      :scroll="isNarrow ? undefined : { x: 900 }"
      :on-row="rowProps"
      row-class-name="cursor-pointer"
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

      <template #expandedRowRender="{ record }">
        <SnippetPeek :file="record.file" />
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

    <SnippetSync
      v-model:open="isSyncOpen"
      @saved="loadData"
    />
  </ACard>
</template>
