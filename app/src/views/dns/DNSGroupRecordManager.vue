<script setup lang="ts">
import type { TableColumnsType } from 'ant-design-vue'
import type { DNSDomain, DNSRecord, DNSRecordLine, RecordPayload } from '@/api/dns'
import type { DNSDomainGroup } from '@/pinia/moudule/dnsGroup'
import { ArrowLeftOutlined, PlusOutlined, ReloadOutlined, SyncOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { dnsApi } from '@/api/dns'
import FooterToolBar from '@/components/FooterToolbar'
import { useDnsGroupStore } from '@/pinia'
import DNSRecordForm from '@/views/dns/components/DNSRecordForm.vue'
import {
  getCommonRecordPayload,
  getErrorMessage,
  getRecordKey,
  hasSameRecordValues,
  listAllDNSDomains,
  listAllDNSRecords,
  mergeRecordPayload,
} from '@/views/dns/group'

type GroupRecordStatus = 'synced' | 'drifted' | 'missing' | 'ambiguous' | 'unknown'
type BatchAction = 'create' | 'update' | 'skip' | 'blocked'
type BatchResultStatus = 'success' | 'failed' | 'skipped' | 'blocked'
type ProviderFieldKey = 'line' | 'proxied' | 'comment'
type RecordGroupScope = 'all' | 'selected'

interface AggregatedGroupRecord {
  key: string
  name: string
  type: string
  members: Record<number, DNSRecord[]>
}

interface BatchTask {
  taskKey: string
  domain: DNSDomain
  action: BatchAction
  payload: RecordPayload
  recordId?: string
  reason?: string
}

interface BatchResult extends BatchTask {
  status: BatchResultStatus
  detail?: string
}

interface ProviderFieldModel {
  line?: string
  proxied?: boolean
  comment?: string
}

interface RecordSource {
  key: string
  domain: DNSDomain
  record: DNSRecord
}

interface RecordGroupValues {
  content: string
  ttl: number
  priority?: number
  weight?: number
}

interface AutomaticRecordGroup {
  key: string
  type: string
  recordNames: string[]
  values: RecordGroupValues
}

interface RecordGroupStats {
  targetCount: number
  syncedCount: number
  driftedCount: number
  missingCount: number
  ambiguousCount: number
  unknownCount: number
}

interface RecordGroupRow {
  key: string
  recordGroup: AutomaticRecordGroup
  stats: RecordGroupStats
}

const route = useRoute()
const router = useRouter()
const groupStore = useDnsGroupStore()

const groupId = computed(() => String(route.params.id ?? ''))
const group = computed<DNSDomainGroup | undefined>(() => groupStore.getGroup(groupId.value))
const domains = ref<DNSDomain[]>([])
const recordsByDomainId = ref<Record<number, DNSRecord[]>>({})
const loadErrors = ref<Record<number, string>>({})
const missingDomainIds = ref<number[]>([])
const isLoading = ref(false)
const searchKeyword = ref('')

const isOperationDrawerOpen = ref(false)
const operationMode = ref<'create' | 'recordGroup'>('create')
const operationRecordGroupKey = ref<string | null>(null)
const recordGroupScope = ref<RecordGroupScope>('all')
const selectedRecordNames = ref<string[]>([])
const selectedDomainIds = ref<number[]>([])
const operationSourceKey = ref<string | null>(null)
const formModel = ref<RecordPayload>(createEmptyPayload())
const providerFieldModels = ref<Record<number, ProviderFieldModel>>({})
const touchedProviderFields = ref<Record<number, ProviderFieldKey[]>>({})
const recordLinesByDomainId = ref<Record<number, DNSRecordLine[]>>({})
const recordLineLoadingByDomainId = ref<Record<number, boolean>>({})
const recordLineErrors = ref<Record<number, string>>({})

const isPreviewOpen = ref(false)
const previewTasks = ref<BatchTask[]>([])
const isApplying = ref(false)
const isResultOpen = ref(false)
const batchResults = ref<BatchResult[]>([])

const recordGroupColumns: TableColumnsType = [{
  title: $gettext('Type'),
  key: 'type',
  width: 90,
}, {
  title: $gettext('Shared value'),
  key: 'value',
  width: 300,
}, {
  title: $gettext('Names'),
  key: 'names',
  width: 280,
}, {
  title: $gettext('Coverage'),
  key: 'coverage',
  width: 130,
}, {
  title: $gettext('Status'),
  key: 'status',
  width: 180,
}, {
  title: $gettext('Actions'),
  key: 'actions',
  width: 220,
  fixed: 'right',
}]

const previewColumns: TableColumnsType = [{
  title: $gettext('Domain'),
  key: 'domain',
  width: 180,
}, {
  title: $gettext('Name'),
  key: 'name',
  width: 180,
}, {
  title: $gettext('Action'),
  key: 'action',
  width: 120,
}, {
  title: $gettext('Details'),
  key: 'details',
}]

const resultColumns: TableColumnsType = [{
  title: $gettext('Domain'),
  key: 'domain',
  width: 180,
}, {
  title: $gettext('Name'),
  key: 'name',
  width: 180,
}, {
  title: $gettext('Action'),
  key: 'action',
  width: 110,
}, {
  title: $gettext('Result'),
  key: 'status',
  width: 110,
}, {
  title: $gettext('Details'),
  key: 'details',
}]

const domainOptions = computed(() => domains.value.map(domain => ({
  value: domain.id,
  label: `${domain.domain} · ${domain.dns_credential?.provider ?? $gettext('Unknown provider')}`,
})))

const aggregatedRecords = computed<AggregatedGroupRecord[]>(() => {
  const rows = new Map<string, AggregatedGroupRecord>()
  for (const domain of domains.value) {
    for (const record of recordsByDomainId.value[domain.id] ?? []) {
      const key = getRecordKey(record)
      const row = rows.get(key) ?? {
        key,
        name: record.name,
        type: record.type.toUpperCase(),
        members: {},
      }
      row.members[domain.id] = [...(row.members[domain.id] ?? []), record]
      rows.set(key, row)
    }
  }

  return [...rows.values()].sort((left, right) => left.name.localeCompare(right.name) || left.type.localeCompare(right.type))
})

const recordSources = computed<RecordSource[]>(() => domains.value.flatMap(domain =>
  (recordsByDomainId.value[domain.id] ?? []).map(record => ({
    key: getRecordSourceKey(domain.id, record),
    domain,
    record,
  })),
))

const recordSourceMap = computed(() => new Map(recordSources.value.map(source => [source.key, source])))

const automaticRecordGroups = computed<AutomaticRecordGroup[]>(() => {
  interface CanonicalNameValue {
    row: AggregatedGroupRecord
    signature: string
    sourceRecord: DNSRecord
    matchingRecords: DNSRecord[]
  }

  const canonicalValues: CanonicalNameValue[] = []
  for (const row of aggregatedRecords.value) {
    const signatures = new Map<string, {
      sourceRecord: DNSRecord
      domainIds: Set<number>
      records: DNSRecord[]
    }>()

    for (const domain of domains.value) {
      for (const record of row.members[domain.id] ?? []) {
        const signature = getSharedValueKey(record)
        const entry = signatures.get(signature) ?? {
          sourceRecord: record,
          domainIds: new Set<number>(),
          records: [],
        }
        entry.domainIds.add(domain.id)
        entry.records.push(record)
        signatures.set(signature, entry)
      }
    }

    let selected: { signature: string, sourceRecord: DNSRecord, domainCount: number, records: DNSRecord[] } | undefined
    for (const [signature, entry] of signatures) {
      if (!selected
        || entry.domainIds.size > selected.domainCount
        || (entry.domainIds.size === selected.domainCount && entry.records.length > selected.records.length)) {
        selected = {
          signature,
          sourceRecord: entry.sourceRecord,
          domainCount: entry.domainIds.size,
          records: entry.records,
        }
      }
    }
    if (selected) {
      canonicalValues.push({
        row,
        signature: selected.signature,
        sourceRecord: selected.sourceRecord,
        matchingRecords: selected.records,
      })
    }
  }

  const clusters = new Map<string, {
    type: string
    names: Map<string, string>
    sourceRecord: DNSRecord
    matchingRecords: DNSRecord[]
  }>()
  for (const candidate of canonicalValues) {
    const cluster = clusters.get(candidate.signature) ?? {
      type: candidate.sourceRecord.type.toUpperCase(),
      names: new Map<string, string>(),
      sourceRecord: candidate.sourceRecord,
      matchingRecords: [],
    }
    cluster.names.set(normalizeRecordName(candidate.row.name), candidate.row.name)
    cluster.matchingRecords.push(...candidate.matchingRecords)
    clusters.set(candidate.signature, cluster)
  }

  return [...clusters.entries()].map(([key, cluster]) => {
    const ttlCounts = new Map<number, number>()
    let ttl = cluster.sourceRecord.ttl
    let ttlCount = 0
    for (const record of cluster.matchingRecords) {
      const count = (ttlCounts.get(record.ttl) ?? 0) + 1
      ttlCounts.set(record.ttl, count)
      if (count > ttlCount) {
        ttl = record.ttl
        ttlCount = count
      }
    }

    return {
      key,
      type: cluster.type,
      recordNames: [...cluster.names.values()].sort((left, right) => left.localeCompare(right)),
      values: {
        content: cluster.sourceRecord.content,
        ttl,
        priority: cluster.sourceRecord.priority,
        weight: cluster.sourceRecord.weight,
      },
    }
  }).sort((left, right) => left.type.localeCompare(right.type)
    || left.values.content.localeCompare(right.values.content)
    || left.recordNames[0].localeCompare(right.recordNames[0]))
})

const recordGroupRows = computed<RecordGroupRow[]>(() => automaticRecordGroups.value.map(recordGroup => ({
  key: recordGroup.key,
  recordGroup,
  stats: getRecordGroupStats(recordGroup),
})))

const filteredRecordGroupRows = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword)
    return recordGroupRows.value
  return recordGroupRows.value.filter(row =>
    row.recordGroup.type.toLowerCase().includes(keyword)
    || row.recordGroup.values.content.toLowerCase().includes(keyword)
    || row.recordGroup.recordNames.some(name => name.toLowerCase().includes(keyword)),
  )
})

