<script setup lang="ts">
import type { WizardMode } from './wizardSteps'
import type { DNSDomain, DNSRecord } from '@/api/dns'
import type { NgxConfig, NgxDirective, NgxServer } from '@/api/ngx'
import { StdSelector } from '@uozi-admin/curd'
import namespace from '@/api/namespace'
import ngx from '@/api/ngx'
import site from '@/api/site'
import NgxConfigEditor, { DirectiveEditor, LocationEditor, useNgxConfigStore } from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'
import namespaceColumns from '@/views/namespace/columns'
import QuickSetupForm from '../components/QuickSetup/QuickSetupForm.vue'
import { useQuickConfig } from '../components/QuickSetup/useQuickConfig'
import ConfigTemplate from '../site_edit/components/ConfigTemplate'
import { extractSiteDomains, HTTPSCard, sameStringList, serializeNgxConfig } from '../site_edit/components/HTTPS'
import { useSiteEditorStore } from '../site_edit/components/SiteEditor/store'
import DNSRecordIntegration from './components/DNSRecordIntegration.vue'
import { staleDraftAction, staleDraftName } from './draftCleanup'
import { defaultEditorPanelKeys, EDITOR_PANEL_KEY, isModeLocked, SSL_STEP, sslStepFinishLabel, sslStepTLSState } from './wizardSteps'

const currentStep = ref(0)
const { message } = useGlobalApp()

// Quick setup mode
const currentMode = ref<WizardMode>('quick')
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

// Configure SSL step state, reset by init().
const createdSiteName = ref('')
// Canonical form of the config written by the last draft save.
const draftSnapshot = ref<string>()
const editorKeys = ref<string[]>([])
// Namespace chosen on the first step. Like the draft name it survives a mode
// switch, since it does not depend on how the configuration is built.
const namespaceId = ref<number>()
// Name of the disabled draft this wizard last wrote. Unlike the step state it
// survives init() on a mode switch, so a renamed site can still clean it up.
const draftName = ref('')

onMounted(() => {
  init()
})

const ngxConfigStore = useNgxConfigStore()
const editorStore = useSiteEditorStore()
const { ngxConfig, curServerDirectives, curServerLocations } = storeToRefs(ngxConfigStore)
const route = useRoute()
const router = useRouter()

function init() {
  currentStep.value = 0
  selectedDNSRecords.value = null
  createdSiteName.value = ''
  draftSnapshot.value = undefined
  editorKeys.value = []
  // The site editor store is a singleton: without a reset, autoCert and the
  // certificate state of the last opened site would leak into this new one.
  editorStore.reset()
  // A server tab remembered in the URL (e.g. after "Create Another" reloads
  // the page) would override the TLS server the SSL step selects.
  if (route.query.server_idx !== undefined)
    router.replace({ query: {} })

  site.get_default_template().then(r => {
    ngxConfigStore.setNgxConfig(r.tokenized)
  })
}

// ---- Configure SSL step --------------------------------------------------
//
// The step is the same for both modes, with or without a TLS server: entering
// it saves the site as a draft (not enabled), keeping a TLS server that still
// waits for its certificate. The HTTPS card is the only entry point: it lets
// the backend stage, verify, issue (or use an existing certificate) and switch
// the draft to HTTPS in one run, generating the TLS server when there is none,
// so the wizard never writes the file again after a successful run. "Skip for
// now" saves and enables the site without HTTPS.

const tlsState = computed(() => sslStepTLSState(ngxConfig.value))
const hasPendingTLS = computed(() => tlsState.value === 'pending')

// Keep the same array while the names are unchanged so the card does not
// reset domains the operator added.
const siteDomains = computed<string[]>(previous => {
  const next = extractSiteDomains(ngxConfig.value)
  return previous && sameStringList(previous, next) ? previous : next
})

const draftSaving = ref(false)
const finishing = ref(false)

