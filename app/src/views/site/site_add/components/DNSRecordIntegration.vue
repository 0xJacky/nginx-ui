<script setup lang="ts">
import type { DNSDomain, DNSRecord } from '@/api/dns'
import { isAllowedDnsProvider } from '@/constants/dns_providers'
import { useDnsStore } from '@/pinia/moudule/dns'

const props = defineProps<{
  serverName: string
}>()

const emit = defineEmits<{
  recordCreated: [record: DNSRecord, domain: DNSDomain]
  recordSelected: [record: DNSRecord, domain: DNSDomain]
  cleared: []
}>()

const { message } = useGlobalApp()
const dnsStore = useDnsStore()

const selectedDomainId = ref<number | null>(null)
const selectedRecordId = ref<string | null>(null)
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
  selectedRecordId.value = null
  createNewRecord.value = false
  if (domainId) {
    loadRecordsForDomain(domainId)
  }
  else {
    availableRecords.value = []
  }
}

// Handle record selection
function onRecordSelect(value: unknown) {
  const recordId = typeof value === 'string' ? value : null
  createNewRecord.value = false
  if (recordId && selectedDomainId.value) {
    const record = availableRecords.value.find(r => r.id === recordId)
    const domain = availableDomains.value.find(d => d.id === selectedDomainId.value)
    if (record && domain) {
      emit('recordSelected', record, domain)
    }
  }
}

// Handle create new record toggle
function onCreateNewToggle(e: { target: { checked: boolean } }) {
  const checked = e.target.checked
  if (checked) {
    selectedRecordId.value = null
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
    selectedRecordId.value = record.id
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
  selectedRecordId.value = null
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
          v-model:value="selectedDomainId"
          :placeholder="$gettext('Select DNS domain')"
          :loading="loading"
          allow-clear
          @change="onDomainChange"
        >
          <ASelectOption
            v-for="domain in availableDomains"
            :key="domain.id"
            :value="domain.id"
          >
            {{ domain.domain }}
            <span v-if="domain.dns_credential" class="text-gray-400">
              ({{ domain.dns_credential.name }})
            </span>
          </ASelectOption>
        </ASelect>
      </AFormItem>

      <AFormItem
        v-if="selectedDomainId"
        :label="$gettext('DNS Record')"
      >
        <ASpace direction="vertical" style="width: 100%">
          <ASelect
            v-model:value="selectedRecordId"
            :placeholder="$gettext('Select existing record')"
            :loading="loading"
            :disabled="createNewRecord"
            allow-clear
            @change="onRecordSelect"
          >
            <ASelectOption
              v-for="record in availableRecords"
              :key="record.id"
              :value="record.id"
            >
              <ATag :color="record.type === 'A' ? 'blue' : record.type === 'AAAA' ? 'green' : 'orange'">
                {{ record.type }}
              </ATag>
              {{ record.name === '@' ? availableDomains.find(d => d.id === selectedDomainId)?.domain : record.name }}
              → {{ record.content }}
              <ATag v-if="record.proxied" color="orange" class="ml-2">
                {{ $gettext('Proxied') }}
              </ATag>
            </ASelectOption>
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
          <ASelect v-model:value="newRecordForm.type">
            <ASelectOption value="A">
              A
            </ASelectOption>
            <ASelectOption value="AAAA">
              AAAA
            </ASelectOption>
            <ASelectOption value="CNAME">
              CNAME
            </ASelectOption>
          </ASelect>
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
        :message="$gettext('No DNS domains available')"
        :description="$gettext('Please add a DNS domain first in the DNS management section.')"
        show-icon
        class="mt-4"
      />
    </AForm>
  </ACard>
</template>

<style scoped lang="less">
</style>