const editingRecordGroup = computed(() => operationRecordGroupKey.value
  ? automaticRecordGroups.value.find(recordGroup => recordGroup.key === operationRecordGroupKey.value)
  : undefined)

const operationSourceOptions = computed(() => {
  if (operationMode.value !== 'recordGroup')
    return []
  const selectedNames = new Set(selectedRecordNames.value.map(name => normalizeRecordName(name)))
  return recordSources.value
    .filter(source => selectedDomainIds.value.includes(source.domain.id)
      && source.record.type.toUpperCase() === formModel.value.type.toUpperCase()
      && selectedNames.has(normalizeRecordName(source.record.name)))
    .map(source => ({
      value: source.key,
      label: `${source.domain.domain} · ${source.record.name} · ${source.record.content}`,
    }))
})

const providerFieldDomains = computed(() => domains.value.filter(domain =>
  selectedDomainIds.value.includes(domain.id) && hasProviderSpecificFields(domain),
))

const actionablePreviewCount = computed(() => previewTasks.value.filter(task =>
  task.action === 'create' || task.action === 'update',
).length)

const failedResults = computed(() => batchResults.value.filter(result => result.status === 'failed'))

const operationTargetCount = computed(() => selectedDomainIds.value.length * (
  operationMode.value === 'recordGroup' ? selectedRecordNames.value.length : 1
))

function createEmptyPayload(): RecordPayload {
  return {
    type: 'A',
    name: '@',
    content: '',
    ttl: 600,
  }
}

function normalizeRecordName(name: string) {
  return name.trim().toLowerCase()
}

function getTypedNameKey(type: string, name: string) {
  return `${type.trim().toUpperCase()}\u0000${normalizeRecordName(name)}`
}

function getRecordSourceKey(domainId: number, record: DNSRecord) {
  return `${domainId}\u0000${record.id}`
}

function getTaskKey(domainId: number, payload: Pick<RecordPayload, 'name' | 'type'>) {
  return `${domainId}\u0000${getRecordKey(payload)}`
}

function getSharedValueKey(record: DNSRecord) {
  return JSON.stringify({
    type: record.type.toUpperCase(),
    content: record.content,
    priority: record.priority ?? null,
    weight: record.weight ?? null,
  })
}

function getRecordGroupPayload(recordGroup: AutomaticRecordGroup, name: string): RecordPayload {
  return {
    type: recordGroup.type,
    name,
    ...recordGroup.values,
  }
}

function hasSameRecordGroupValues(record: DNSRecord, recordGroup: AutomaticRecordGroup) {
  return record.type.toUpperCase() === recordGroup.type.toUpperCase()
    && record.content === recordGroup.values.content
    && record.ttl === recordGroup.values.ttl
    && record.priority === recordGroup.values.priority
    && record.weight === recordGroup.values.weight
}

async function runWithConcurrency<T, R>(items: T[], limit: number, worker: (item: T) => Promise<R>) {
  const results: R[] = []
  let nextIndex = 0

  async function runWorker() {
    while (nextIndex < items.length) {
      const currentIndex = nextIndex++
      results[currentIndex] = await worker(items[currentIndex])
    }
  }

  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, runWorker))
  return results
}

