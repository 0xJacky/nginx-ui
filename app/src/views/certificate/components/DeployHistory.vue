<script setup lang="ts">
import type { TableColumnType } from 'antdv-next'
import type { CertDeployment, CertDeployTarget } from '@/api/cert_deploy'
import { listDeployments } from '@/api/cert_deploy'
import { formatDateTime } from '@/lib/helper'
import { certificateLabel, loadCertificateOptions } from '../deployTargets'

// The latest pushes of one deploy target.
const props = defineProps<{
  target?: CertDeployTarget
}>()

const open = defineModel<boolean>('open', { default: false })

const loading = ref(false)
const deployments = ref<CertDeployment[]>([])

watch(open, async value => {
  if (!value || !props.target?.id)
    return
  loading.value = true
  try {
    loadCertificateOptions()
    const res = await listDeployments(props.target.id)
    deployments.value = res.data ?? []
  }
  catch {
    deployments.value = []
  }
  finally {
    loading.value = false
  }
})

const columns: TableColumnType<CertDeployment>[] = [
  { title: () => $gettext('Time'), dataIndex: 'created_at', key: 'created_at', width: 170 },
  { title: () => $gettext('Certificate'), dataIndex: 'cert_id', key: 'cert_id', width: 160 },
  { title: () => $gettext('Status'), dataIndex: 'status', key: 'status', width: 90 },
  { title: () => $gettext('Attempts'), dataIndex: 'attempts', key: 'attempts', width: 90 },
  { title: () => $gettext('Message'), dataIndex: 'message', key: 'message' },
]
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Deploy History of %{name}', { name: props.target?.name ?? '' })"
    :footer="null"
    width="860px"
    destroy-on-hidden
  >
    <ATable
      :columns="columns"
      :data-source="deployments"
      :loading="loading"
      :pagination="false"
      row-key="id"
      size="small"
      :scroll="{ x: 700 }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'created_at'">
          {{ formatDateTime(record.created_at) }}
        </template>
        <template v-else-if="column.key === 'cert_id'">
          {{ certificateLabel(record.cert_id) }}
        </template>
        <template v-else-if="column.key === 'status'">
          <ATag :color="record.status === 'ok' ? 'green' : 'red'">
            {{ record.status === 'ok' ? $gettext('Success') : $gettext('Failed') }}
          </ATag>
        </template>
        <template v-else-if="column.key === 'message'">
          <span class="break-all">{{ record.message }}</span>
        </template>
      </template>
    </ATable>
  </AModal>
</template>
