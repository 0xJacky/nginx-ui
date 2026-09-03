<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { DNSDomain, DNSRecord } from '@/api/dns'
import { isAllowedDnsProvider } from '@/constants/dns_providers'
import { useDnsStore } from '@/pinia/moudule/dns'

const props = defineProps<{
  serverName: string
}>()

const emit = defineEmits<{
  recordCreated: [record: DNSRecord, domain: DNSDomain]
  recordsSelected: [records: DNSRecord[], domain: DNSDomain]
  cleared: []
}>()

const { message } = useGlobalApp()
const dnsStore = useDnsStore()

const selectedDomainId = ref<number | null>(null)
const selectedRecordIds = ref<string[]>([])
const createNewRecord = ref(false)
const loading = ref(false)
const availableDomains = ref<DNSDomain[]>([])
const availableRecords = ref<DNSRecord[]>([])
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

function findDomainById(domainId: unknown) {
  return availableDomains.value.find(domain => domain.id === domainId)
}

function findRecordById(recordId: unknown) {
  return availableRecords.value.find(record => record.id === recordId)
}

const domainOptions = computed<SelectProps['options']>(() => availableDomains.value.map(domain => ({
  key: domain.id,
  label: domain.domain,
  value: domain.id,
  domainName: domain.domain,
  credentialName: domain.dns_credential?.name,
  hasCredential: Boolean(domain.dns_credential),
})))

const recordOptions = computed<SelectProps['options']>(() => availableRecords.value.map(record => {
  const recordName = record.name === '@'
    ? findDomainById(selectedDomainId.value)?.domain
    : record.name

  return {
    key: record.id,
    label: `${record.type} ${recordName ?? ''} → ${record.content}${record.proxied ? ` ${$gettext('Proxied')}` : ''}`,
    value: record.id,
    recordType: record.type,
    recordName: record.name,
    recordContent: record.content,
    isProxied: record.proxied,
  }
}))

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

// Watch for server name changes to extract domain
watch(() => props.serverName, newServerName => {
  if (!newServerName)
    return

  // Try to match domain from server_name
  const domainMatch = extractDomain(newServerName)
  if (domainMatch) {
    const matchingDomain = availableDomains.value.find(d => d.domain === domainMatch)
    if (matchingDomain) {
      selectedDomainId.value = matchingDomain.id
      loadRecordsForDomain(matchingDomain.id)
    }
  }
}, { immediate: true })

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

// Load available DNS domains on mount
onMounted(async () => {
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
})

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

// Handle domain selection change
function onDomainChange(value: unknown) {
  const domainId = typeof value === 'number' ? value : null
  selectedRecordIds.value = []
  createNewRecord.value = false
  emit('cleared')
  if (domainId) {
    loadRecordsForDomain(domainId)
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
    const records = recordIds.flatMap(recordId => {
      const record = availableRecords.value.find(item => item.id === recordId)
      return record ? [record] : []
    })
    const domain = availableDomains.value.find(d => d.id === selectedDomainId.value)
    if (records.length > 0 && domain) {
      emit('recordsSelected', records, domain)
    }
  }
  else {
    emit('cleared')
  }
}

// Handle create new record toggle
function onCreateNewToggle(e: { target: { checked: boolean } }) {
  const checked = e.target.checked
  if (checked) {
    selectedRecordIds.value = []
    emit('cleared')
    // Pre-fill record name from server_name
    if (props.serverName && selectedDomainId.value) {
      const domain = availableDomains.value.find(d => d.id === selectedDomainId.value)
      if (domain) {
        newRecordForm.type = 'A'
        // Don't set content, let user fill it
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

    const subdomain = extractSubdomain(props.serverName, domain.domain)

    const record = await dnsStore.createRecord(selectedDomainId.value, {
      type: newRecordForm.type,
      name: subdomain,
      content: newRecordForm.content,
      ttl: newRecordForm.ttl,
      proxied: newRecordForm.proxied,
    })

    message.success($gettext('DNS record created successfully'))
    emit('recordCreated', record, domain)

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
  selectedRecordIds.value = []
  createNewRecord.value = false
  availableRecords.value = []
  emit('cleared')
}

defineExpose({
  clearSelection,
})
</script>

<template>
  <ACard :title="$gettext('DNS Record Integration (Optional)')">
    <p class="text-gray-600 mb-4">
      {{ $gettext('Link this site to a DNS record. The server_name will be used for the DNS record name. You can skip this step if DNS is already configured.') }}
    </p>

    <AForm layout="vertical">
      <AFormItem :label="$gettext('DNS Domain')">
        <ASelect
          v-model:value="selectedDomainValue"
          :options="domainOptions"
          :placeholder="$gettext('Select DNS domain')"
          :loading="loading"
          allow-clear
          @change="onDomainChange"
        >
          <template #optionRender="{ option }">
            {{ option.data.domainName }}
            <span v-if="option.data.hasCredential" class="text-gray-400">
              ({{ option.data.credentialName }})
            </span>
          </template>
          <template #labelRender="{ label, value }">
            <template v-if="findDomainById(value)">
              {{ label }}
              <span v-if="findDomainById(value)?.dns_credential" class="text-gray-400">
                ({{ findDomainById(value)?.dns_credential?.name }})
              </span>
            </template>
            <template v-else>
              {{ label ?? value }}
            </template>
          </template>
        </ASelect>
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
            allow-clear
            :options="recordOptions"
            @change="onRecordSelect"
          >
            <template #optionRender="{ option }">
              <ATag :color="option.data.recordType === 'A' ? 'blue' : option.data.recordType === 'AAAA' ? 'green' : 'orange'">
                {{ option.data.recordType }}
              </ATag>
              {{ option.data.recordName === '@' ? findDomainById(selectedDomainId)?.domain : option.data.recordName }}
              → {{ option.data.recordContent }}
              <ATag v-if="option.data.isProxied" color="orange" class="ml-2">
                {{ $gettext('Proxied') }}
              </ATag>
            </template>
            <template #labelRender="{ label, value }">
              <template v-if="findRecordById(value)">
                <ATag :color="findRecordById(value)?.type === 'A' ? 'blue' : findRecordById(value)?.type === 'AAAA' ? 'green' : 'orange'">
                  {{ findRecordById(value)?.type }}
                </ATag>
                {{ findRecordById(value)?.name === '@' ? findDomainById(selectedDomainId)?.domain : findRecordById(value)?.name }}
                → {{ findRecordById(value)?.content }}
                <ATag v-if="findRecordById(value)?.proxied" color="orange" class="ml-2">
                  {{ $gettext('Proxied') }}
                </ATag>
              </template>
              <template v-else>
                {{ label ?? value }}
              </template>
            </template>
          </ASelect>

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
            :value="extractSubdomain(serverName, availableDomains.find(d => d.id === selectedDomainId)?.domain || '')"
            disabled
          />
          <div class="text-gray-500 text-sm mt-1">
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
          <div class="text-gray-500 text-sm mt-1">
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
  </ACard>
</template>

<style scoped lang="less">
</style>