async function loadGroupRecords() {
  if (!group.value)
    return

  isLoading.value = true
  loadErrors.value = {}
  recordsByDomainId.value = {}
  try {
    const allDomains = await listAllDNSDomains()
    const allDomainMap = new Map(allDomains.map(domain => [domain.id, domain]))
    domains.value = group.value.domainIds.flatMap(domainId => {
      const domain = allDomainMap.get(domainId)
      return domain ? [domain] : []
    })
    missingDomainIds.value = group.value.domainIds.filter(domainId => !allDomainMap.has(domainId))

    await runWithConcurrency(domains.value, 3, async domain => {
      try {
        recordsByDomainId.value[domain.id] = await listAllDNSRecords(domain.id)
      }
      catch (error) {
        loadErrors.value[domain.id] = getErrorMessage(error)
        recordsByDomainId.value[domain.id] = []
      }
    })
  }
  finally {
    isLoading.value = false
  }
}

function getStatusLabel(status: GroupRecordStatus) {
  const labels: Record<GroupRecordStatus, string> = {
    synced: $gettext('Synced'),
    drifted: $gettext('Different values'),
    missing: $gettext('Missing records'),
    ambiguous: $gettext('Multiple matches'),
    unknown: $gettext('Incomplete'),
  }
  return labels[status]
}

function getStatusColor(status: GroupRecordStatus) {
  const colors: Record<GroupRecordStatus, string> = {
    synced: 'success',
    drifted: 'warning',
    missing: 'processing',
    ambiguous: 'error',
    unknown: 'default',
  }
  return colors[status]
}

function getRecordGroupCellRecords(recordGroup: AutomaticRecordGroup, name: string, domainId: number) {
  const key = getTypedNameKey(recordGroup.type, name)
  return (recordsByDomainId.value[domainId] ?? []).filter(record =>
    getTypedNameKey(record.type, record.name) === key,
  )
}

function getRecordGroupCellStatus(recordGroup: AutomaticRecordGroup, name: string, domain: DNSDomain): GroupRecordStatus {
  if (loadErrors.value[domain.id])
    return 'unknown'
  const records = getRecordGroupCellRecords(recordGroup, name, domain.id)
  if (records.length === 0)
    return 'missing'
  if (records.length > 1)
    return 'ambiguous'
  if (!hasSameRecordGroupValues(records[0], recordGroup))
    return 'drifted'
  return hasRecordGroupProviderDrift(recordGroup, name, domain, records[0]) ? 'drifted' : 'synced'
}

function getProviderIdentity(domain: DNSDomain) {
  return getProviderCode(domain) || domain.dns_credential?.provider?.toLowerCase() || 'unknown'
}

function getProviderSpecificSignature(domain: DNSDomain, record: DNSRecord) {
  if (supportsRecordLines(domain))
    return JSON.stringify({ line: record.line ?? getDefaultRecordLine(domain) })
  if (isCloudflareDomain(domain))
    return JSON.stringify({ proxied: record.proxied ?? false, comment: record.comment ?? '' })
  return undefined
}

function hasRecordGroupProviderDrift(recordGroup: AutomaticRecordGroup, name: string, domain: DNSDomain, record: DNSRecord) {
  const currentSignature = getProviderSpecificSignature(domain, record)
  if (!currentSignature)
    return false

  const signatures = domains.value.flatMap(peerDomain => {
    if (getProviderIdentity(peerDomain) !== getProviderIdentity(domain))
      return []
    const peerRecords = getRecordGroupCellRecords(recordGroup, name, peerDomain.id)
    if (peerRecords.length !== 1 || !hasSameRecordGroupValues(peerRecords[0], recordGroup))
      return []
    const signature = getProviderSpecificSignature(peerDomain, peerRecords[0])
    return signature ? [signature] : []
  })
  if (signatures.length < 2)
    return false

  const counts = new Map<string, number>()
  for (const signature of signatures)
    counts.set(signature, (counts.get(signature) ?? 0) + 1)
  const expectedSignature = [...counts.entries()].sort((left, right) => right[1] - left[1])[0]?.[0]
  return currentSignature !== expectedSignature
}

function getRecordGroupStats(recordGroup: AutomaticRecordGroup): RecordGroupStats {
  const stats: RecordGroupStats = {
    targetCount: recordGroup.recordNames.length * domains.value.length,
    syncedCount: 0,
    driftedCount: 0,
    missingCount: 0,
    ambiguousCount: 0,
    unknownCount: 0,
  }
  for (const name of recordGroup.recordNames) {
    for (const domain of domains.value) {
      const status = getRecordGroupCellStatus(recordGroup, name, domain)
      if (status === 'synced')
        stats.syncedCount++
      else if (status === 'drifted')
        stats.driftedCount++
      else if (status === 'missing')
        stats.missingCount++
      else if (status === 'ambiguous')
        stats.ambiguousCount++
      else
        stats.unknownCount++
    }
  }
  return stats
}

function getRecordGroupValueParts(recordGroup: AutomaticRecordGroup) {
  const parts = [recordGroup.values.content, `TTL ${recordGroup.values.ttl}`]
  if (recordGroup.values.priority !== undefined)
    parts.push(`${$gettext('Priority')} ${recordGroup.values.priority}`)
  if (recordGroup.values.weight !== undefined)
    parts.push(`${$gettext('Weight')} ${recordGroup.values.weight}`)
  return parts
}

function getRecordGroupCoverage(stats: RecordGroupStats) {
  return `${stats.syncedCount}/${stats.targetCount}`
}

function getRecordGroupStatusItems(stats: RecordGroupStats) {
  const items: { key: keyof RecordGroupStats, color: string, label: string }[] = []
  if (stats.syncedCount > 0) {
    items.push({
      key: 'syncedCount',
      color: 'success',
      label: $gettext('%{count} synced', { count: String(stats.syncedCount) }),
    })
  }
  if (stats.driftedCount > 0) {
    items.push({
      key: 'driftedCount',
      color: 'warning',
      label: $gettext('%{count} drifted', { count: String(stats.driftedCount) }),
    })
  }
  if (stats.missingCount > 0) {
    items.push({
      key: 'missingCount',
      color: 'processing',
      label: $gettext('%{count} missing', { count: String(stats.missingCount) }),
    })
  }
  if (stats.ambiguousCount > 0) {
    items.push({
      key: 'ambiguousCount',
      color: 'error',
      label: $gettext('%{count} duplicated', { count: String(stats.ambiguousCount) }),
    })
  }
  if (stats.unknownCount > 0) {
    items.push({
      key: 'unknownCount',
      color: 'default',
      label: $gettext('%{count} unknown', { count: String(stats.unknownCount) }),
    })
  }
  return items
}

function getRecordGroupCellDetail(recordGroup: AutomaticRecordGroup, name: string, domain: DNSDomain) {
  const status = getRecordGroupCellStatus(recordGroup, name, domain)
  const records = getRecordGroupCellRecords(recordGroup, name, domain.id)
  if (status === 'unknown')
    return loadErrors.value[domain.id]
  if (status === 'missing')
    return $gettext('Record is missing')
  if (status === 'ambiguous')
    return $gettext('%{count} matching records found', { count: String(records.length) })
  const record = records[0]
  const parts = [`${record.content} · TTL ${record.ttl}`]
  if (record.line)
    parts.push(`${$gettext('Resolution Line')}: ${record.line}`)
  if (record.proxied !== undefined)
    parts.push(record.proxied ? $gettext('Proxied') : $gettext('DNS Only'))
  if (record.comment)
    parts.push(`${$gettext('Comment')}: ${record.comment}`)
  return parts.join(' · ')
}

