<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { DNSDomain, DNSRecord } from '@/api/dns'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import type { SiteDNSRecord } from '@/api/site'
import { Tag } from 'antdv-next'
import { Fragment, h } from 'vue'
import { isAllowedDnsProvider } from '@/constants/dns_providers'
import { useDnsStore } from '@/pinia/moudule/dns'
import { useSiteEditorStore } from '../SiteEditor/store'

const { message } = useGlobalApp()
const dnsStore = useDnsStore()
const editorStore = useSiteEditorStore()
const { ngxConfig, dnsLinked, linkedDNSName, data } = storeToRefs(editorStore)

interface LinkedDNSRecord {
  record: DNSRecord
  domain: DNSDomain
  exists: boolean
  recreateContent: string
  recreateTTL: number
  recreateProxied: boolean
}

const selectedDomainId = ref<number | null>(null)
const selectedRecordIds = ref<string[]>([])
const createNewRecord = ref(false)
const loading = ref(false)
const initialLoading = ref(true) // Loading state for initial DNS link check
const availableDomains = ref<DNSDomain[]>([])
const availableRecords = ref<DNSRecord[]>([])
const linkedRecords = ref<LinkedDNSRecord[]>([])
const newRecordForm = reactive({
  type: 'A',
  content: '',
  ttl: 600,
  proxied: false,
})

const recordTypes = ['A', 'AAAA', 'CNAME']

const recordTypeOptions: SelectProps['options'] = recordTypes.map(type => ({
  label: type,
  value: type,
}))

const domainOptions = computed<SelectProps['options']>(() => availableDomains.value.map(domain => ({
  key: domain.id,
  value: domain.id,
  label: h(Fragment, null, [
    domain.domain,
    domain.dns_credential
      ? h('span', { class: 'text-gray-400' }, [
          ' (',
          domain.dns_credential.name,
          ')',
        ])
      : null,
  ]),
})))

// Computed properties for v-model bindings to handle null values
const selectedDomainValue = computed({
  get: () => selectedDomainId.value ?? undefined,
  set: val => {
    selectedDomainId.value = typeof val === 'number' ? val : null
  },
})

const selectedRecordValue = computed({
  get: () => selectedRecordIds.value,
  set: val => {
    selectedRecordIds.value = Array.isArray(val)
      ? val.filter((recordId): recordId is string => typeof recordId === 'string')
      : []
  },
})

const selectableRecords = computed(() => {
  const records = [...availableRecords.value]
  const recordIds = new Set(records.map(record => record.id))
  for (const linkedRecord of linkedRecords.value) {
    if (!recordIds.has(linkedRecord.record.id)) {
      records.push(linkedRecord.record)
    }
  }
  return records
})

const recordOptions = computed<SelectProps['options']>(() => selectableRecords.value.map(record => {
  const proxiedLabel = record.proxied ? $gettext('Proxied') : ''

  return {
    key: record.id,
    value: record.id,
    label: h(Fragment, null, [
      h(Tag, {
        color: record.type === 'A' ? 'blue' : record.type === 'AAAA' ? 'green' : 'orange',
      }, { default: () => record.type }),
      ' ',
      record.name === '@'
        ? availableDomains.value.find(domain => domain.id === selectedDomainId.value)?.domain
        : record.name,
      ' → ',
      record.content,
      record.proxied
        ? h(Tag, { color: 'orange', class: 'ml-2' }, { default: () => proxiedLabel })
        : null,
    ]),
  }
}))

const existingLinkedRecords = computed(() => linkedRecords.value.filter(record => record.exists))
const missingLinkedRecords = computed(() => linkedRecords.value.filter(record => !record.exists))

// Get server_name value from config
const serverNameValue = computed(() => {
  const servers = ngxConfig.value.servers

  for (const server of Object.values(servers) as NgxServer[]) {
    if (!server.directives)
      continue

    for (const directive of Object.values(server.directives) as NgxDirective[]) {
      if (directive.directive === 'server_name' && directive.params.trim() !== '') {
        // Return first domain from server_name
        const names = directive.params.trim().split(/\s+/)
        return names[0] || ''
      }
    }
  }

  return ''
})

const hasServerName = computed(() => serverNameValue.value !== '')

// Get full DNS name from linked record
function getFullDNSName(record: DNSRecord, domain: DNSDomain): string {
  if (record.name === '@') {
    return domain.domain
  }
  return `${record.name}.${domain.domain}`
}

