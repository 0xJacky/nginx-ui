<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { AccessList } from '@/api/access_list'
import { ExclamationCircleOutlined, PlusOutlined, ReloadOutlined } from '@antdv-next/icons'
import accessList from '@/api/access_list'
import { formatDateTime } from '@/lib/helper'
import AccessListEditor from './components/AccessListEditor.vue'
import { warningText } from './warnings'

const { message, modal } = App.useApp()

const lists = ref<AccessList[]>([])
const isLoading = ref(false)
const isEditorOpen = ref(false)
const editingId = ref<number>()

const columns: TableColumnsType<AccessList> = [
  { title: () => $gettext('Name'), key: 'name', width: 220 },
  { title: () => $gettext('Rules'), key: 'rules' },
  { title: () => $gettext('Everything else'), key: 'fallback', width: 140 },
  { title: () => $gettext('Used By'), key: 'used_by', width: 110 },
  { title: () => $gettext('Updated at'), key: 'updated_at', width: 170 },
  { title: () => $gettext('Actions'), key: 'actions', width: 140, fixed: 'right' },
]

const byId = computed(() => {
  const map: Record<number, AccessList> = {}
  lists.value.forEach(l => {
    map[l.id] = l
  })
  return map
})

async function loadData() {
  isLoading.value = true
  try {
    lists.value = (await accessList.getAll()).data ?? []
  }
  finally {
    isLoading.value = false
  }
}

onMounted(loadData)

function openCreate() {
  editingId.value = undefined
  isEditorOpen.value = true
}

function openEdit(id: number) {
  editingId.value = id
  isEditorOpen.value = true
}

function ruleLabel(record: AccessList, index: number) {
  const rule = record.rules[index]
  if (rule.type === 'ref')
    return $gettext('List %{name}', { name: byId.value[rule.ref_id ?? 0]?.name ?? '?' })
  return `${rule.type} ${rule.value}`
}

function confirmDelete(record: AccessList) {
  // Nginx fails to load a site that includes a missing file, so a list in use
  // is never deleted; the backend refuses it too.
  if ((record.used_by ?? 0) > 0) {
    modal.warning({
      title: $gettext('Access list %{name} is still in use', { name: record.name }),
      content: $gettext('Open the list to see which lists, sites and streams use it, and switch them to another list first.'),
      okText: $gettext('OK'),
    })
    return
  }

  modal.confirm({
    title: $gettext('Delete access list %{name}?', { name: record.name }),
    content: $gettext('nginx-ui/access/%{slug}.conf will be removed.', { slug: record.slug }),
    okText: $gettext('Delete'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    async onOk() {
      await accessList.deleteItem(record.id)
      message.success($gettext('Access list %{name} deleted', { name: record.name }))
      await loadData()
    },
  })
}
</script>

<template>
  <ACard :title="$gettext('Access Lists')">
    <template #extra>
      <ASpace>
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
          type="primary"
          data-testid="access-list-create"
          @click="openCreate"
        >
          <template #icon>
            <PlusOutlined />
          </template>
          {{ $gettext('Create Access List') }}
        </AButton>
      </ASpace>
    </template>

    <p class="mt-0 mb-4 text-gray-500 dark:text-gray-400">
      {{ $gettext('An access list keeps IP addresses in one place. Choose it for a server or a location in the site editor; changing the list here updates every site that uses it.') }}
    </p>

    <ATable
      :columns="columns"
      :data-source="lists"
      :loading="isLoading"
      :pagination="false"
      :scroll="{ x: 900 }"
      row-key="id"
      data-testid="access-list-table"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'">
          <div class="flex flex-col">
            <span class="font-medium">
              {{ record.name }}
              <ATooltip
                v-if="record.warnings?.length"
                :title="record.warnings.map(warningText).join('\n')"
              >
                <ExclamationCircleOutlined class="ml-1 text-amber-500" />
              </ATooltip>
            </span>
            <span class="font-mono text-xs text-gray-500 dark:text-gray-400">{{ record.slug }}</span>
          </div>
        </template>

        <template v-else-if="column.key === 'rules'">
          <div class="flex flex-wrap gap-1">
            <ATag
              v-for="(rule, index) in record.rules.slice(0, 6)"
              :key="index"
              :color="rule.type === 'allow' ? 'green' : rule.type === 'deny' ? 'red' : 'purple'"
              :bordered="false"
              class="font-mono"
            >
              {{ ruleLabel(record, index) }}
            </ATag>
            <ATag v-if="record.rules.length > 6" :bordered="false">
              {{ $gettext('+%{count} more', { count: String(record.rules.length - 6) }) }}
            </ATag>
          </div>
        </template>

        <template v-else-if="column.key === 'fallback'">
          <ATag :color="record.fallback === 'deny' ? 'red' : 'green'" :bordered="false">
            {{ record.fallback === 'deny' ? $gettext('Deny') : $gettext('Allow') }}
          </ATag>
        </template>

        <template v-else-if="column.key === 'used_by'">
          <span v-if="record.used_by">{{ record.used_by }}</span>
          <span v-else class="text-gray-500 dark:text-gray-400">{{ $gettext('Not used') }}</span>
        </template>

        <template v-else-if="column.key === 'updated_at'">
          {{ formatDateTime(record.updated_at) }}
        </template>

        <template v-else-if="column.key === 'actions'">
          <ASpace :size="0">
            <AButton
              type="link"
              size="small"
              :data-testid="`access-list-edit-${record.slug}`"
              @click="openEdit(record.id)"
            >
              {{ $gettext('Edit') }}
            </AButton>
            <AButton
              type="link"
              size="small"
              danger
              :data-testid="`access-list-delete-${record.slug}`"
              @click="confirmDelete(record)"
            >
              {{ $gettext('Delete') }}
            </AButton>
          </ASpace>
        </template>
      </template>
    </ATable>

    <AccessListEditor
      :id="editingId"
      v-model:open="isEditorOpen"
      :lists="lists"
      @saved="loadData"
    />
  </ACard>
</template>
