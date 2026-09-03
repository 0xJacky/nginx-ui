<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { AnyObject } from 'antdv-next/dist/_util/type'
import type { DNSRecord, DNSRecordLine } from '@/api/dns'
import { computed, ref, watch } from 'vue'

interface DNSRecordGroup {
  key: string
  type: string
  content: string
  priority?: number
  weight?: number
  records: DNSRecord[]
  names: string[]
  ttlValues: number[]
  lineValues: Array<string | undefined>
  proxiedValues: boolean[]
  commentValues: string[]
}

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

const pageSizeOptions = ['20', '50', '100', '200']
const currentPage = ref(1)
const pageSize = ref(50)
const expandedRowKeys = ref<string[]>([])

const baseColumns: TableColumnsType = [{
  title: $gettext('Name'),
  key: 'names',
  width: 240,
}, {
  title: $gettext('Type'),
  key: 'type',
  width: 100,
}, {
  title: $gettext('Value'),
  key: 'content',
  width: 280,
}, {
  title: $gettext('TTL'),
  key: 'ttl',
  width: 100,
}, {
  title: $gettext('Priority'),
  key: 'priority',
  width: 100,
}, {
  title: $gettext('Weight'),
  key: 'weight',
  width: 100,
}, {
  title: $gettext('Actions'),
  key: 'actions',
  width: 180,
  fixed: 'right',
}]

const commentColumn = {
  title: $gettext('Comment'),
  key: 'comment',
  width: 200,
  ellipsis: true,
}

const lineColumn = {
  title: $gettext('Resolution Line'),
  key: 'line',
  width: 180,
}

const lineLabelByCode = computed(() => new Map(
  (props.lineOptions ?? []).map(line => [line.code, line.display_name || line.name || line.code]),
))

const columns = computed<TableColumnsType>(() => {
  const list = baseColumns.slice()
  if (props.showLine) {
    list.splice(3, 0, lineColumn)
  }
  if (props.showComment) {
    list.splice(list.length - 1, 0, commentColumn)
  }
  if (props.showProxied) {
    list.splice(list.length - 1, 0, {
      title: $gettext('Proxied'),
      key: 'proxied',
      width: 120,
    })
  }
  return list
})

function getRecordGroupKey(record: DNSRecord) {
  return JSON.stringify({
    type: record.type.trim().toUpperCase(),
    content: record.content,
    priority: record.priority ?? null,
    weight: record.weight ?? null,
  })
}

function uniqueValues<T>(values: T[]) {
  return [...new Set(values)]
}

const recordGroups = computed<DNSRecordGroup[]>(() => {
  const groups = new Map<string, Omit<DNSRecordGroup, 'names' | 'ttlValues' | 'lineValues' | 'proxiedValues' | 'commentValues'>>()

  for (const record of props.records) {
    const key = getRecordGroupKey(record)
    const group = groups.get(key) ?? {
      key,
      type: record.type.trim().toUpperCase(),
      content: record.content,
      priority: record.priority,
      weight: record.weight,
      records: [],
    }
    group.records.push(record)
    groups.set(key, group)
  }

  return [...groups.values()].map(group => {
    const records = group.records.slice().sort((left, right) => left.name.localeCompare(right.name))
    return {
      ...group,
      records,
      names: uniqueValues(records.map(record => record.name)),
      ttlValues: uniqueValues(records.map(record => record.ttl)),
      lineValues: uniqueValues(records.map(record => record.line)),
      proxiedValues: uniqueValues(records.map(record => Boolean(record.proxied))),
      commentValues: uniqueValues(records.map(record => record.comment ?? '')),
    }
  })
})

const tablePagination = computed(() => ({
  current: currentPage.value,
  pageSize: pageSize.value,
  total: recordGroups.value.length,
  showSizeChanger: true,
  pageSizeOptions,
}))

watch(recordGroups, groups => {
  const validGroupKeys = new Set(groups.map(group => group.key))
  expandedRowKeys.value = expandedRowKeys.value.filter(key => validGroupKeys.has(key))

  const lastPage = Math.max(1, Math.ceil(groups.length / pageSize.value))
  if (currentPage.value > lastPage)
    currentPage.value = lastPage
})

