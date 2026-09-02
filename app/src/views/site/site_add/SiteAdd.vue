<script setup lang="ts">
import type { DNSDomain, DNSRecord } from '@/api/dns'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import ngx from '@/api/ngx'
import site from '@/api/site'
import NgxConfigEditor, { DirectiveEditor, LocationEditor, useNgxConfigStore } from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'
import QuickSetupForm from '../components/QuickSetup/QuickSetupForm.vue'
import { useQuickConfig } from '../components/QuickSetup/useQuickConfig'
import Cert from '../site_edit/components/Cert'
import EnableTLS from '../site_edit/components/EnableTLS'
import { useSiteEditorStore } from '../site_edit/components/SiteEditor/store'
import DNSRecordIntegration from './components/DNSRecordIntegration.vue'

const currentStep = ref(0)
const { message } = useGlobalApp()

// Quick setup mode
const currentMode = ref<'quick' | 'advanced'>('quick')
const quickMode = computed(() => currentMode.value === 'quick')
const quick = useQuickConfig()
const { quickGenerating, quickFormValid } = quick

// DNS record integration state
const selectedDNSRecords = ref<{ records: DNSRecord[], domain: DNSDomain } | null>(null)
const selectedDNSRecordNames = computed(() => {
  if (!selectedDNSRecords.value)
    return ''
  return selectedDNSRecords.value.records
    .map(record => getFullDNSName(record, selectedDNSRecords.value!.domain))
    .join(', ')
})

onMounted(() => {
  init()
})

const ngxConfigStore = useNgxConfigStore()
const editorStore = useSiteEditorStore()
const { ngxConfig, curServerDirectives, curServerLocations } = storeToRefs(ngxConfigStore)
const { curSupportSSL } = storeToRefs(editorStore)

function init() {
  currentStep.value = 0
  selectedDNSRecords.value = null
  ngxConfigStore.reset()

  site.get_default_template().then(r => {
    ngxConfigStore.setNgxConfig(r.tokenized)
  })
}

const quickTLSMissingCert = computed(() => {
  if (!quickMode.value || quick.state.type === 'redirect')
    return false
  return editorStore.getTLSServerIssues().length > 0
})

async function next() {
  if (quickMode.value && currentStep.value === 0) {
    const r = await quick.generate()
    ngxConfigStore.setNgxConfig(r.tokenized)
    ngxConfig.value.name = quick.state.name.trim()
    // Select the TLS server so the certificate flow targets the 443 block.
    if (r.tokenized.servers.length > 1)
      ngxConfigStore.curServerIdx = 1
  }
  // Block leaving the SSL step until a certificate is issued for the TLS server.
  if (currentStep.value === 2 && quickTLSMissingCert.value) {
    message.warning($gettext('Issue a certificate to enable TLS before continuing.'))
    return
  }
  // Only save on the final step (step 2 -> step 3)
  if (currentStep.value === 2) {
    await save()
  }
  currentStep.value++
}

function onModeChange(mode: string | number) {
  currentMode.value = mode as 'quick' | 'advanced'
  selectedDNSRecords.value = null

  if (currentStep.value === 0)
    init()
}

async function save() {
  const r = await ngx.build_config(ngxConfig.value)

  const payload: Record<string, unknown> = {
    name: ngxConfig.value.name,
    content: r.content,
    overwrite: true, // Always overwrite to avoid conflicts during multi-step process
  }

  // Include DNS information if a record was selected/created in step 1
  if (selectedDNSRecords.value) {
    payload.dns_domain_id = selectedDNSRecords.value.domain.id
    payload.dns_records = selectedDNSRecords.value.records.map(record => ({
      id: record.id,
      name: record.name,
      type: record.type,
      exists: true,
    }))
  }

  await site.updateItem(ngxConfig.value.name, payload)

  message.success($gettext('Saved successfully'))

  await site.enable(ngxConfig.value.name)
  message.success($gettext('Enabled successfully'))

  window.scroll({ top: 0, left: 0, behavior: 'smooth' })
}

const router = useRouter()

function gotoModify() {
  router.push(`/sites/${ngxConfig.value.name}`)
}

function createAnother() {
  router.go(0)
}

const hasServerName = computed(() => {
  const servers = ngxConfig.value.servers

  for (const server of Object.values(servers) as NgxServer[]) {
    if (!server.directives)
      continue

    for (const directive of Object.values(server.directives) as NgxDirective[]) {
      if (directive.directive === 'server_name' && directive.params.trim() !== '')
        return true
    }
  }

  return false
})

// Get server_name value for DNS integration
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

// Update server_name directive with DNS name
function updateServerNameWithDNS(dnsNames: string[]) {
  const servers = ngxConfig.value.servers

  for (const server of Object.values(servers) as NgxServer[]) {
    if (!server.directives)
      continue

    for (const directive of Object.values(server.directives) as NgxDirective[]) {
      if (directive.directive === 'server_name') {
        directive.params = dnsNames.join(' ')
        break
      }
    }
  }
}

