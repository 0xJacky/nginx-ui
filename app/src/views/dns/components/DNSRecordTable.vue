<script setup lang="ts">
import type { DNSRecord, DNSRecordLine } from '@/api/dns'
import { computed } from 'vue'

const props = defineProps<{
  records: DNSRecord[]
  loading?: boolean
  showProxied?: boolean
  showComment?: boolean
  showLine?: boolean
  lineOptions?: DNSRecordLine[]
}>()

const emit = defineEmits<{
  (event: 'edit', record: DNSRecord): void
  (event: 'delete', record: DNSRecord): void
}>()

const baseColumns = [{
  title: $gettext('Name'),
  dataIndex: 'name',
  width: 160,
}, {
  title: $gettext('Type'),
  dataIndex: 'type',
  width: 100,
}, {
  title: $gettext('Value'),
  dataIndex: 'content',
  width: 240,
}, {
  title: $gettext('TTL'),
  dataIndex: 'ttl',
  width: 100,
}, {
  title: $gettext('Priority'),
  dataIndex: 'priority',
  width: 100,
}, {
  title: $gettext('Weight'),
  dataIndex: 'weight',
  width: 100,
}, {
  title: $gettext('Actions'),
  dataIndex: 'actions',
  width: 180,
  fixed: 'right',
}]

const commentColumn = {
  title: $gettext('Comment'),
  dataIndex: 'comment',
  width: 200,
  ellipsis: true,
}

const lineColumn = {
  title: $gettext('Resolution Line'),
  dataIndex: 'line',
  width: 180,
}

const lineLabelByCode = computed(() => new Map(
  (props.lineOptions ?? []).map(line => [line.code, line.display_name || line.name || line.code]),
))

const columns = computed(() => {
  const list = baseColumns.slice()
  if (props.showLine) {
    list.splice(3, 0, lineColumn)
  }
  // Insert comment column before actions column if showComment is enabled
  if (props.showComment) {
    list.splice(list.length - 1, 0, commentColumn)
  }
  // Insert proxied column before actions column if showProxied is enabled
  if (props.showProxied) {
    list.splice(list.length - 1, 0, {
      title: $gettext('Proxied'),
      dataIndex: 'proxied',
      width: 120,
    })
  }
  return list
})

function handleEdit(record: DNSRecord) {
  emit('edit', record)
}

function formatLine(line?: string) {
  if (!line)
    return '-'
  return lineLabelByCode.value.get(line) || line
}

function handleDelete(record: DNSRecord) {
  emit('delete', record)
}
</script>

<template>
  <ATable
    class="dns-record-table"
    row-key="id"
    :columns="(columns as any)"
    :data-source="records"
    :loading
    :scroll="{ x: 'max-content' }"
    :pagination="false"
  >
    <template #bodyCell="{ column, record }">
      <template v-if="column.dataIndex === 'proxied'">
        <ATag :color="(record as DNSRecord).proxied ? 'green' : 'default'">
          {{ (record as DNSRecord).proxied ? $gettext('Proxied') : $gettext('DNS Only') }}
        </ATag>
      </template>
      <template v-else-if="column.dataIndex === 'line'">
        <ATag>{{ formatLine((record as DNSRecord).line) }}</ATag>
      </template>
      <template v-else-if="column.dataIndex === 'content'">
        <span class="record-value">{{ (record as DNSRecord).content }}</span>
      </template>
      <template v-else-if="column.dataIndex === 'actions'">
        <ASpace>
          <AButton type="link" size="small" @click="handleEdit(record as DNSRecord)">
            {{ $gettext('Edit') }}
          </AButton>
          <APopconfirm
            :title="$gettext('Are you sure to delete this record?')"
            @confirm="handleDelete(record as DNSRecord)"
          >
            <AButton type="link" danger size="small">
              {{ $gettext('Delete') }}
            </AButton>
          </APopconfirm>
        </ASpace>
      </template>
    </template>
  </ATable>
</template>

<style scoped lang="less">
.dns-record-table {
  :deep(.ant-table-cell) {
    white-space: normal;
    word-break: break-word;
  }
}

// Record sets that hold several values render one value per line.
.record-value {
  white-space: pre-wrap;
}
</style>