// Update server_name directive with DNS name
function updateServerNameDirective(dnsNames: string[]) {
  // Find and update server_name directive in the first server
  const servers = ngxConfig.value.servers
  if (servers && servers.length > 0) {
    const directives = servers[0].directives
    if (directives) {
      // Find server_name directive
      const serverNameDirective = Object.values(directives).find(
        (d): d is NgxDirective => (d as NgxDirective).directive === 'server_name',
      ) as NgxDirective | undefined

      if (serverNameDirective) {
        serverNameDirective.params = dnsNames.join(' ')
      }
    }
  }
}

// Extract domain from server_name (e.g., "example.com" or "www.example.com")
function extractDomain(serverName: string): string | null {
  // Remove wildcard and trim
  const cleaned = serverName.replace(/^\*\./, '').trim()

  // Split by space (multiple domains)
  const domains = cleaned.split(/\s+/)
  if (domains.length === 0)
    return null

  // Get first domain
  const firstDomain = domains[0]

  // Extract base domain (handle subdomains)
  const parts = firstDomain.split('.')
  if (parts.length >= 2) {
    // Return last two parts as base domain
    return parts.slice(-2).join('.')
  }

  return firstDomain
}

// Extract subdomain from server_name
function extractSubdomain(serverName: string, baseDomain: string): string {
  const cleaned = serverName.replace(/^\*\./, '').trim()
  const domains = cleaned.split(/\s+/)
  if (domains.length === 0)
    return '@'

  const firstDomain = domains[0]

  if (firstDomain === baseDomain)
    return '@'

  // Remove base domain to get subdomain
  const subdomain = firstDomain.replace(`.${baseDomain}`, '')
  return subdomain || '@'
}

// Load available DNS domains
async function loadDomains() {
  try {
    loading.value = true
    await dnsStore.fetchDomains()
    // Filter only allowed DNS providers
    availableDomains.value = dnsStore.domains.filter(domain =>
      domain.dns_credential && isAllowedDnsProvider({
        code: domain.dns_credential.provider_code,
        provider: domain.dns_credential.provider,
        name: domain.dns_credential.name,
      }),
    )
  }
  catch (error) {
    console.error('Failed to load DNS domains:', error)
  }
  finally {
    loading.value = false
  }
}

// Load records for selected domain
async function loadRecordsForDomain(domainId: number) {
  try {
    loading.value = true
    await dnsStore.fetchRecords(domainId)
    availableRecords.value = dnsStore.records.filter(record =>
      recordTypes.includes(record.type),
    )
  }
  catch (error) {
    message.error($gettext('Failed to load DNS records'))
    console.error(error)
  }
  finally {
    loading.value = false
  }
}

function getStoredDNSRecords(): SiteDNSRecord[] {
  if (data.value.dns_records?.length) {
    return data.value.dns_records
  }
  if (!data.value.dns_record_id) {
    return []
  }
  return [{
    id: data.value.dns_record_id,
    name: data.value.dns_record_name || '',
    type: data.value.dns_record_type || 'A',
    exists: data.value.dns_record_exists !== false,
  }]
}

function toLinkedRecord(storedRecord: SiteDNSRecord, domain: DNSDomain): LinkedDNSRecord {
  const availableRecord = availableRecords.value.find(record => record.id === storedRecord.id)
  return {
    record: availableRecord || {
      id: storedRecord.id,
      name: storedRecord.name,
      type: storedRecord.type,
      content: '',
      ttl: 600,
    },
    domain,
    exists: Boolean(availableRecord),
    recreateContent: '',
    recreateTTL: 600,
    recreateProxied: false,
  }
}

function saveDNSLinks(domain: DNSDomain, records: LinkedDNSRecord[]) {
  const storedRecords = records.map(({ record, exists }) => ({
    id: record.id,
    name: record.name,
    type: record.type,
    exists,
  }))
  const firstRecord = storedRecords[0]

  data.value.dns_domain_id = domain.id
  data.value.dns_records = storedRecords
  data.value.dns_record_id = firstRecord?.id || null
  data.value.dns_record_name = firstRecord?.name || null
  data.value.dns_record_type = firstRecord?.type || null
  data.value.dns_record_exists = firstRecord?.exists ?? null
}

function clearDNSLinks() {
  selectedRecordIds.value = []
  linkedRecords.value = []
  dnsLinked.value = false
  linkedDNSName.value = ''
  data.value.dns_domain_id = null
  data.value.dns_records = []
  data.value.dns_record_id = null
  data.value.dns_record_name = null
  data.value.dns_record_type = null
  data.value.dns_record_exists = null
}