function resetOperation() {
  operationMode.value = 'create'
  operationRecordGroupKey.value = null
  recordGroupScope.value = 'all'
  selectedRecordNames.value = []
  selectedDomainIds.value = []
  operationSourceKey.value = null
  formModel.value = createEmptyPayload()
  providerFieldModels.value = {}
  touchedProviderFields.value = {}
  recordLinesByDomainId.value = {}
  recordLineLoadingByDomainId.value = {}
  recordLineErrors.value = {}
}

function getProviderCode(domain: DNSDomain) {
  return domain.dns_credential?.provider_code?.trim().toLowerCase() ?? ''
}

function isCloudflareDomain(domain: DNSDomain) {
  return getProviderCode(domain) === 'cloudflare'
    || domain.dns_credential?.provider?.toLowerCase().includes('cloudflare') === true
}

function isAliDNSDomain(domain: DNSDomain) {
  return getProviderCode(domain) === 'alidns'
}

function isHuaweiCloudDomain(domain: DNSDomain) {
  return getProviderCode(domain) === 'huaweicloud'
}

function supportsRecordLines(domain: DNSDomain) {
  return isAliDNSDomain(domain) || isHuaweiCloudDomain(domain)
}

function hasProviderSpecificFields(domain: DNSDomain) {
  return supportsRecordLines(domain) || isCloudflareDomain(domain)
}

function getDefaultRecordLine(domain: DNSDomain) {
  if (isAliDNSDomain(domain))
    return 'default'
  if (isHuaweiCloudDomain(domain))
    return 'default_view'
  return undefined
}

function getOperationRecord(domainId: number) {
  if (operationMode.value === 'recordGroup') {
    for (const name of selectedRecordNames.value) {
      const row = aggregatedRecords.value.find(record =>
        getTypedNameKey(record.type, record.name) === getTypedNameKey(formModel.value.type, name),
      )
      const record = row?.members[domainId]?.[0]
      if (record)
        return record
    }
  }
  return undefined
}

function initializeProviderField(domain: DNSDomain) {
  if (providerFieldModels.value[domain.id])
    return

  const record = operationMode.value === 'recordGroup' ? undefined : getOperationRecord(domain.id)
  providerFieldModels.value[domain.id] = {
    line: record?.line || getDefaultRecordLine(domain),
    proxied: isCloudflareDomain(domain) ? record?.proxied ?? false : undefined,
    comment: isCloudflareDomain(domain) ? record?.comment ?? '' : undefined,
  }
  touchedProviderFields.value[domain.id] = []
}

function initializeProviderFields(domainIds: number[]) {
  for (const domain of domains.value) {
    if (domainIds.includes(domain.id) && hasProviderSpecificFields(domain))
      initializeProviderField(domain)
  }
}

async function fetchRecordLines(domain: DNSDomain) {
  if (!supportsRecordLines(domain) || recordLinesByDomainId.value[domain.id])
    return

  recordLineLoadingByDomainId.value[domain.id] = true
  delete recordLineErrors.value[domain.id]
  try {
    const { data } = await dnsApi.listRecordLines(domain.id)
    recordLinesByDomainId.value[domain.id] = data
  }
  catch (error) {
    recordLineErrors.value[domain.id] = getErrorMessage(error)
  }
  finally {
    recordLineLoadingByDomainId.value[domain.id] = false
  }
}

function loadRecordLines(domainIds: number[]) {
  for (const domain of domains.value) {
    if (domainIds.includes(domain.id))
      void fetchRecordLines(domain)
  }
}

function getRecordLineOptions(domain: DNSDomain) {
  const lines = [...(recordLinesByDomainId.value[domain.id] ?? [])]
  const currentLine = providerFieldModels.value[domain.id]?.line?.trim()
  const defaultLine = getDefaultRecordLine(domain)

  if (defaultLine && !lines.some(line => line.code === defaultLine))
    lines.unshift({ code: defaultLine, display_name: $gettext('Default') })
  if (currentLine && !lines.some(line => line.code === currentLine))
    lines.push({ code: currentLine, display_name: currentLine })

  return lines.map(line => {
    const name = line.display_name || line.name || line.code
    return {
      label: name === line.code ? name : `${name} (${line.code})`,
      value: line.code,
    }
  })
}

function markProviderFieldTouched(domainId: number, field: ProviderFieldKey) {
  const fields = touchedProviderFields.value[domainId] ?? []
  if (!fields.includes(field))
    touchedProviderFields.value[domainId] = [...fields, field]
}

function updateProviderLine(domainId: number, value: unknown) {
  if (typeof value !== 'string')
    return
  providerFieldModels.value[domainId].line = value
  markProviderFieldTouched(domainId, 'line')
}

function updateProviderProxied(domainId: number, value: unknown) {
  if (typeof value !== 'boolean')
    return
  providerFieldModels.value[domainId].proxied = value
  markProviderFieldTouched(domainId, 'proxied')
}

function updateProviderComment(domainId: number, value: unknown) {
  if (typeof value !== 'string')
    return
  providerFieldModels.value[domainId].comment = value
  markProviderFieldTouched(domainId, 'comment')
}

function isProviderFieldTouched(domainId: number, field: ProviderFieldKey) {
  return touchedProviderFields.value[domainId]?.includes(field) === true
}

function hasTouchedProviderFields(domainId: number) {
  return (touchedProviderFields.value[domainId]?.length ?? 0) > 0
}

function handleTargetDomainsChange(value: unknown) {
  if (!Array.isArray(value))
    return
  const domainIds = value.filter((item): item is number => typeof item === 'number')
  selectedDomainIds.value = domainIds
  initializeProviderFields(domainIds)
  loadRecordLines(domainIds)
}

function isRecordLineDisabled(domain: DNSDomain) {
  return isHuaweiCloudDomain(domain) && Boolean(getOperationRecord(domain.id))
}

function openCreateDrawer() {
  resetOperation()
  selectedDomainIds.value = domains.value.map(domain => domain.id)
  initializeProviderFields(selectedDomainIds.value)
  loadRecordLines(selectedDomainIds.value)
  isOperationDrawerOpen.value = true
}