// Get full DNS name (record.domain)
function getFullDNSName(record: DNSRecord, domain: DNSDomain): string {
  if (record.name === '@' || record.name === domain.domain) {
    return domain.domain
  }
  return `${record.name}.${domain.domain}`
}

// Handle DNS record selection
function onDNSRecordsSelected(records: DNSRecord[], domain: DNSDomain) {
  selectedDNSRecords.value = { records, domain }
  const fullDNSNames = records.map(record => getFullDNSName(record, domain))
  updateServerNameWithDNS(fullDNSNames)
  message.info($gettext('DNS record selected: %{name}').replace('%{name}', fullDNSNames.join(', ')))
}

// Handle DNS record creation
function onDNSRecordCreated(record: DNSRecord, domain: DNSDomain) {
  selectedDNSRecords.value = { records: [record], domain }
  const fullDNSName = getFullDNSName(record, domain)
  updateServerNameWithDNS([fullDNSName])
  message.success($gettext('DNS record created and linked successfully'))
}

// Handle DNS record cleared
function onDNSRecordCleared() {
  selectedDNSRecords.value = null
}
</script>

<template>
  <ACard :title="$gettext('Add Site')">
    <div class="domain-add-container">
      <ASegmented
        :value="currentMode"
        :options="[
          { label: $gettext('Quick Setup'), value: 'quick' },
          { label: $gettext('Advanced'), value: 'advanced' },
        ]"
        class="mb-6"
        block
        @change="onModeChange"
      />

      <ASteps
        :current="currentStep"
        size="small"
      >
        <AStep :title="$gettext('Base information')" />
        <AStep :title="$gettext('DNS Record')" />
        <AStep :title="$gettext('Configure SSL')" />
        <AStep :title="$gettext('Finished')" />
      </ASteps>

      <div v-if="currentStep === 0" class="mb-6">
        <QuickSetupForm
          v-if="quickMode"
          :quick="quick"
        />

        <template v-else>
          <AForm layout="vertical">
            <AFormItem :label="$gettext('Configuration Name')">
              <AInput v-model:value="ngxConfig.name" />
            </AFormItem>
          </AForm>

          <AAlert
            v-if="!hasServerName"
            type="warning"
            class="mb-4"
            show-icon
            :message="$gettext('The parameter of server_name is required')"
          />

          <DirectiveEditor
            v-model:directives="curServerDirectives"
            class="mb-4"
          />
          <LocationEditor
            v-model:locations="curServerLocations"
            :current-server-index="0"
          />
        </template>
      </div>

      <!-- DNS Record Integration Step -->
      <div v-else-if="currentStep === 1" class="mb-6">
        <DNSRecordIntegration
          v-if="hasServerName"
          :server-name="serverNameValue"
          @record-created="onDNSRecordCreated"
          @records-selected="onDNSRecordsSelected"
          @cleared="onDNSRecordCleared"
        />
      </div>

      <template v-else-if="currentStep === 2">
        <AAlert
          v-if="quickTLSMissingCert"
          type="warning"
          class="mb-4"
          show-icon
          :message="$gettext('Issue a certificate to enable TLS before continuing.')"
        />

        <EnableTLS />

        <NgxConfigEditor>
          <template v-if="curSupportSSL" #tab-content>
            <Cert
              class="mb-4"
              :site-status="ConfigStatus.Enabled"
              :config-name="ngxConfig.name"
            />
          </template>
        </NgxConfigEditor>

        <br>
      </template>

      <ASpace v-if="currentStep < 3">
        <AButton
          v-if="currentStep === 0"
          type="primary"
          :disabled="quickMode ? !quickFormValid : !ngxConfig.name || !hasServerName"
          :loading="quickMode && quickGenerating"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
        <AButton
          v-else
          type="primary"
          :disabled="currentStep === 2 && quickTLSMissingCert"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
        <AButton
          v-if="currentStep === 1"
          @click="currentStep--"
        >
          {{ $gettext('Back') }}
        </AButton>
      </ASpace>
      <AResult
        v-else-if="currentStep === 3"
        status="success"
        :title="$gettext('Site Config Created Successfully')"
        :sub-title="selectedDNSRecordNames ? $gettext('DNS record has been linked: %{name}').replace('%{name}', selectedDNSRecordNames) : undefined"
      >
        <template #extra>
          <AButton
            type="primary"
            @click="gotoModify"
          >
            {{ $gettext('Modify Config') }}
          </AButton>
          <AButton @click="createAnother">
            {{ $gettext('Create Another') }}
          </AButton>
        </template>
      </AResult>
    </div>
  </ACard>
</template>

<style lang="less" scoped>
.ant-steps {
  padding: 10px 0 20px 0;
}

.domain-add-container {
  max-width: 800px;
  margin: 0 auto
}
</style>
