<script setup lang="tsx">
import type { TableColumnsType } from 'antdv-next'
import type { Cert, CertListCounts, CertListFilter, DiscoveredCertificatePair } from '@/api/cert'
import { CloudUploadOutlined, InfoCircleOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@antdv-next/icons'
import { useMediaQuery, watchDebounced } from '@vueuse/core'
import { Tag, Tooltip } from 'antdv-next'
import dayjs from 'dayjs'
import cert from '@/api/cert'
import { useGlobalStore } from '@/pinia'
import { certRenewalMethodLabel, certStateLabel, certStateTone, isAcmeCert, splitVisible } from '../certState'
import WildcardCertificate from '../components/DNSIssueCertificate.vue'
import RetryCert from '../components/RetryCert.vue'

const refWildcard = ref()
const router = useRouter()

// ---- List ------------------------------------------------------------------

const rows = ref<Cert[]>([])
const loading = ref(false)
const keyword = ref('')
const filter = ref<CertListFilter>('all')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const counts = ref<CertListCounts>({ all: 0, expiring: 0, failed: 0, expired: 0 })

let listSeq = 0

async function loadList() {
  const seq = ++listSeq
  loading.value = true
  try {
    const r = await cert.get_overview_list({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value.trim() || undefined,
      state: filter.value === 'all' ? undefined : filter.value,
      with_counts: true,
    })
    if (seq !== listSeq)
      return
    rows.value = r.data ?? []
    total.value = r.pagination?.total ?? rows.value.length
    if (r.counts)
      counts.value = r.counts
  }
  finally {
    if (seq === listSeq)
      loading.value = false
  }
}

function refresh() {
  void loadList()
}

watch(filter, () => {
  page.value = 1
  void loadList()
})

watchDebounced(keyword, () => {
  page.value = 1
  void loadList()
}, { debounce: 300 })

onMounted(loadList)

const filterOptions = computed(() => [
  { value: 'all', label: $gettext('All') },
  { value: 'expiring', label: $gettext('Expiring within 30 days') },
  { value: 'failed', label: $gettext('Renewal failed') },
  { value: 'expired', label: $gettext('Expired') },
])

const pagination = computed(() => ({
  current: page.value,
  pageSize: pageSize.value,
  total: total.value,
  showSizeChanger: true,
  size: 'small' as const,
  onChange: (next: number, size: number) => {
    page.value = next
    pageSize.value = size
    void loadList()
  },
}))

function toneColor(record: Cert) {
  return `var(--ant-color-${{
    success: 'success',
    warning: 'warning',
    error: 'error',
    processing: 'primary',
    default: 'text-quaternary',
  }[certStateTone(record.state)]})`
}

function editCert(record: Cert) {
  router.push(`/certificates/${record.id}`)
}

function renderState(record: Cert) {
  const label = (
    <span class="cert-state" style={{ color: toneColor(record) }}>
      <span class="cert-state-dot" />
      {certStateLabel(record)}
      {record.state === 'failed' && record.last_renewal_error && <InfoCircleOutlined />}
    </span>
  )
  if (record.state === 'failed' && record.last_renewal_error)
    return <Tooltip title={record.last_renewal_error}>{label}</Tooltip>
  return label
}

function expiryDate(record: Cert) {
  return record.certificate_info?.not_after
    ? dayjs(record.certificate_info.not_after).format('YYYY-MM-DD')
    : '-'
}

// A function so the title follows a language switch.
function actionsColumn(): TableColumnsType<Cert>[number] {
  return {
    title: $gettext('Actions'),
    key: 'actions',
    fixed: 'right',
    align: 'right',
    width: 130,
  }
}

// A certificate issued for a node names that node, since it does not serve
// any configuration of this instance.
function renderName(record: Cert) {
  const link = <a onClick={() => editCert(record)}>{record.name || record.domains?.[0] || '-'}</a>
  if (!record.delegated_node_id)
    return link
  const node = record.delegated_node_name || `#${record.delegated_node_id}`
  return (
    <div class="cert-name">
      {link}
      <Tooltip title={$gettext('Issued by this instance for %{node} and renewed here', { node })}>
        <Tag class="m-0" color="geekblue" variant="filled">{$gettext('For %{node}', { node })}</Tag>
      </Tooltip>
    </div>
  )
}

const columns = computed<TableColumnsType<Cert>>(() => [
  {
    title: $gettext('Name'),
    dataIndex: 'name',
    ellipsis: true,
    width: 200,
    render: (_: unknown, record: Cert) => renderName(record),
  },
  {
    title: $gettext('Domains'),
    dataIndex: 'domains',
    width: 260,
    render: (_: unknown, record: Cert) => {
      const { visible, hidden } = splitVisible(record.domains ?? [], 2)
      if (!visible.length)
        return '-'
      return (
        <div class="cert-domains">
          {visible.map(domain => <Tag class="m-0 font-mono">{domain}</Tag>)}
          {hidden > 0 && (
            <Tooltip title={(record.domains ?? []).slice(2).join(', ')}>
              <Tag class="m-0">{`+${hidden}`}</Tag>
            </Tooltip>
          )}
        </div>
      )
    },
  },
  {
    title: $gettext('Status'),
    dataIndex: 'state',
    width: 190,
    render: (_: unknown, record: Cert) => renderState(record),
  },
  {
    title: $gettext('Renewal method'),
    dataIndex: 'renewal_method',
    width: 190,
    ellipsis: true,
    render: (_: unknown, record: Cert) => certRenewalMethodLabel(record.renewal_method, record.dns_provider),
  },
  {
    title: $gettext('Expiry date'),
    dataIndex: ['certificate_info', 'not_after'],
    width: 120,
    render: (_: unknown, record: Cert) => expiryDate(record),
  },
  actionsColumn(),
])

// Phones get one column with the status and expiry under the name, so the
// status stays visible without scrolling sideways.
const isNarrow = useMediaQuery('(max-width: 575px)')

const narrowColumns = computed<TableColumnsType<Cert>>(() => [
  {
    title: $gettext('Name'),
    dataIndex: 'name',
    render: (_: unknown, record: Cert) => (
      <div class="min-w-0">
        {renderName(record)}
        <div class="cert-narrow-meta">
          {renderState(record)}
          {record.certificate_info?.not_after && <span class="cert-narrow-expiry">{expiryDate(record)}</span>}
        </div>
      </div>
    ),
  },
  { ...actionsColumn(), fixed: undefined, width: 110 },
])

const tableColumns = computed(() => isNarrow.value ? narrowColumns.value : columns.value)

const globalStore = useGlobalStore()

const { processingStatus } = storeToRefs(globalStore)

const discoveryVisible = ref(false)
const discoveryLoading = ref(false)
const discoveryImporting = ref(false)
const discoveryCandidates = ref<DiscoveredCertificatePair[]>([])
const selectedDiscoveryKeys = ref<string[]>([])
const { message } = App.useApp()

function discoveryRowKey(record: DiscoveredCertificatePair) {
  return record.fingerprint || `${record.ssl_certificate_path}|${record.ssl_certificate_key_path}`
}

const discoveryRowSelection = computed(() => ({
  selectedRowKeys: selectedDiscoveryKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedDiscoveryKeys.value = keys.map(String)
  },
}))