function openRecordGroupOperation(recordGroup: AutomaticRecordGroup, initialNames = recordGroup.recordNames) {
  resetOperation()
  operationMode.value = 'recordGroup'
  operationRecordGroupKey.value = recordGroup.key
  recordGroupScope.value = initialNames.length === recordGroup.recordNames.length ? 'all' : 'selected'
  selectedRecordNames.value = [...initialNames]
  selectedDomainIds.value = domains.value.map(domain => domain.id)
  formModel.value = getRecordGroupPayload(recordGroup, initialNames[0] ?? recordGroup.recordNames[0] ?? '@')

  const source = recordSources.value.find(item =>
    selectedDomainIds.value.includes(item.domain.id)
    && item.record.type.toUpperCase() === recordGroup.type
    && selectedRecordNames.value.some(name => normalizeRecordName(name) === normalizeRecordName(item.record.name))
    && hasSameRecordGroupValues(item.record, recordGroup),
  )
  operationSourceKey.value = source?.key ?? null
  initializeProviderFields(selectedDomainIds.value)
  loadRecordLines(selectedDomainIds.value)
  isOperationDrawerOpen.value = true
}

function closeOperationDrawer() {
  if (isApplying.value)
    return
  isOperationDrawerOpen.value = false
  resetOperation()
}

function handleOperationSourceChange(value: unknown) {
  if (typeof value !== 'string')
    return
  operationSourceKey.value = value
  const source = recordSourceMap.value.get(value)
  if (!source)
    return
  formModel.value = {
    ...getCommonRecordPayload(source.record),
    name: selectedRecordNames.value[0] ?? source.record.name,
  }
}

function handleRecordGroupScopeChange(value: unknown) {
  if (value !== 'all' && value !== 'selected')
    return
  recordGroupScope.value = value
  if (value === 'all' && editingRecordGroup.value)
    selectedRecordNames.value = [...editingRecordGroup.value.recordNames]
}

function validateOperation() {
  if (selectedDomainIds.value.length === 0) {
    message.warning($gettext('Select at least one target domain'))
    return false
  }
  if (operationMode.value === 'recordGroup' && selectedRecordNames.value.length === 0) {
    message.warning($gettext('Select at least one record name'))
    return false
  }
  if (!formModel.value.type.trim()
    || (operationMode.value !== 'recordGroup' && !formModel.value.name.trim())
    || !formModel.value.content.trim()) {
    message.warning($gettext('Complete the required record fields'))
    return false
  }
  if (!Number.isFinite(formModel.value.ttl) || formModel.value.ttl < 1) {
    message.warning($gettext('TTL must be at least one second'))
    return false
  }
  return true
}

function buildPreviewTasks() {
  const commonPayload = {
    type: formModel.value.type.trim().toUpperCase(),
    content: formModel.value.content.trim(),
    ttl: formModel.value.ttl,
    priority: formModel.value.priority,
    weight: formModel.value.weight,
  }
  const targetNames = operationMode.value === 'recordGroup'
    ? selectedRecordNames.value
    : [formModel.value.name.trim()]

  return domains.value
    .filter(domain => selectedDomainIds.value.includes(domain.id))
    .flatMap(domain => targetNames.map<BatchTask>(name => {
      const payloadForName: RecordPayload = { ...commonPayload, name }
      const taskKey = getTaskKey(domain.id, payloadForName)
      if (loadErrors.value[domain.id]) {
        return {
          taskKey,
          domain,
          action: 'blocked',
          payload: payloadForName,
          reason: $gettext('Records could not be loaded for this domain'),
        }
      }

      const nextKey = getRecordKey(payloadForName)
      const matches = (recordsByDomainId.value[domain.id] ?? []).filter(record => getRecordKey(record) === nextKey)

      if (matches.length > 1) {
        return {
          taskKey,
          domain,
          action: 'blocked',
          payload: payloadForName,
          reason: $gettext('Multiple records have the same name and type'),
        }
      }
      if (matches.length === 0) {
        const payload = buildDomainPayload(domain, payloadForName)
        return {
          taskKey,
          domain,
          action: 'create',
          payload,
        }
      }

      const record = matches[0]
      const payload = buildDomainPayload(domain, payloadForName, record)
      if (isHuaweiCloudDomain(domain) && payload.line && record.line && payload.line !== record.line) {
        return {
          taskKey,
          domain,
          action: 'blocked',
          recordId: record.id,
          payload,
          reason: $gettext('Huawei Cloud does not support changing the resolution line of an existing record'),
        }
      }
      if (hasSameRecordValues(record, payload)) {
        return {
          taskKey,
          domain,
          action: 'skip',
          recordId: record.id,
          payload,
          reason: $gettext('Record already matches'),
        }
      }
      return {
        taskKey,
        domain,
        action: 'update',
        recordId: record.id,
        payload,
      }
    }))
}

function buildDomainPayload(domain: DNSDomain, commonPayload: RecordPayload, record?: DNSRecord) {
  const payload = record ? mergeRecordPayload(record, commonPayload) : { ...commonPayload }
  const fields = providerFieldModels.value[domain.id]
  if (!fields)
    return payload

  const shouldApplyField = (field: ProviderFieldKey) => !record
    || operationMode.value === 'create'
    || isProviderFieldTouched(domain.id, field)

  if (supportsRecordLines(domain) && shouldApplyField('line'))
    payload.line = fields.line
  if (isCloudflareDomain(domain) && shouldApplyField('proxied'))
    payload.proxied = fields.proxied
  if (isCloudflareDomain(domain) && shouldApplyField('comment'))
    payload.comment = fields.comment
  return payload
}

function openPreview() {
  if (!validateOperation())
    return
  previewTasks.value = buildPreviewTasks()
  isPreviewOpen.value = true
}

function getActionLabel(action: BatchAction) {
  const labels: Record<BatchAction, string> = {
    create: $gettext('Create'),
    update: $gettext('Update'),
    skip: $gettext('No change'),
    blocked: $gettext('Blocked'),
  }
  return labels[action]
}

function getActionColor(action: BatchAction) {
  const colors: Record<BatchAction, string> = {
    create: 'processing',
    update: 'warning',
    skip: 'default',
    blocked: 'error',
  }
  return colors[action]
}

function getTaskDetails(task: BatchTask) {
  if (task.reason)
    return task.reason

  const details = [task.payload.content, `TTL ${task.payload.ttl}`]
  if (task.payload.line)
    details.push(`${$gettext('Resolution Line')}: ${task.payload.line}`)
  if (task.payload.proxied !== undefined)
    details.push(task.payload.proxied ? $gettext('Proxied') : $gettext('DNS Only'))
  if (task.payload.comment)
    details.push(`${$gettext('Comment')}: ${task.payload.comment}`)
  return details.join(' · ')
}

async function executeTask(task: BatchTask): Promise<BatchResult> {
  try {
    if (task.action === 'create') {
      await dnsApi.createRecord(task.domain.id, task.payload)
    }
    else if (task.action === 'update' && task.recordId) {
      await dnsApi.updateRecord(task.domain.id, task.recordId, task.payload)
    }
    return {
      ...task,
      status: 'success',
    }
  }
  catch (error) {
    return {
      ...task,
      status: 'failed',
      detail: getErrorMessage(error),
    }
  }
}