function applyLinkedRecords(records: LinkedDNSRecord[], domain: DNSDomain, shouldUpdateServerName: boolean) {
  linkedRecords.value = records
  const dnsNames = records.map(({ record }) => getFullDNSName(record, domain))
  dnsLinked.value = records.length > 0
  linkedDNSName.value = dnsNames.join(' ')
  saveDNSLinks(domain, records)
  if (shouldUpdateServerName) {
    updateServerNameDirective(dnsNames)
  }
}

// Load available DNS domains on mount
onMounted(async () => {
  try {
    initialLoading.value = true
    await loadDomains()

    const storedRecords = getStoredDNSRecords()
    if (data.value.dns_domain_id && storedRecords.length > 0) {
      selectedDomainId.value = data.value.dns_domain_id
      await loadRecordsForDomain(data.value.dns_domain_id)

      const domain = availableDomains.value.find(d => d.id === data.value.dns_domain_id)
      if (domain) {
        const records = storedRecords.map(record => toLinkedRecord(record, domain))
        selectedRecordIds.value = records.map(({ record }) => record.id)
        applyLinkedRecords(records, domain, false)
      }
    }
    else {
      await autoMatchDomain()
    }
  }
  finally {
    initialLoading.value = false
  }
})

// Helper function to auto-match domain from server_name
async function autoMatchDomain() {
  if (serverNameValue.value) {
    const domainMatch = extractDomain(serverNameValue.value)
    if (domainMatch) {
      const matchingDomain = availableDomains.value.find(d => d.domain === domainMatch)
      if (matchingDomain) {
        selectedDomainId.value = matchingDomain.id
        await loadRecordsForDomain(matchingDomain.id)
      }
    }
  }
}

// Handle domain selection change
async function onDomainChange(value: unknown) {
  const domainId = typeof value === 'number' ? value : null
  clearDNSLinks()
  createNewRecord.value = false
  if (domainId) {
    await loadRecordsForDomain(domainId)
  }
  else {
    availableRecords.value = []
  }
}

// Handle record selection
function onRecordSelect(value: unknown) {
  const recordIds = Array.isArray(value)
    ? value.filter((recordId): recordId is string => typeof recordId === 'string')
    : []
  createNewRecord.value = false
  if (recordIds.length > 0 && selectedDomainId.value) {
    const domain = availableDomains.value.find(d => d.id === selectedDomainId.value)
    if (domain) {
      const records = recordIds.flatMap(recordId => {
        const record = selectableRecords.value.find(item => item.id === recordId)
        if (!record)
          return []
        const previousRecord = linkedRecords.value.find(item => item.record.id === recordId)
        return [{
          record,
          domain,
          exists: availableRecords.value.some(item => item.id === recordId),
          recreateContent: previousRecord?.recreateContent || '',
          recreateTTL: previousRecord?.recreateTTL || 600,
          recreateProxied: previousRecord?.recreateProxied || false,
        } satisfies LinkedDNSRecord]
      })
      applyLinkedRecords(records, domain, true)
      const dnsNames = records.map(({ record }) => getFullDNSName(record, domain)).join(', ')
      message.success($gettext('DNS record linked and server_name updated: %{name}').replace('%{name}', dnsNames))
    }
  }
  else {
    clearDNSLinks()
  }
}

// Handle create new record toggle
function onCreateNewToggle(e: { target: { checked: boolean } }) {
  const checked = e.target.checked
  if (checked) {
    clearDNSLinks()
    // Pre-fill form
    if (serverNameValue.value && selectedDomainId.value) {
      const domain = availableDomains.value.find(d => d.id === selectedDomainId.value)
      if (domain) {
        newRecordForm.type = 'A'
        newRecordForm.content = ''
        newRecordForm.ttl = 600
        newRecordForm.proxied = false
      }
    }
  }
}

// Create new DNS record
async function createRecord() {
  if (!selectedDomainId.value || !newRecordForm.content) {
    message.error($gettext('Please fill in all required fields'))
    return
  }

  try {
    loading.value = true
    const domain = availableDomains.value.find(d => d.id === selectedDomainId.value)
    if (!domain)
      return

    const subdomain = extractSubdomain(serverNameValue.value, domain.domain)

    const record = await dnsStore.createRecord(selectedDomainId.value, {
      type: newRecordForm.type,
      name: subdomain,
      content: newRecordForm.content,
      ttl: newRecordForm.ttl,
      proxied: newRecordForm.proxied,
    })

    message.success($gettext('DNS record created successfully'))
    const linkedRecord: LinkedDNSRecord = {
      record,
      domain,
      exists: true,
      recreateContent: '',
      recreateTTL: 600,
      recreateProxied: false,
    }
    applyLinkedRecords([linkedRecord], domain, true)

    // Reload records
    await loadRecordsForDomain(selectedDomainId.value)
    selectedRecordIds.value = [record.id]
    createNewRecord.value = false
  }
  catch (error) {
    message.error($gettext('Failed to create DNS record'))
    console.error(error)
  }
  finally {
    loading.value = false
  }
}