const discoveryColumns = computed(() => [
  {
    title: $gettext('Name'),
    dataIndex: 'name',
  },
  {
    title: $gettext('Type'),
    render: () => (
      <Tag variant="filled" color="purple">
        {$gettext('General Certificate')}
      </Tag>
    ),
  },
  {
    title: $gettext('SSL Certificate Path'),
    dataIndex: 'ssl_certificate_path',
    ellipsis: true,
  },
  {
    title: $gettext('SSL Certificate Key Path'),
    dataIndex: 'ssl_certificate_key_path',
    ellipsis: true,
  },
  {
    title: $gettext('Not After'),
    render: (_value: unknown, record: DiscoveredCertificatePair) => {
      return record.certificate_info?.not_after ?? '-'
    },
  },
])

async function scanDiscoveredCertificates() {
  discoveryLoading.value = true
  try {
    const result = await cert.discover_new({
      new_only: true,
    })
    discoveryCandidates.value = result.candidates ?? []
    selectedDiscoveryKeys.value = discoveryCandidates.value.map(discoveryRowKey)
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to scan certificates'))
  }
  finally {
    discoveryLoading.value = false
  }
}

async function openDiscovery() {
  discoveryVisible.value = true
  await scanDiscoveredCertificates()
}

async function importSelectedDiscoveredCerts() {
  const selected = new Set(selectedDiscoveryKeys.value)
  const candidates = discoveryCandidates.value.filter(item => selected.has(discoveryRowKey(item)))
  if (!candidates.length) {
    message.warning($gettext('Please select at least one certificate'))
    return
  }

  discoveryImporting.value = true
  try {
    for (const item of candidates) {
      await cert.import_existing({
        name: item.name,
        ssl_certificate_path: item.ssl_certificate_path,
        ssl_certificate_key_path: item.ssl_certificate_key_path,
        key_type: item.key_type,
      })
    }
    message.success($gettext('Import successfully'))
    discoveryVisible.value = false
    refresh()
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to import certificate'))
  }
  finally {
    discoveryImporting.value = false
  }
}
</script>

