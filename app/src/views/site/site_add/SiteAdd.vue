<script setup lang="ts">
import type { DNSDomain, DNSRecord } from '@/api/dns'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import ngx from '@/api/ngx'
import site from '@/api/site'
import NgxConfigEditor, { DirectiveEditor, LocationEditor, useNgxConfigStore } from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'
import Cert from '../site_edit/components/Cert'
import EnableTLS from '../site_edit/components/EnableTLS'
import { useSiteEditorStore } from '../site_edit/components/SiteEditor/store'
import DNSRecordIntegration from './components/DNSRecordIntegration.vue'

const currentStep = ref(0)
const { message } = useGlobalApp()

// DNS record integration state
const selectedDNSRecord = ref<{ record: DNSRecord, domain: DNSDomain } | null>(null)

onMounted(() => {
  init()
})

const ngxConfigStore = useNgxConfigStore()
const editorStore = useSiteEditorStore()
const { ngxConfig, curServerDirectives, curServerLocations } = storeToRefs(ngxConfigStore)
const { curSupportSSL } = storeToRefs(editorStore)

function init() {
  site.get_default_template().then(r => {
    ngxConfig.value = r.tokenized
  })
}

async function save() {
  const r = await ngx.build_config(ngxConfig.value)

  const payload: Record<string, unknown> = {
    name: ngxConfig.value.name,
    content: r.content,
    overwrite: true, // Always overwrite to avoid conflicts during multi-step process
  }

  // Include DNS information if a record was selected/created in step 1
  if (selectedDNSRecord.value) {
    payload.dns_domain_id = selectedDNSRecord.value.domain.id
    payload.dns_record_id = selectedDNSRecord.value.record.id
    payload.dns_record_name = selectedDNSRecord.value.record.name
    payload.dns_record_type = selectedDNSRecord.value.record.type
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
function updateServerNameWithDNS(dnsName: string) {
  const servers = ngxConfig.value.servers

  for (const server of Object.values(servers) as NgxServer[]) {
    if (!server.directives)
      continue

    for (const directive of Object.values(server.directives) as NgxDirective[]) {
      if (directive.directive === 'server_name') {
        directive.params = dnsName
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
function onDNSRecordSelected(record: DNSRecord, domain: DNSDomain) {
  selectedDNSRecord.value = { record, domain }
  const fullDNSName = getFullDNSName(record, domain)
  updateServerNameWithDNS(fullDNSName)
  message.info($gettext('DNS record selected: %{name}').replace('%{name}', record.name))
}

// Handle DNS record creation
function onDNSRecordCreated(record: DNSRecord, domain: DNSDomain) {
  selectedDNSRecord.value = { record, domain }
  const fullDNSName = getFullDNSName(record, domain)
  updateServerNameWithDNS(fullDNSName)
  message.success($gettext('DNS record created and linked successfully'))
}

// Handle DNS record cleared
function onDNSRecordCleared() {
  selectedDNSRecord.value = null
}

async function next() {
  // Only save on the final step (step 2 -> step 3)
  if (currentStep.value === 2) {
    await save()
  }
  currentStep.value++
}
</script>

<template>
  <ACard :title="$gettext('Add Site')">
    <div class="domain-add-container">
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
      </div>

      <!-- DNS Record Integration Step -->
      <div v-else-if="currentStep === 1" class="mb-6">
        <DNSRecordIntegration
          v-if="hasServerName"
          :server-name="serverNameValue"
          @record-created="onDNSRecordCreated"
          @record-selected="onDNSRecordSelected"
          @cleared="onDNSRecordCleared"
        />
      </div>

      <template v-else-if="currentStep === 2">
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
          :disabled="!ngxConfig.name || !hasServerName"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
        <AButton
          v-else
          type="primary"
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
        :sub-title="selectedDNSRecord ? $gettext('DNS record has been linked: %{name}').replace('%{name}', selectedDNSRecord.record.name) : undefined"
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