async function executeActionableTasks(tasks: BatchTask[]) {
  if (tasks.length === 0)
    return []

  const firstResult = await executeTask(tasks[0])
  const remainingResults = await runWithConcurrency(tasks.slice(1), 3, executeTask)
  return [firstResult, ...remainingResults]
}

async function applyPreview() {
  isApplying.value = true
  try {
    const actionableTasks = previewTasks.value.filter(task => task.action === 'create' || task.action === 'update')
    const executedResults = await executeActionableTasks(actionableTasks)
    const executedByTask = new Map(executedResults.map(result => [result.taskKey, result]))
    batchResults.value = previewTasks.value.map(task => {
      const executed = executedByTask.get(task.taskKey)
      if (executed)
        return executed
      return {
        ...task,
        status: task.action === 'skip' ? 'skipped' : 'blocked',
        detail: task.reason,
      }
    })

    isPreviewOpen.value = false
    isOperationDrawerOpen.value = false
    isResultOpen.value = true
    await loadGroupRecords()

    const failedCount = batchResults.value.filter(result => result.status === 'failed').length
    if (failedCount > 0)
      message.warning($gettext('%{count} record operation(s) failed', { count: String(failedCount) }))
    else
      message.success($gettext('Group record operation completed'))
  }
  finally {
    isApplying.value = false
  }
}

async function retryFailed() {
  isApplying.value = true
  try {
    const retryTasks = failedResults.value.map(result => rebuildRetryTask(result))
    const actionableTasks = retryTasks.filter(task => task.action === 'create' || task.action === 'update')
    const executedResults = await executeActionableTasks(actionableTasks)
    const executedByTask = new Map(executedResults.map(result => [result.taskKey, result]))
    const resolvedResults = retryTasks.map<BatchResult>(task => {
      const executed = executedByTask.get(task.taskKey)
      if (executed)
        return executed
      return {
        ...task,
        status: task.action === 'skip' ? 'skipped' : 'blocked',
        detail: task.reason,
      }
    })
    const retryByTask = new Map(resolvedResults.map(result => [result.taskKey, result]))
    batchResults.value = batchResults.value.map(result => retryByTask.get(result.taskKey) ?? result)
    await loadGroupRecords()
  }
  finally {
    isApplying.value = false
  }
}

function rebuildRetryTask(result: BatchResult): BatchTask {
  if (loadErrors.value[result.domain.id]) {
    return {
      ...result,
      action: 'blocked',
      reason: $gettext('Records could not be loaded for this domain'),
    }
  }

  const currentRecords = recordsByDomainId.value[result.domain.id] ?? []
  const recordById = result.recordId
    ? currentRecords.find(record => record.id === result.recordId)
    : undefined
  const matches = recordById
    ? [recordById]
    : currentRecords.filter(record => getRecordKey(record) === getRecordKey(result.payload))

  if (matches.length > 1) {
    return {
      ...result,
      action: 'blocked',
      reason: $gettext('Multiple records have the same name and type'),
    }
  }
  if (matches.length === 0) {
    return {
      ...result,
      action: 'create',
      recordId: undefined,
      reason: undefined,
    }
  }

  const record = matches[0]
  if (hasSameRecordValues(record, result.payload)) {
    return {
      ...result,
      action: 'skip',
      recordId: record.id,
      reason: $gettext('Record already matches'),
    }
  }
  return {
    ...result,
    action: 'update',
    recordId: record.id,
    payload: result.payload,
    reason: undefined,
  }
}

function getResultLabel(status: BatchResultStatus) {
  const labels: Record<BatchResultStatus, string> = {
    success: $gettext('Succeeded'),
    failed: $gettext('Failed'),
    skipped: $gettext('Skipped'),
    blocked: $gettext('Blocked'),
  }
  return labels[status]
}

function getResultColor(status: BatchResultStatus) {
  const colors: Record<BatchResultStatus, string> = {
    success: 'success',
    failed: 'error',
    skipped: 'default',
    blocked: 'warning',
  }
  return colors[status]
}

function getBatchRowKey(record: BatchTask) {
  return record.taskKey
}

function closeResults() {
  isResultOpen.value = false
  batchResults.value = []
  resetOperation()
}

onMounted(loadGroupRecords)
</script>