function handleTableChange(pagination: { current?: number, pageSize?: number }) {
  if (pagination.pageSize && pagination.pageSize !== pageSize.value) {
    pageSize.value = pagination.pageSize
    currentPage.value = 1
    return
  }
  currentPage.value = pagination.current ?? 1
}

function handleExpand(expanded: boolean, group: DNSRecordGroup) {
  if (expanded) {
    expandedRowKeys.value = [...expandedRowKeys.value, group.key]
    return
  }
  expandedRowKeys.value = expandedRowKeys.value.filter(key => key !== group.key)
}

function toggleGroup(group: DNSRecordGroup) {
  handleExpand(!expandedRowKeys.value.includes(group.key), group)
}

function isGroupExpandable(group: DNSRecordGroup) {
  return group.records.length > 1
}

function handleEdit(record: DNSRecord) {
  emit('edit', record)
}

function handleDelete(record: DNSRecord) {
  emit('delete', record)
}

function formatLine(line?: string) {
  if (!line)
    return '-'
  return lineLabelByCode.value.get(line) || line
}

function formatOptionalNumber(value?: number) {
  return value ?? '-'
}

function getVisibleNames(group: DNSRecordGroup) {
  return group.names.slice(0, 5)
}

function getHiddenNames(group: DNSRecordGroup) {
  return group.names.slice(5)
}

function resetPagination() {
  currentPage.value = 1
}

defineExpose({ resetPagination })
</script>

