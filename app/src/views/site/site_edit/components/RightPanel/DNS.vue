<script setup lang="ts">
import type { DNSDomain, DNSRecord } from '@/api/dns'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import { isAllowedDnsProvider } from '@/constants/dns_providers'
import { useDnsStore } from '@/pinia/moudule/dns'
import { useSiteEditorStore } from '../SiteEditor/store'

const { message } = useGlobalApp()
const dnsStore = useDnsStore()
const editorStore = useSiteEditorStore()
const { ngxConfig, dnsLinked, linkedDNSName, data } = storeToRefs(editorStore)

const selectedDomainId = ref<number | null>(null)
const selectedRecordId = ref<string | null>(null)
const createNewRecord = ref(false)
const loading = ref(false)
const initialLoading = ref(true) // Loading state for initial DNS link check
const availableDomains = ref<DNSDomain[]>([])
const availableRecords = ref<DNSRecord[]>([])
const linkedRecord = ref<{ record: DNSRecord, domain: DNSDomain } | null>(null)
const newRecordForm = reactive({
  type: 'A',
  content: '',
  ttl: 600,
  proxied: false,
})

const recordTypes = ['A', 'AAAA', 'CNAME']

// Computed properties for v-model bindings to handle null values
const selectedDomainValue = computed({
  get: () => selectedDomainId.value ?? undefined,
  set: val => {
    selectedDomainId.value = typeof val === 'number' ? val : null
  },
})

