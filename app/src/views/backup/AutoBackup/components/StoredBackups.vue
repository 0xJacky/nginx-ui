<script setup lang="ts">
import type { TableColumnType } from 'antdv-next'
import type { AutoBackup, StoredBackup } from '@/api/backup'
import { deleteStoredBackup, listStoredBackups, restoreStoredBackup } from '@/api/backup'
import { formatDateTime } from '@/lib/helper'
import { formatSize, storageTypeLabel } from '../pluginStorage'

// Lists the runs a plugin storage backend keeps for one task, and deletes or
// restores one of them.
const props = defineProps<{
  record?: AutoBackup
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = useGlobalApp()

const loading = ref(false)
const backups = ref<StoredBackup[]>([])
const deleting = ref<Record<string, boolean>>({})

const restoreTarget = ref<StoredBackup>()
const restoring = ref(false)
const restoreOptions = reactive({
  restoreNginx: true,
  restoreNginxUI: true,
  verifyHash: true,
})

const canRestore = computed(() => props.record?.backup_type === 'nginx_and_nginx_ui')

async function load() {
  if (!props.record?.id)
    return
  loading.value = true
  try {
    const res = await listStoredBackups(props.record.id)
    backups.value = res.data ?? []
  }
  catch {
    backups.value = []
  }
  finally {
    loading.value = false
  }
}

watch(open, value => {
  if (value)
    load()
})

async function handleDelete(backup: StoredBackup) {
  if (!props.record?.id)
    return
  deleting.value[backup.key] = true
  try {
    await deleteStoredBackup(props.record.id, backup.key)
    message.success($gettext('Deleted successfully'))
    await load()
  }
  finally {
    deleting.value[backup.key] = false
  }
}

function openRestore(backup: StoredBackup) {
  restoreOptions.restoreNginx = true
  restoreOptions.restoreNginxUI = true
  restoreOptions.verifyHash = true
  restoreTarget.value = backup
}

async function handleRestore() {
  const backup = restoreTarget.value
  if (!props.record?.id || !backup)
    return
  restoring.value = true
  try {
    await restoreStoredBackup(props.record.id, {
      key: backup.key,
      restore_nginx: restoreOptions.restoreNginx,
      restore_nginx_ui: restoreOptions.restoreNginxUI,
      verify_hash: restoreOptions.verifyHash,
    })
    message.success($gettext('Restore completed successfully'))
    restoreTarget.value = undefined
    open.value = false
    if (restoreOptions.restoreNginxUI) {
      // Nginx UI restarts with the restored data.
      message.info($gettext('Please log in.'))
      setTimeout(() => window.location.reload(), 3000)
    }
  }
  finally {
    restoring.value = false
  }
}

function fileName(key: string) {
  return key.split('/').pop() ?? key
}

const columns: TableColumnType<StoredBackup>[] = [
  { title: () => $gettext('Created at'), dataIndex: 'created_at', key: 'created_at' },
  { title: () => $gettext('File'), dataIndex: 'key', key: 'key', ellipsis: true },
  { title: () => $gettext('Size'), dataIndex: 'size', key: 'size', width: 110 },
  { title: () => $gettext('Actions'), key: 'actions', width: 160 },
]
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Stored Backups')"
    :footer="null"
    width="760px"
    destroy-on-hidden
  >
    <div class="mb-4 text-gray-500">
      {{ $gettext('Backups kept in %{storage} under %{prefix}', {
        storage: storageTypeLabel(props.record?.storage_type),
        prefix: props.record?.storage_path || '/',
      }) }}
    </div>

    <ATable
      :columns="columns"
      :data-source="backups"
      :loading="loading"
      :pagination="false"
      row-key="key"
      size="small"
      :scroll="{ x: 600 }"
    >
      <template #bodyCell="{ column, record: backup }">
        <template v-if="column.key === 'created_at'">
          {{ formatDateTime(backup.created_at) }}
        </template>
        <template v-else-if="column.key === 'key'">
          <span :title="backup.key">{{ fileName(backup.key) }}</span>
        </template>
        <template v-else-if="column.key === 'size'">
          {{ formatSize(backup.size) }}
        </template>
        <template v-else-if="column.key === 'actions'">
          <AButton
            v-if="canRestore && backup.key_file"
            type="link"
            size="small"
            @click="openRestore(backup)"
          >
            {{ $gettext('Restore') }}
          </AButton>
          <APopconfirm
            :title="$gettext('Delete this backup from the storage?')"
            @confirm="handleDelete(backup)"
          >
            <AButton
              type="link"
              size="small"
              danger
              :loading="deleting[backup.key]"
            >
              {{ $gettext('Delete') }}
            </AButton>
          </APopconfirm>
        </template>
      </template>
    </ATable>

    <AModal
      :open="!!restoreTarget"
      :title="$gettext('Restore Backup')"
      :confirm-loading="restoring"
      :ok-text="$gettext('Restore')"
      :ok-button-props="{ danger: true }"
      @ok="handleRestore"
      @cancel="restoreTarget = undefined"
    >
      <AAlert
        class="mb-4"
        type="warning"
        show-icon
        :message="$gettext('Restoring replaces the current configuration. Nginx and Nginx UI restart afterwards.')"
      />
      <div class="mb-2">
        {{ restoreTarget ? fileName(restoreTarget.key) : '' }}
      </div>
      <AFlex vertical :gap="8">
        <ACheckbox v-model:checked="restoreOptions.verifyHash" :disabled="true">
          {{ $gettext('Verify Backup File Integrity (required)') }}
        </ACheckbox>
        <ACheckbox v-model:checked="restoreOptions.restoreNginx">
          {{ $gettext('Restore Nginx Configuration') }}
        </ACheckbox>
        <ACheckbox v-model:checked="restoreOptions.restoreNginxUI">
          {{ $gettext('Restore Nginx UI Configuration') }}
        </ACheckbox>
      </AFlex>
    </AModal>
  </AModal>
</template>