<template>
  <ATable
    class="dns-record-table"
    row-key="key"
    :columns
    :data-source="recordGroups"
    :loading
    :scroll="{ x: 'max-content' }"
    :pagination="tablePagination"
    :expanded-row-keys="expandedRowKeys"
    :row-expandable="isGroupExpandable"
    @change="handleTableChange"
    @expand="(expanded: boolean, record: AnyObject) => handleExpand(expanded, record as DNSRecordGroup)"
  >
    <template #bodyCell="{ column, record }">
      <template v-if="column.key === 'names'">
        <ASpace wrap size="small">
          <ATag v-for="name in getVisibleNames(record as DNSRecordGroup)" :key="name">
            {{ name }}
          </ATag>
          <ATooltip
            v-if="getHiddenNames(record as DNSRecordGroup).length > 0"
            :title="getHiddenNames(record as DNSRecordGroup).join(', ')"
          >
            <ATag>
              {{ $gettext('%{count} more', { count: String(getHiddenNames(record as DNSRecordGroup).length) }) }}
            </ATag>
          </ATooltip>
        </ASpace>
      </template>
      <template v-else-if="column.key === 'type'">
        <ATag>{{ (record as DNSRecordGroup).type }}</ATag>
      </template>
      <template v-else-if="column.key === 'content'">
        <span class="record-value">{{ (record as DNSRecordGroup).content }}</span>
      </template>
      <template v-else-if="column.key === 'line'">
        <ATooltip
          v-if="(record as DNSRecordGroup).lineValues.length > 1"
          :title="(record as DNSRecordGroup).lineValues.map(formatLine).join(', ')"
        >
          <ATag color="orange">
            {{ $gettext('Mixed') }}
          </ATag>
        </ATooltip>
        <ATag v-else>
          {{ formatLine((record as DNSRecordGroup).lineValues[0]) }}
        </ATag>
      </template>
      <template v-else-if="column.key === 'ttl'">
        <ATooltip
          v-if="(record as DNSRecordGroup).ttlValues.length > 1"
          :title="(record as DNSRecordGroup).ttlValues.join(', ')"
        >
          <ATag color="orange">
            {{ $gettext('Mixed') }}
          </ATag>
        </ATooltip>
        <template v-else>
          {{ (record as DNSRecordGroup).ttlValues[0] }}
        </template>
      </template>
      <template v-else-if="column.key === 'priority'">
        {{ formatOptionalNumber((record as DNSRecordGroup).priority) }}
      </template>
      <template v-else-if="column.key === 'weight'">
        {{ formatOptionalNumber((record as DNSRecordGroup).weight) }}
      </template>
      <template v-else-if="column.key === 'comment'">
        <ATooltip
          v-if="(record as DNSRecordGroup).commentValues.length > 1"
          :title="(record as DNSRecordGroup).commentValues.filter(Boolean).join(', ')"
        >
          <ATag color="orange">
            {{ $gettext('Mixed') }}
          </ATag>
        </ATooltip>
        <template v-else>
          {{ (record as DNSRecordGroup).commentValues[0] || '-' }}
        </template>
      </template>
      <template v-else-if="column.key === 'proxied'">
        <ATag v-if="(record as DNSRecordGroup).proxiedValues.length > 1" color="orange">
          {{ $gettext('Mixed') }}
        </ATag>
        <ATag v-else :color="(record as DNSRecordGroup).proxiedValues[0] ? 'green' : 'default'">
          {{ (record as DNSRecordGroup).proxiedValues[0] ? $gettext('Proxied') : $gettext('DNS Only') }}
        </ATag>
      </template>
      <template v-else-if="column.key === 'actions'">
        <AButton
          v-if="(record as DNSRecordGroup).records.length > 1"
          type="link"
          size="small"
          @click="toggleGroup(record as DNSRecordGroup)"
        >
          {{ $gettext('Manage records') }}
        </AButton>
        <ASpace v-else>
          <AButton type="link" size="small" @click="handleEdit((record as DNSRecordGroup).records[0])">
            {{ $gettext('Edit') }}
          </AButton>
          <APopconfirm
            :title="$gettext('Are you sure to delete this record?')"
            @confirm="handleDelete((record as DNSRecordGroup).records[0])"
          >
            <AButton type="link" danger size="small">
              {{ $gettext('Delete') }}
            </AButton>
          </APopconfirm>
        </ASpace>
      </template>
    </template>

    <template #expandedRowRender="{ record }">
      <div class="record-group-members-wrap">
        <table class="record-group-members">
          <thead>
            <tr>
              <th>{{ $gettext('Name') }}</th>
              <th v-if="showLine">
                {{ $gettext('Resolution Line') }}
              </th>
              <th>{{ $gettext('TTL') }}</th>
              <th v-if="showComment">
                {{ $gettext('Comment') }}
              </th>
              <th v-if="showProxied">
                {{ $gettext('Proxied') }}
              </th>
              <th>{{ $gettext('Actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="member in (record as DNSRecordGroup).records" :key="member.id">
              <th>{{ member.name }}</th>
              <td v-if="showLine">
                <ATag>{{ formatLine(member.line) }}</ATag>
              </td>
              <td>{{ member.ttl }}</td>
              <td v-if="showComment">
                {{ member.comment || '-' }}
              </td>
              <td v-if="showProxied">
                <ATag :color="member.proxied ? 'green' : 'default'">
                  {{ member.proxied ? $gettext('Proxied') : $gettext('DNS Only') }}
                </ATag>
              </td>
              <td>
                <ASpace>
                  <AButton type="link" size="small" @click="handleEdit(member)">
                    {{ $gettext('Edit') }}
                  </AButton>
                  <APopconfirm
                    :title="$gettext('Are you sure to delete this record?')"
                    @confirm="handleDelete(member)"
                  >
                    <AButton type="link" danger size="small">
                      {{ $gettext('Delete') }}
                    </AButton>
                  </APopconfirm>
                </ASpace>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
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

.record-value {
  display: inline-block;
  max-width: 44ch;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.record-group-members-wrap {
  max-width: 100%;
  overflow-x: auto;
}

.record-group-members {
  width: 100%;
  min-width: 560px;
  border-collapse: collapse;
  background: var(--ant-color-bg-container);

  th,
  td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--ant-color-border-secondary);
    text-align: left;
    vertical-align: middle;
  }

  thead th {
    color: var(--ant-color-text-secondary);
    font-size: 12px;
    font-weight: 500;
  }

  tbody th {
    color: var(--ant-color-text);
    font-weight: 600;
  }
}
</style>