<template>
  <AResult
    v-if="!group"
    status="404"
    :title="$gettext('DNS group not found')"
    :sub-title="$gettext('This local group may belong to another user or node, or it may have been deleted.')"
  >
    <template #extra>
      <AButton type="primary" @click="router.push({ name: 'DNS Groups' })">
        {{ $gettext('Back to groups') }}
      </AButton>
    </template>
  </AResult>

  <div v-else class="group-records-page">
    <AAlert
      v-if="missingDomainIds.length > 0"
      type="warning"
      show-icon
      class="mb-4"
      :message="$gettext('Some group domains are unavailable')"
      :description="$gettext('Edit the group to remove unavailable domain IDs: %{ids}', { ids: missingDomainIds.join(', ') })"
    />

    <AAlert
      v-if="Object.keys(loadErrors).length > 0"
      type="error"
      show-icon
      class="mb-4"
      :message="$gettext('Some DNS records could not be loaded')"
    >
      <template #description>
        <ul class="load-error-list">
          <li v-for="domain in domains.filter(item => loadErrors[item.id])" :key="domain.id">
            {{ domain.domain }}: {{ loadErrors[domain.id] }}
          </li>
        </ul>
      </template>
    </AAlert>

    <ACard>
      <template #title>
        <div class="group-card-title">
          <div>{{ group.name }}</div>
          <div v-if="group.description" class="group-description">
            {{ group.description }}
          </div>
        </div>
      </template>
      <template #extra>
        <div class="record-toolbar">
          <AInput
            v-model:value="searchKeyword"
            allow-clear
            :placeholder="$gettext('Search by name, type, or value')"
            class="record-search"
          />
          <AButton :loading="isLoading" @click="loadGroupRecords">
            <template #icon>
              <ReloadOutlined />
            </template>
            {{ $gettext('Refresh') }}
          </AButton>
          <AButton type="primary" :disabled="domains.length === 0" @click="openCreateDrawer">
            <template #icon>
              <PlusOutlined />
            </template>
            {{ $gettext('Add group record') }}
          </AButton>
        </div>
      </template>

      <ATable
        row-key="key"
        :columns="recordGroupColumns"
        :data-source="filteredRecordGroupRows"
        :loading="isLoading"
        :pagination="{ pageSize: 50, hideOnSinglePage: true }"
        :scroll="{ x: 1110 }"
      >
        <template #emptyText>
          <AEmpty :description="$gettext('No DNS records were found in this group')">
            <AButton type="primary" :disabled="domains.length === 0" @click="openCreateDrawer">
              {{ $gettext('Add group record') }}
            </AButton>
          </AEmpty>
        </template>
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'type'">
            <ATag>{{ (record as RecordGroupRow).recordGroup.type }}</ATag>
          </template>
          <template v-else-if="column.key === 'value'">
            <div
              v-for="value in getRecordGroupValueParts((record as RecordGroupRow).recordGroup)"
              :key="value"
              class="record-value"
            >
              {{ value }}
            </div>
          </template>
          <template v-else-if="column.key === 'names'">
            <ASpace wrap size="small">
              <ATag v-for="name in (record as RecordGroupRow).recordGroup.recordNames" :key="name">
                {{ name }}
              </ATag>
            </ASpace>
          </template>
          <template v-else-if="column.key === 'coverage'">
            <div class="coverage-value">
              {{ getRecordGroupCoverage((record as RecordGroupRow).stats) }}
            </div>
            <div class="coverage-label">
              {{ $gettext('targets synced') }}
            </div>
          </template>
          <template v-else-if="column.key === 'status'">
            <ASpace wrap size="small">
              <ATag
                v-for="item in getRecordGroupStatusItems((record as RecordGroupRow).stats)"
                :key="item.key"
                :color="item.color"
              >
                {{ item.label }}
              </ATag>
            </ASpace>
          </template>
          <template v-else-if="column.key === 'actions'">
            <AButton type="link" size="small" @click="openRecordGroupOperation((record as RecordGroupRow).recordGroup)">
              <template #icon>
                <SyncOutlined />
              </template>
              {{ $gettext('Edit and sync') }}
            </AButton>
          </template>
        </template>
        <template #expandedRowRender="{ record }">
          <div class="record-group-matrix-wrap">
            <div class="record-group-matrix-heading">
              {{ $gettext('Name by domain') }}
            </div>
            <table class="record-group-matrix">
              <thead>
                <tr>
                  <th>{{ $gettext('Name') }}</th>
                  <th v-for="domain in domains" :key="domain.id">
                    <span>{{ domain.domain }}</span>
                    <small>{{ domain.dns_credential?.provider ?? $gettext('Unknown provider') }}</small>
                  </th>
                  <th>{{ $gettext('Actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="name in (record as RecordGroupRow).recordGroup.recordNames" :key="name">
                  <th>{{ name }}</th>
                  <td v-for="domain in domains" :key="domain.id">
                    <ATooltip :title="getRecordGroupCellDetail((record as RecordGroupRow).recordGroup, name, domain)">
                      <ATag :color="getStatusColor(getRecordGroupCellStatus((record as RecordGroupRow).recordGroup, name, domain))">
                        {{ getStatusLabel(getRecordGroupCellStatus((record as RecordGroupRow).recordGroup, name, domain)) }}
                      </ATag>
                    </ATooltip>
                  </td>
                  <td>
                    <AButton
                      type="link"
                      size="small"
                      @click="openRecordGroupOperation((record as RecordGroupRow).recordGroup, [name])"
                    >
                      {{ $gettext('Sync this name') }}
                    </AButton>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
      </ATable>
    </ACard>

    <ADrawer
      :open="isOperationDrawerOpen"
      :title="operationMode === 'create'
        ? $gettext('Add group record')
        : $gettext('Edit and synchronize records')"
      width="640"
      @close="closeOperationDrawer"
    >
      <AForm layout="vertical">
        <template v-if="operationMode === 'recordGroup' && editingRecordGroup">
          <AAlert
            type="info"
            show-icon
            class="mb-4"
            :message="$gettext('Automatically grouped by shared value')"
            :description="$gettext('Choose which names to synchronize. Existing provider-specific values are preserved unless you change them below.')"
          />
          <AFormItem v-if="editingRecordGroup.recordNames.length > 1" :label="$gettext('Edit scope')" required>
            <ARadioGroup :value="recordGroupScope" button-style="solid" @change="event => handleRecordGroupScopeChange(event.target.value)">
              <ARadioButton value="all">
                {{ $gettext('All names') }}
              </ARadioButton>
              <ARadioButton value="selected">
                {{ $gettext('Selected names') }}
              </ARadioButton>
            </ARadioGroup>
          </AFormItem>
          <AFormItem v-if="recordGroupScope === 'selected'" :label="$gettext('Record names')" required>
            <ASelect
              v-model:value="selectedRecordNames"
              mode="multiple"
              max-tag-count="responsive"
              :options="editingRecordGroup.recordNames.map(name => ({ label: name, value: name }))"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Use values from')">
            <ASelect
              :value="operationSourceKey ?? undefined"
              show-search
              option-filter-prop="label"
              :options="operationSourceOptions"
              @change="handleOperationSourceChange"
            />
          </AFormItem>
        </template>
        <AFormItem :label="$gettext('Target domains')" required>
          <ASelect
            :value="selectedDomainIds"
            mode="multiple"
            show-search
            option-filter-prop="label"
            max-tag-count="responsive"
            :options="domainOptions"
            @change="handleTargetDomainsChange"
          />
          <div class="form-help">
            {{ $gettext('%{count} name and domain targets will be reviewed.', { count: String(operationTargetCount) }) }}
          </div>
        </AFormItem>
      </AForm>

      <DNSRecordForm v-model:record="formModel" :show-name="operationMode !== 'recordGroup'" />

      <section v-if="providerFieldDomains.length > 0" class="provider-fields-section">
        <div class="provider-fields-heading">
          <div class="provider-fields-title">
            {{ $gettext('Provider-specific settings') }}
          </div>
          <div class="provider-fields-description">
            {{ operationMode === 'recordGroup'
              ? $gettext('Existing values are preserved per name. Changing a field overrides it for every selected name on that domain; new records use the shown defaults.')
              : $gettext('Each setting is sent only to its matching target domain.') }}
          </div>
        </div>

        <div
          v-for="domain in providerFieldDomains"
          :key="domain.id"
          class="provider-domain-fields"
        >
          <div class="provider-domain-heading">
            <span>{{ domain.domain }}</span>
            <ASpace wrap size="small">
              <ATag>{{ domain.dns_credential?.provider ?? $gettext('Unknown provider') }}</ATag>
              <ATag
                v-if="operationMode === 'recordGroup'"
                :color="hasTouchedProviderFields(domain.id) ? 'warning' : 'default'"
              >
                {{ hasTouchedProviderFields(domain.id)
                  ? $gettext('Override selected names')
                  : $gettext('Preserve existing') }}
              </ATag>
            </ASpace>
          </div>

          <AForm layout="vertical">
            <AFormItem
              v-if="supportsRecordLines(domain)"
              :label="$gettext('Resolution Line')"
              :help="recordLineErrors[domain.id]"
              :validate-status="recordLineErrors[domain.id] ? 'error' : undefined"
              required
            >
              <ASelect
                :value="providerFieldModels[domain.id].line"
                :options="getRecordLineOptions(domain)"
                :loading="recordLineLoadingByDomainId[domain.id]"
                :disabled="isRecordLineDisabled(domain)"
                show-search
                option-filter-prop="label"
                @update:value="value => updateProviderLine(domain.id, value)"
              />
              <div v-if="isRecordLineDisabled(domain)" class="provider-field-hint">
                {{ $gettext('Huawei Cloud resolution lines cannot be changed after record creation.') }}
              </div>
            </AFormItem>

            <AFormItem v-if="isCloudflareDomain(domain)" :label="$gettext('Proxied')">
              <ASwitch
                :checked="providerFieldModels[domain.id].proxied"
                @update:checked="value => updateProviderProxied(domain.id, value)"
              />
            </AFormItem>

            <AFormItem v-if="isCloudflareDomain(domain)" :label="$gettext('Comment')">
              <ATextarea
                :value="providerFieldModels[domain.id].comment"
                :placeholder="$gettext('Optional comment for this DNS record')"
                :auto-size="{ minRows: 2, maxRows: 4 }"
                @update:value="value => updateProviderComment(domain.id, value)"
              />
            </AFormItem>
          </AForm>
        </div>
      </section>

      <template #footer>
        <ASpace>
          <AButton :disabled="isApplying" @click="closeOperationDrawer">
            {{ $gettext('Cancel') }}
          </AButton>
          <AButton type="primary" @click="openPreview">
            {{ $gettext('Preview changes') }}
          </AButton>
        </ASpace>
      </template>
    </ADrawer>

    <AModal
      v-model:open="isPreviewOpen"
      :title="$gettext('Review group record changes')"
      width="920px"
      :confirm-loading="isApplying"
      :ok-text="$gettext('Apply changes')"
      :cancel-text="$gettext('Back')"
      :ok-button-props="{ disabled: actionablePreviewCount === 0 }"
      @ok="applyPreview"
    >
      <AAlert
        type="warning"
        show-icon
        class="mb-4"
        :message="$gettext('Changes are applied per domain and cannot be rolled back as one transaction.')"
      />
      <ATable
        :row-key="getBatchRowKey"
        size="small"
        :columns="previewColumns"
        :data-source="previewTasks"
        :pagination="false"
        :scroll="{ y: 360 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'domain'">
            {{ (record as BatchTask).domain.domain }}
          </template>
          <template v-else-if="column.key === 'name'">
            {{ (record as BatchTask).payload.name }}
          </template>
          <template v-else-if="column.key === 'action'">
            <ATag :color="getActionColor((record as BatchTask).action)">
              {{ getActionLabel((record as BatchTask).action) }}
            </ATag>
          </template>
          <template v-else-if="column.key === 'details'">
            {{ getTaskDetails(record as BatchTask) }}
          </template>
        </template>
      </ATable>
    </AModal>

    <AModal
      v-model:open="isResultOpen"
      :title="$gettext('Group record results')"
      width="940px"
      :closable="!isApplying"
      :mask-closable="!isApplying"
      @cancel="closeResults"
    >
      <ATable
        :row-key="getBatchRowKey"
        size="small"
        :columns="resultColumns"
        :data-source="batchResults"
        :pagination="false"
        :scroll="{ y: 420 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'domain'">
            {{ (record as BatchResult).domain.domain }}
          </template>
          <template v-else-if="column.key === 'name'">
            {{ (record as BatchResult).payload.name }}
          </template>
          <template v-else-if="column.key === 'action'">
            {{ getActionLabel((record as BatchResult).action) }}
          </template>
          <template v-else-if="column.key === 'status'">
            <ATag :color="getResultColor((record as BatchResult).status)">
              {{ getResultLabel((record as BatchResult).status) }}
            </ATag>
          </template>
          <template v-else-if="column.key === 'details'">
            {{ (record as BatchResult).detail || '-' }}
          </template>
        </template>
      </ATable>
      <template #footer>
        <ASpace>
          <AButton v-if="failedResults.length > 0" :loading="isApplying" @click="retryFailed">
            {{ $gettext('Retry failed') }}
          </AButton>
          <AButton type="primary" :disabled="isApplying" @click="closeResults">
            {{ $gettext('Close') }}
          </AButton>
        </ASpace>
      </template>
    </AModal>

    <FooterToolBar>
      <AButton @click="router.push({ name: 'DNS Groups' })">
        <template #icon>
          <ArrowLeftOutlined />
        </template>
        {{ $gettext('Back to groups') }}
      </AButton>
    </FooterToolBar>
  </div>
</template>

<style scoped>
.group-records-page {
  padding-bottom: 72px;
}

.group-description {
  margin-top: 4px;
  color: var(--ant-color-text-secondary);
  font-size: 12px;
  font-weight: 400;
}

.group-card-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
}

.group-card-title > :first-child {
  flex: 0 0 auto;
}

.group-card-title .group-description {
  flex: 1 1 240px;
  margin-top: 0;
}

.record-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.record-search {
  width: min(260px, 60vw);
}

.record-value {
  max-width: 44ch;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.form-help,
.coverage-label {
  margin-top: 4px;
  color: var(--ant-color-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.record-group-matrix-heading {
  color: var(--ant-color-text);
  font-weight: 600;
}

.coverage-value {
  color: var(--ant-color-text);
  font-size: 16px;
  font-weight: 600;
}

.record-group-matrix-wrap {
  max-width: 100%;
  overflow-x: auto;
}

.record-group-matrix-heading {
  margin-bottom: 12px;
}

.record-group-matrix {
  width: 100%;
  min-width: 720px;
  border-collapse: collapse;
  background: var(--ant-color-bg-container);
}

.record-group-matrix th,
.record-group-matrix td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--ant-color-border-secondary);
  text-align: left;
  vertical-align: middle;
}

.record-group-matrix thead th {
  color: var(--ant-color-text-secondary);
  font-size: 12px;
  font-weight: 500;
}

.record-group-matrix tbody th {
  color: var(--ant-color-text);
  font-weight: 600;
}

.record-group-matrix th small {
  display: block;
  margin-top: 2px;
  color: var(--ant-color-text-tertiary);
  font-weight: 400;
}

.load-error-list {
  margin: 0;
  padding-left: 18px;
}

.provider-fields-section {
  margin-top: 24px;
}

.provider-fields-heading {
  margin-bottom: 12px;
}

.provider-fields-title {
  color: var(--ant-color-text);
  font-size: 16px;
  font-weight: 600;
}

.provider-fields-description,
.provider-field-hint {
  margin-top: 4px;
  color: var(--ant-color-text-secondary);
  font-size: 13px;
}

.provider-domain-fields {
  padding: 16px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius-lg);
  background: var(--ant-color-fill-quaternary);
}

.provider-domain-fields + .provider-domain-fields {
  margin-top: 12px;
}

.provider-domain-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  font-weight: 600;
}

.provider-domain-fields :deep(.ant-form-item:last-child) {
  margin-bottom: 0;
}

@media (max-width: 720px) {
  .record-toolbar,
  .record-search {
    width: 100%;
  }

  .record-toolbar :deep(.ant-btn) {
    flex: 1;
  }

  .group-card-title {
    width: 100%;
  }
}
</style>