const selectedRecordValue = computed({
  get: () => selectedRecordId.value ?? undefined,
  set: val => {
    selectedRecordId.value = typeof val === 'string' ? val : null
  },
})

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
function updateServerNameDirective(dnsName: string) {
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
        serverNameDirective.params = dnsName
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

// Load available DNS domains on mount
onMounted(async () => {
  try {
    initialLoading.value = true
    await loadDomains()

    // Load existing DNS link if present
    if (data.value.dns_domain_id && data.value.dns_record_id) {
      selectedDomainId.value = data.value.dns_domain_id
      await loadRecordsForDomain(data.value.dns_domain_id)

      // Try to find the linked record
      const record = availableRecords.value.find(r => r.id === data.value.dns_record_id)
      const domain = availableDomains.value.find(d => d.id === data.value.dns_domain_id)

      if (record && domain) {
        selectedRecordId.value = data.value.dns_record_id
        linkedRecord.value = { record, domain }
        dnsLinked.value = true
        linkedDNSName.value = getFullDNSName(record, domain)
      }
      else if (domain) {
        // Record doesn't exist anymore, but we have the cached info
        linkedRecord.value = {
          record: {
            id: data.value.dns_record_id!,
            name: data.value.dns_record_name || '',
            type: data.value.dns_record_type || 'A',
            content: '',
            ttl: 600,
          },
          domain,
        }
        dnsLinked.value = true
        const recordName = data.value.dns_record_name
        if (recordName === '@') {
          linkedDNSName.value = domain.domain
        }
        else if (recordName) {
          linkedDNSName.value = `${recordName}.${domain.domain}`
        }
        else {
          linkedDNSName.value = domain.domain
        }
      }
    }
    else {
      // Try to auto-match domain from server_name
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
  selectedRecordId.value = null
  createNewRecord.value = false
  linkedRecord.value = null
  if (domainId) {
    await loadRecordsForDomain(domainId)
  }
  else {
    availableRecords.value = []
  }
}

// Save DNS link to backend
async function saveDNSLink(domainId: number, recordId: string, recordName: string, recordType: string) {
  data.value.dns_domain_id = domainId
  data.value.dns_record_id = recordId
  data.value.dns_record_name = recordName
  data.value.dns_record_type = recordType
  data.value.dns_record_exists = true
}

// Handle record selection
function onRecordSelect(value: unknown) {
  const recordId = typeof value === 'string' ? value : null
  createNewRecord.value = false
  if (recordId && selectedDomainId.value) {
    const record = availableRecords.value.find(r => r.id === recordId)
    const domain = availableDomains.value.find(d => d.id === selectedDomainId.value)
    if (record && domain) {
      linkedRecord.value = { record, domain }

      // Update server_name with DNS name
      const dnsName = getFullDNSName(record, domain)
      updateServerNameDirective(dnsName)

      // Update store state
      dnsLinked.value = true
      linkedDNSName.value = dnsName

      // Save DNS link to backend
      saveDNSLink(domain.id, record.id, record.name, record.type)

      message.success($gettext('DNS record linked and server_name updated: %{name}').replace('%{name}', dnsName))
    }
  }
  else {
    linkedRecord.value = null
    dnsLinked.value = false
    linkedDNSName.value = ''
  }
}

// Handle create new record toggle
function onCreateNewToggle(e: { target: { checked: boolean } }) {
  const checked = e.target.checked
  if (checked) {
    selectedRecordId.value = null
    linkedRecord.value = null
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
    linkedRecord.value = { record, domain }

    // Update server_name with DNS name
    const dnsName = getFullDNSName(record, domain)
    updateServerNameDirective(dnsName)

    // Update store state
    dnsLinked.value = true
    linkedDNSName.value = dnsName

    // Save DNS link to backend
    saveDNSLink(domain.id, record.id, record.name, record.type)

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
  linkedRecord.value = null
  dnsLinked.value = false
  linkedDNSName.value = ''

  // Clear DNS link in backend
  data.value.dns_domain_id = null
  data.value.dns_record_id = null
  data.value.dns_record_name = null
  data.value.dns_record_type = null
  data.value.dns_record_exists = null
}

// Recreate missing DNS record
async function recreateRecord() {
  if (!linkedRecord.value || !selectedDomainId.value)
    return

  try {
    loading.value = true
    const { record, domain } = linkedRecord.value

    const newRecord = await dnsStore.createRecord(selectedDomainId.value, {
      type: record.type,
      name: record.name,
      content: newRecordForm.content || '', // User should fill this
      ttl: newRecordForm.ttl,
      proxied: newRecordForm.proxied,
    })

    message.success($gettext('DNS record recreated successfully'))

    // Update linked record
    linkedRecord.value = { record: newRecord, domain }
    selectedRecordId.value = newRecord.id

    // Save new link
    await saveDNSLink(domain.id, newRecord.id, newRecord.name, newRecord.type)

    // Reload records
    await loadRecordsForDomain(selectedDomainId.value)
    data.value.dns_record_exists = true

    // Update server_name with DNS name
    const dnsName = getFullDNSName(newRecord, domain)
    updateServerNameDirective(dnsName)

    // Update store state
    dnsLinked.value = true
    linkedDNSName.value = dnsName

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
      :message="$gettext('The parameter of server_name is required')"
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

      <!-- Current linked record -->
      <div v-if="linkedRecord" class="mb-4">
        <!-- Record exists -->
        <div v-if="data.dns_record_exists !== false" class="p-3 border border-green-200 rounded">
          <div class="flex items-center justify-between">
            <div>
              <div class="text-sm font-medium text-green-800 mb-1">
                {{ $gettext('Linked DNS Record') }}
              </div>
              <div class="text-xs text-gray-600">
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
            <AButton size="small" @click="clearSelection">
              {{ $gettext('Clear') }}
            </AButton>
          </div>
        </div>

        <!-- Record doesn't exist -->
        <div v-else class="p-3 border border-orange-200 rounded">
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

          <!-- Recreate form -->
          <AForm layout="vertical" size="small">
            <AFormItem :label="$gettext('IP Address / Target')" required>
              <AInput
                v-model:value="newRecordForm.content"
                size="small"
                :placeholder="linkedRecord.record.type === 'CNAME' ? $gettext('target.example.com') : $gettext('192.168.1.1')"
              />
            </AFormItem>
            <AFormItem :label="$gettext('TTL (seconds)')">
              <AInputNumber
                v-model:value="newRecordForm.ttl"
                size="small"
                :min="60"
                :max="86400"
                style="width: 100%"
              />
            </AFormItem>
            <AFormItem>
              <ACheckbox v-model:checked="newRecordForm.proxied">
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
              @click="recreateRecord"
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
              v-model:value="selectedRecordValue"
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
          :message="$gettext('No DNS domains available')"
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