// Clear selection
function clearSelection() {
  selectedDomainId.value = null
  createNewRecord.value = false
  availableRecords.value = []
  clearDNSLinks()
}

// Recreate missing DNS record
async function recreateRecord(linkedRecord: LinkedDNSRecord) {
  if (!selectedDomainId.value || !linkedRecord.recreateContent)
    return

  try {
    loading.value = true
    const { record, domain } = linkedRecord

    const newRecord = await dnsStore.createRecord(selectedDomainId.value, {
      type: record.type,
      name: record.name,
      content: linkedRecord.recreateContent,
      ttl: linkedRecord.recreateTTL,
      proxied: linkedRecord.recreateProxied,
    })

    message.success($gettext('DNS record recreated successfully'))

    const updatedRecords = linkedRecords.value.map(item => item.record.id === record.id
      ? {
          record: newRecord,
          domain,
          exists: true,
          recreateContent: '',
          recreateTTL: 600,
          recreateProxied: false,
        }
      : item)
    selectedRecordIds.value = updatedRecords.map(item => item.record.id)
    applyLinkedRecords(updatedRecords, domain, true)

    // Reload records
    await loadRecordsForDomain(selectedDomainId.value)

    // Automatically save the site configuration with the updated DNS link
    await editorStore.save()
    message.success($gettext('Site configuration updated with recreated DNS record'))
  }
  catch (error) {
    message.error($gettext('Failed to recreate DNS record'))
    console.error(error)
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="px-6 pb-2">
    <AAlert
      v-if="!hasServerName"
      type="warning"
      class="mb-4"
      show-icon
      :title="$gettext('The parameter of server_name is required')"
      :description="$gettext('Please configure server_name directive in the configuration before linking DNS records.')"
    />

    <!-- Loading skeleton -->
    <div v-else-if="initialLoading" class="mb-4">
      <ASkeleton active :paragraph="{ rows: 4 }" />
    </div>

    <div v-else>
      <p class="text-gray-600 mb-4 text-sm">
        {{ $gettext('Link this site to a DNS record. The server_name will be used for the DNS record name.') }}
      </p>

      <!-- Current linked records -->
      <div v-if="linkedRecords.length" class="mb-4">
        <div v-if="existingLinkedRecords.length" class="p-3 border border-green-200 rounded">
          <div class="flex items-center justify-between mb-2">
            <div class="text-sm font-medium text-green-800">
              {{ $gettext('Linked DNS Record') }}
            </div>
            <AButton size="small" @click="clearSelection">
              {{ $gettext('Clear') }}
            </AButton>
          </div>
          <div
            v-for="linkedRecord in existingLinkedRecords"
            :key="linkedRecord.record.id"
            class="text-xs text-gray-600 not-last:mb-2"
          >
            <ATag :color="linkedRecord.record.type === 'A' ? 'blue' : linkedRecord.record.type === 'AAAA' ? 'green' : 'orange'">
              {{ linkedRecord.record.type }}
            </ATag>
            {{ linkedRecord.record.name === '@' ? linkedRecord.domain.domain : linkedRecord.record.name }}
            → {{ linkedRecord.record.content }}
            <ATag v-if="linkedRecord.record.proxied" color="orange" class="ml-1">
              {{ $gettext('Proxied') }}
            </ATag>
          </div>
        </div>

        <div
          v-for="linkedRecord in missingLinkedRecords"
          :key="linkedRecord.record.id"
          class="p-3 mt-2 border border-orange-200 rounded"
        >
          <div class="mb-2">
            <div class="text-sm font-medium text-orange-800 mb-1">
              {{ $gettext('DNS Record Missing') }}
            </div>
            <div class="text-xs text-gray-600 mb-2">
              <ATag :color="linkedRecord.record.type === 'A' ? 'blue' : linkedRecord.record.type === 'AAAA' ? 'green' : 'orange'">
                {{ linkedRecord.record.type }}
              </ATag>
              {{ linkedRecord.record.name === '@' ? linkedRecord.domain.domain : linkedRecord.record.name }}
            </div>
            <div class="text-xs text-orange-700 mb-3">
              {{ $gettext('The linked DNS record was deleted from the DNS server. You can recreate it or clear the link.') }}
            </div>
          </div>

          <AForm layout="vertical" size="small">
            <AFormItem :label="$gettext('IP Address / Target')" required>
              <AInput
                v-model:value="linkedRecord.recreateContent"
                size="small"
                :placeholder="linkedRecord.record.type === 'CNAME' ? $gettext('target.example.com') : $gettext('192.168.1.1')"
              />
            </AFormItem>
            <AFormItem :label="$gettext('TTL (seconds)')">
              <AInputNumber
                v-model:value="linkedRecord.recreateTTL"
                size="small"
                :min="60"
                :max="86400"
                style="width: 100%"
              />
            </AFormItem>
            <AFormItem>
              <ACheckbox v-model:checked="linkedRecord.recreateProxied">
                {{ $gettext('Enable Proxy (Cloudflare)') }}
              </ACheckbox>
            </AFormItem>
          </AForm>

          <div class="flex gap-2">
            <AButton
              type="primary"
              size="small"
              danger
              :loading="loading"
              :disabled="!linkedRecord.recreateContent"
              @click="recreateRecord(linkedRecord)"
            >
              {{ $gettext('Recreate DNS Record') }}
            </AButton>
            <AButton size="small" @click="clearSelection">
              {{ $gettext('Clear Link') }}
            </AButton>
          </div>
        </div>
      </div>

      <AForm layout="vertical">
        <AFormItem :label="$gettext('DNS Domain')">
          <ASelect
            v-model:value="selectedDomainValue"
            :placeholder="$gettext('Select DNS domain')"
            :loading="loading"
            :options="domainOptions"
            allow-clear
            @change="onDomainChange"
          />
        </AFormItem>

        <AFormItem
          v-if="selectedDomainId"
          :label="$gettext('DNS Record')"
        >
          <ASpace orientation="vertical" style="width: 100%">
            <ASelect
              v-model:value="selectedRecordValue"
              mode="multiple"
              :placeholder="$gettext('Select existing record')"
              :loading="loading"
              :disabled="createNewRecord"
              max-tag-count="responsive"
              :options="recordOptions"
              allow-clear
              @change="onRecordSelect"
            />

            <ACheckbox
              v-model:checked="createNewRecord"
              @change="onCreateNewToggle"
            >
              {{ $gettext('Create new DNS record') }}
            </ACheckbox>
          </ASpace>
        </AFormItem>

        <template v-if="createNewRecord && selectedDomainId">
          <AFormItem :label="$gettext('Record Type')">
            <ASelect
              v-model:value="newRecordForm.type"
              :options="recordTypeOptions"
            />
          </AFormItem>

          <AFormItem :label="$gettext('Record Name')">
            <AInput
              :value="extractSubdomain(serverNameValue, availableDomains.find(d => d.id === selectedDomainId)?.domain || '')"
              disabled
            />
            <div class="text-gray-500 text-xs mt-1">
              {{ $gettext('Automatically extracted from server_name') }}
            </div>
          </AFormItem>

          <AFormItem :label="$gettext('IP Address / Target')" required>
            <AInput
              v-model:value="newRecordForm.content"
              :placeholder="newRecordForm.type === 'CNAME' ? $gettext('target.example.com') : $gettext('192.168.1.1')"
            />
          </AFormItem>

          <AFormItem :label="$gettext('TTL (seconds)')">
            <AInputNumber
              v-model:value="newRecordForm.ttl"
              :min="60"
              :max="86400"
              style="width: 100%"
            />
          </AFormItem>

          <AFormItem>
            <ACheckbox v-model:checked="newRecordForm.proxied">
              {{ $gettext('Enable Proxy (Cloudflare)') }}
            </ACheckbox>
            <div class="text-gray-500 text-xs mt-1">
              {{ $gettext('Route traffic through proxy for additional protection and features') }}
            </div>
          </AFormItem>

          <AButton
            type="primary"
            :loading="loading"
            @click="createRecord"
          >
            {{ $gettext('Create DNS Record') }}
          </AButton>
        </template>

        <AAlert
          v-if="!availableDomains.length"
          type="info"
          :title="$gettext('No DNS domains available')"
          :description="$gettext('Please add a DNS domain first in the DNS management section.')"
          show-icon
          class="mt-4"
        />
      </AForm>
    </div>
  </div>
</template>

<style scoped lang="less">
</style>