<template>
  <ACard :title="$gettext('Certificates')">
    <template #extra>
      <AFlex wrap gap="small">
        <AButton :aria-label="$gettext('Discover')" @click="openDiscovery">
          <template #icon>
            <SearchOutlined />
          </template>
          <span class="certificate-action-label">{{ $gettext('Discover') }}</span>
        </AButton>

        <AButton :aria-label="$gettext('Import')" @click="$router.push('/certificates/import')">
          <template #icon>
            <CloudUploadOutlined />
          </template>
          <span class="certificate-action-label">{{ $gettext('Import') }}</span>
        </AButton>

        <AButton
          type="primary"
          :aria-label="$gettext('Issue certificate')"
          :disabled="processingStatus.auto_cert_processing"
          @click="() => refWildcard.open()"
        >
          <template #icon>
            <PlusOutlined />
          </template>
          <span class="certificate-action-label">{{ $gettext('Issue certificate') }}</span>
        </AButton>
      </AFlex>
    </template>

    <div class="cert-toolbar">
      <AInput
        v-model:value="keyword"
        class="cert-search"
        :placeholder="$gettext('Search by name or domain')"
        allow-clear
      >
        <template #prefix>
          <SearchOutlined class="cert-muted" />
        </template>
      </AInput>
      <div class="cert-filter">
        <ASegmented v-model:value="filter" :options="filterOptions">
          <template #labelRender="option">
            <span>
              {{ option.label }}
              <span class="cert-filter-count">{{ counts[option.value as CertListFilter] }}</span>
            </span>
          </template>
        </ASegmented>
      </div>
      <AButton class="cert-refresh" :aria-label="$gettext('Reload')" :loading="loading" @click="refresh">
        <template #icon>
          <ReloadOutlined />
        </template>
      </AButton>
    </div>

    <ATable
      :columns="tableColumns"
      :data-source="rows"
      :loading="loading"
      :pagination="pagination"
      :scroll="isNarrow ? undefined : { x: 1000 }"
      row-key="id"
      size="middle"
    >
      <template #bodyCell="{ column, record }">
        <AFlex v-if="column.key === 'actions'" justify="flex-end" gap="small">
          <RetryCert
            v-if="(record as Cert).renewal_failed && isAcmeCert(record as Cert)"
            :cert="record as Cert"
            @retried="refresh"
          />
          <AButton type="link" size="small" @click="editCert(record as Cert)">
            {{ $gettext('Edit') }}
          </AButton>
        </AFlex>
      </template>
    </ATable>
    <WildcardCertificate
      ref="refWildcard"
      @issued="refresh"
    />
    <AModal
      v-model:open="discoveryVisible"
      :title="$gettext('Discover Certificates')"
      :ok-text="$gettext('Import selected')"
      :confirm-loading="discoveryImporting"
      :ok-button-props="{ disabled: selectedDiscoveryKeys.length === 0 }"
      width="900px"
      @ok="importSelectedDiscoveredCerts"
    >
      <div class="mb-4 flex justify-end">
        <AButton
          :loading="discoveryLoading"
          @click="scanDiscoveredCertificates"
        >
          {{ $gettext('Scan') }}
        </AButton>
      </div>
      <ATable
        :columns="discoveryColumns"
        :data-source="discoveryCandidates"
        :loading="discoveryLoading"
        :row-key="discoveryRowKey"
        :row-selection="discoveryRowSelection"
        :pagination="false"
        size="small"
      />
    </AModal>
  </ACard>
</template>

<style lang="less" scoped>
.cert-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.cert-search {
  width: 260px;
  max-width: 100%;
}

.cert-filter {
  max-width: 100%;
  overflow-x: auto;
}

.cert-filter-count {
  margin-inline-start: 4px;
  color: var(--ant-color-text-tertiary);
}

.cert-refresh {
  margin-inline-start: auto;
}

.cert-muted {
  color: var(--ant-color-text-quaternary);
}

:deep(.cert-name) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 6px;
  min-width: 0;
}

:deep(.cert-domains) {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

:deep(.cert-state) {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

:deep(.cert-state-dot) {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

@media (max-width: 600px) {
  .certificate-action-label {
    display: none;
  }

  .cert-search {
    width: 100%;
  }
}

:deep(.cert-narrow-meta) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 12px;
  margin-top: 2px;
  font-size: 12px;
}

:deep(.cert-narrow-expiry) {
  color: var(--ant-color-text-tertiary);
}
</style>