const draftSaved = computed(() => draftSnapshot.value !== undefined)
const draftDirty = computed(() => draftSaved.value && serializeNgxConfig(ngxConfig.value) !== draftSnapshot.value)
// The backend reads the saved file, so the card waits for an up-to-date draft.
const httpsBlocked = computed(() => draftSaving.value || !draftSaved.value || draftDirty.value)

const httpsCard = useTemplateRef('httpsCard')
const httpsRunning = computed(() => httpsCard.value?.running ?? false)
const finishLabel = computed(() => sslStepFinishLabel(httpsCard.value?.method, httpsCard.value?.phase))
const canFinish = computed(() => httpsCard.value?.canConfirm ?? false)

// The Finish button runs what the HTTPS card is set to: the HTTPS setup, or
// "Skip for now" which saves and enables the site.
function confirmSSLStep() {
  httpsCard.value?.confirm()
}

const editorItems = computed(() => [{ key: EDITOR_PANEL_KEY, label: $gettext('Edit configuration file') }])

// Skipping keeps a TLS server that already has its certificate, so it does not
// mean "without HTTPS" there.
const skipDescription = computed(() => tlsState.value === 'configured'
  ? $gettext('Save and enable the site with the HTTPS server block it already has.')
  : undefined)

async function writeSite(config: NgxConfig) {
  const r = await ngx.build_config(config)

  const payload: Record<string, unknown> = {
    name: ngxConfig.value.name,
    content: r.content,
    namespace_id: namespaceId.value ?? 0,
    overwrite: true, // Always overwrite to avoid conflicts during multi-step process
    // Nginx is only tested and reloaded when the site is enabled.
    post_action: 'reload_nginx',
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

  return site.updateItem(ngxConfig.value.name, payload)
}

async function isSiteEnabled(name: string) {
  try {
    const r = await site.getItem(encodeURIComponent(name))
    return r.status !== ConfigStatus.Disabled
  }
  catch {
    return false
  }
}

// Removes the draft left under the old name after the operator went back and
// renamed the site. Call it once the file under the new name is written.
async function discardStaleDraft(staleName: string | undefined) {
  if (!staleName)
    return

  let status: string | undefined
  try {
    status = (await site.getItem(encodeURIComponent(staleName), undefined, { skipErrHandling: true })).status
  }
  catch {
    // Already gone, or it cannot be checked; never delete what was not verified.
    return
  }

  if (staleDraftAction(status) === 'keep') {
    message.warning($gettext('The previous draft %{name} is enabled, so it was kept. Remove it from the site list if you no longer need it.', { name: staleName }))
    return
  }

  try {
    await site.deleteItem(encodeURIComponent(staleName), undefined, { skipErrHandling: true })
    message.info($gettext('Removed the previous draft %{name}', { name: staleName }))
  }
  catch {
    message.warning($gettext('The previous draft %{name} could not be removed. Remove it from the site list if you no longer need it.', { name: staleName }))
  }
}

// The in-flight draft save, so finishing the wizard can wait for it.
let draftRequest: Promise<void> | undefined

function saveDraft(): Promise<void> {
  draftRequest ??= writeDraft().finally(() => {
    draftRequest = undefined
  })
  return draftRequest
}

async function writeDraft() {
  const name = ngxConfig.value.name
  if (!name)
    return

  draftSaving.value = true
  try {
    const snapshot = serializeNgxConfig(ngxConfig.value)
    const staleName = staleDraftName(draftName.value, name)
    // A failed HTTPS run leaves the site enabled with its HTTP-only staged
    // config. Nginx rejects a TLS server without certificate there, so the
    // pending one stays out; the backend builds it again from the HTTP server.
    const enabled = draftSaved.value && hasPendingTLS.value && await isSiteEnabled(name)
    const config = enabled ? editorStore.getConfigWithoutIncompleteTLSServers(ngxConfig.value) : ngxConfig.value

    const r = await writeSite(config)
    // Later editor saves (e.g. Enable TLS) reuse the DNS link and namespace.
    editorStore.data = r
    createdSiteName.value = name
    draftName.value = name
    draftSnapshot.value = snapshot
    await discardStaleDraft(staleName)
  }
  catch {
    // The request layer reports the error; the step offers another save.
  }
  finally {
    draftSaving.value = false
  }
}

// The backend reads the saved file, so every site is saved as a draft on
// entering the step, with or without a TLS server.
watch(currentStep, step => {
  if (step !== SSL_STEP)
    return

  editorKeys.value = defaultEditorPanelKeys(currentMode.value)
  saveDraft()
})

function finish(name: string) {
  createdSiteName.value = name
  currentStep.value = 3
  window.scroll({ top: 0, left: 0, behavior: 'smooth' })
}

// "Skip for now": plain HTTP sites, or sites whose TLS servers all have a certificate.
async function saveAndEnable() {
  if (finishing.value)
    return

  finishing.value = true
  try {
    // A draft save still running would otherwise race this write.
    await draftRequest
    const name = ngxConfig.value.name
    // A TLS server without certificate would fail nginx -t, so it is left out
    // and a port-80 server that only redirected to it serves the app instead.
    const config = hasPendingTLS.value
      ? editorStore.getConfigWithoutIncompleteTLSServers(ngxConfig.value)
      : ngxConfig.value
    const staleName = staleDraftName(draftName.value, name)

    await writeSite(config)
    message.success($gettext('Saved successfully'))
    draftName.value = name
    await discardStaleDraft(staleName)

    await site.enable(name)
    message.success($gettext('Enabled successfully'))

    finish(name)
  }
  catch {
    // The request layer reports the error.
  }
  finally {
    finishing.value = false
  }
}

async function onHTTPSSuccess() {
  const name = createdSiteName.value || ngxConfig.value.name
  message.success($gettext('HTTPS is enabled'))
  finish(name)
  // The backend wrote the final config and enabled the site; load it instead
  // of keeping the stale draft in memory.
  await editorStore.init(name)
}

async function next() {
  if (quickMode.value && currentStep.value === 0) {
    const r = await quick.generate()
    ngxConfigStore.setNgxConfig(r.tokenized)
    ngxConfig.value.name = quick.state.name.trim()
    // Open the TLS server tab for anyone expanding the editor.
    if (r.tokenized.servers.length > 1)
      ngxConfigStore.curServerIdx = 1
  }

  currentStep.value++
}

function onModeChange(mode: string | number) {
  if (isModeLocked(currentStep.value))
    return

  currentMode.value = mode as WizardMode
  selectedDNSRecords.value = null
  init()
}

function gotoModify() {
  router.push(`/sites/${encodeURIComponent(createdSiteName.value || ngxConfig.value.name)}`)
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
    <div
      class="domain-add-container"
      :class="{ 'advanced-template-layout': currentStep === 0 && !quickMode }"
    >
      <!-- Locked after the first step, but still shows the mode in use. -->
      <ASegmented
        :value="currentMode"
        :options="[
          { label: $gettext('Quick Setup'), value: 'quick' },
          { label: $gettext('Advanced'), value: 'advanced' },
        ]"
        :disabled="isModeLocked(currentStep)"
        class="mb-6"
        block
        @change="onModeChange"
      />

      <ASteps
        :current="currentStep"
        size="small"
        :items="[
          { title: $gettext('Base information') },
          { title: $gettext('DNS Record') },
          { title: $gettext('Configure SSL') },
          { title: $gettext('Finished') },
        ]"
      />

      <div v-if="currentStep === 0" class="mb-6">
        <QuickSetupForm
          v-if="quickMode"
          :quick="quick"
        >
          <template #afterName>
            <AFormItem :label="$gettext('Namespace')">
              <StdSelector
                v-model:value="namespaceId"
                :get-list-api="namespace.getList"
                :columns="namespaceColumns"
                display-key="name"
                selection-type="radio"
              />
            </AFormItem>
          </template>
        </QuickSetupForm>

        <template v-else>
          <div class="advanced-config-layout">
            <div class="advanced-config-main">
              <AForm layout="vertical">
                <AFormItem :label="$gettext('Configuration Name')">
                  <AInput v-model:value="ngxConfig.name" />
                </AFormItem>
                <AFormItem :label="$gettext('Namespace')">
                  <StdSelector
                    v-model:value="namespaceId"
                    :get-list-api="namespace.getList"
                    :columns="namespaceColumns"
                    display-key="name"
                    selection-type="radio"
                  />
                </AFormItem>
              </AForm>

              <AAlert
                v-if="!hasServerName"
                type="warning"
                class="mb-4"
                show-icon
                :title="$gettext('The parameter of server_name is required')"
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

            <ACard
              class="advanced-config-template"
              :title="$gettext('Config Template')"
              :styles="{ body: { padding: '16px' } }"
            >
              <ConfigTemplate />
            </ACard>
          </div>
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

      <ASpin v-else-if="currentStep === SSL_STEP" :spinning="finishing">
        <AAlert
          v-if="httpsBlocked && !draftSaving && !httpsRunning"
          type="warning"
          class="mb-4"
          show-icon
          :title="draftSaved
            ? $gettext('Save the edited configuration before enabling HTTPS.')
            : $gettext('The site draft is not saved yet.')"
        >
          <template #description>
            <div class="ssl-step-guidance">
              <span>{{ $gettext('HTTPS setup reads the saved draft of this site.') }}</span>
              <AButton type="primary" size="small" :loading="draftSaving" @click="saveDraft">
                {{ $gettext('Save draft') }}
              </AButton>
            </div>
          </template>
        </AAlert>

        <!-- The single entry point for HTTPS in both modes, with or without a
             TLS server in the configuration. -->
        <HTTPSCard
          ref="httpsCard"
          class="mb-4"
          :config-name="ngxConfig.name"
          :domains="siteDomains"
          :has-pending-t-l-s-server="hasPendingTLS"
          :disabled="httpsBlocked"
          :skip-description
          external-confirm
          @success="onHTTPSSuccess"
          @skip="saveAndEnable"
        />

        <ACollapse
          v-model:active-key="editorKeys"
          class="mb-6"
          :items="editorItems"
        >
          <template #contentRender>
            <NgxConfigEditor />
          </template>
        </ACollapse>
      </ASpin>

      <!-- Back on the left, the step's primary action on the right. -->
      <AFlex v-if="currentStep < 3" justify="space-between" align="center" gap="small" wrap>
        <AButton
          v-if="currentStep > 0"
          :disabled="finishing || httpsRunning"
          @click="currentStep--"
        >
          {{ $gettext('Back') }}
        </AButton>
        <span v-else />
        <AButton
          v-if="currentStep === 0"
          type="primary"
          :disabled="quickMode ? !quickFormValid : !ngxConfig.name || !hasServerName"
          :loading="quickMode && quickGenerating"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
        <!-- Runs what the HTTPS card is set to and finishes the wizard. -->
        <AButton
          v-else-if="currentStep === SSL_STEP"
          type="primary"
          :loading="httpsRunning || finishing"
          :disabled="!canFinish"
          data-testid="site-add-finish"
          @click="confirmSSLStep"
        >
          {{ finishLabel }}
        </AButton>
        <AButton
          v-else
          type="primary"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
      </AFlex>
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

.domain-add-container.advanced-template-layout {
  max-width: 1280px;
}

.advanced-config-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  align-items: start;
  gap: 24px;
}

.advanced-config-main {
  min-width: 0;
}

.advanced-config-template {
  position: sticky;
  top: 16px;
  min-width: 0;
}

.ssl-step-guidance {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 900px) {
  .advanced-config-layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .advanced-config-template {
    position: static;
  }
}
</style>
