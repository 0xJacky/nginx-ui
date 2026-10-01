<script setup lang="ts">
import type { SegmentedProps, SelectProps } from 'antdv-next'
import type { HTTPSCardMethod } from './httpsRequest'
import type { HTTPSDelegatedIssue } from './useHTTPSOnboarding'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { Cert } from '@/api/cert'
import type { HTTPSCheckStatus, HTTPSMessageArgs, HTTPSRequest, HTTPSResult, HTTPSStep, HTTPSStepStatus } from '@/api/https'
import type { DnsVerifyOn } from '@/composables/useMainNodeDns01'
import {
  CheckCircleFilled,
  ClockCircleOutlined,
  CloseCircleFilled,
  DownOutlined,
  ExclamationCircleFilled,
  LoadingOutlined,
  MinusCircleOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
} from '@antdv-next/icons'
import { breakpointsAntDesign, useBreakpoints, useLocalStorage, watchDebounced } from '@vueuse/core'
import dayjs from 'dayjs'
import certApi from '@/api/cert'
import DNSChallenge from '@/components/AutoCertForm/DNSChallenge.vue'
import Dns01PluginNotice from '@/components/Dns01PluginNotice'
import PluginSlot from '@/components/PluginSlot'
import { useDns01Plugin } from '@/composables/useDns01Plugin'
import { useMainNodeDns01 } from '@/composables/useMainNodeDns01'
import { PrivateKeyTypeEnum, PrivateKeyTypeList } from '@/constants'
import { isIPAddress, splitCertificateIdentifiers } from '@/utils/certificate'
import ACMEUserSelector from '@/views/certificate/components/ACMEUserSelector.vue'
import CertificatePicker from '../Cert/CertificatePicker.vue'
import { certificateCoverage, certificateNamesOf, isCertificateExpired } from './certificateCoverage'
import { buildHTTPSCheckRequest, buildHTTPSRequest, isChallengeMethod } from './httpsRequest'
import { diagnosticsWithoutHint, HTTPS_MAIN_STEPS, isValidHostname, useHTTPSOnboarding } from './useHTTPSOnboarding'

const props = withDefaults(defineProps<{
  configName: string
  // Initial certificate identifiers, usually the site's server_name values.
  domains: string[]
  // A TLS server block exists but has no certificate yet.
  hasPendingTLSServer?: boolean
  compact?: boolean
  // Blocks "Check only" and the run, e.g. while the caller has unsaved edits
  // to the file the backend is about to rewrite.
  disabled?: boolean
  // Offers the "Skip for now" method, which emits `skip` once confirmed.
  skippable?: boolean
  // Overrides the "Skip for now" description, e.g. when skipping keeps an
  // HTTPS server the configuration already has.
  skipDescription?: string
  // Offers the "Existing certificate" method (a record of the certificate manager).
  existingCertificate?: boolean
  // Leaves the run and skip buttons to the caller (e.g. a wizard's Finish
  // button), which drives them through the exposed `confirm`.
  externalConfirm?: boolean
  // Lets the user fold the card to its title and a one-line summary. The
  // choice is remembered; a run or a finished check always shows the card.
  collapsible?: boolean
}>(), {
  hasPendingTLSServer: false,
  compact: false,
  disabled: false,
  skippable: true,
  existingCertificate: true,
  externalConfirm: false,
  collapsible: false,
})

const emit = defineEmits<{
  success: [result: HTTPSResult]
  skip: []
}>()

const slots = defineSlots<{
  // Extra buttons rendered next to the primary action (e.g. wizard navigation).
  actions?: () => unknown
}>()

type RowStatus = HTTPSStepStatus | HTTPSCheckStatus | 'pending'

interface ChecklistRow {
  key: string
  label: string
  status: RowStatus
  detail?: string
}

const onboarding = useHTTPSOnboarding(() => props.configName)
const { phase, running, steps, logs, diagnostics, result, error, checks, checking } = onboarding

const method = ref<HTTPSCardMethod>('http01')
const redirectHTTPToHTTPS = ref(true)
const certificateOptionKeys = ref<string[]>([])

const options = ref<AutoCertOptions>({
  domains: [],
  challenge_method: 'http01',
  dns_credential_id: undefined,
  key_type: PrivateKeyTypeEnum.P256,
  acme_user_id: undefined,
  profile: '',
  must_staple: false,
  enable_common_name: false,
  revoke_old: false,
})

// ---- Domains ------------------------------------------------------------

function normalizeIdentifier(value: string) {
  return value.trim().replace(/\.$/, '').toLowerCase()
}

const domainList = ref<string[]>([])
const domainInput = ref('')
const domainInputError = ref('')

watch(() => props.domains, value => {
  if (running.value)
    return

  domainList.value = [...new Set(splitCertificateIdentifiers(value ?? []).map(normalizeIdentifier).filter(Boolean))]
}, { immediate: true, deep: true })

function isValidIdentifier(value: string) {
  return isIPAddress(value) || isValidHostname(value, true)
}

function addDomains() {
  const candidates = domainInput.value.split(/[\s,]+/).map(normalizeIdentifier).filter(Boolean)
  if (!candidates.length)
    return

  const invalid = candidates.filter(v => !isValidIdentifier(v))
  if (invalid.length) {
    domainInputError.value = $gettext('Not a valid domain name or IP address: %{value}', { value: invalid.join(', ') })
    return
  }

  domainList.value = [...new Set([...domainList.value, ...candidates])]
  domainInput.value = ''
  domainInputError.value = ''
}

function removeDomain(e: MouseEvent, domain: string) {
  // The list is the source of truth; do not let the tag hide itself.
  e.preventDefault()
  domainList.value = domainList.value.filter(d => d !== domain)
}

watch(domainInput, () => {
  domainInputError.value = ''
})

const hasIPIdentifier = computed(() => domainList.value.some(d => isIPAddress(d)))
const hasWildcardIdentifier = computed(() => domainList.value.some(d => d.startsWith('*.')))

// ---- Method -------------------------------------------------------------

const methodOptions = computed<SegmentedProps['options']>(() => [
  { value: 'http01', label: $gettext('HTTP-01') },
  { value: 'dns01', label: $gettext('DNS-01') },
  ...(props.existingCertificate ? [{ value: 'existing', label: $gettext('Existing certificate') }] : []),
  ...(props.skippable ? [{ value: 'skip', label: $gettext('Skip for now') }] : []),
])

// Four methods do not fit a phone's width side by side; stack them there.
const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('sm')

const methodDescription = computed(() => {
  switch (method.value) {
    case 'http01':
      return $gettext('The certificate authority validates each domain over port 80, so the domains must resolve to this server.')
    case 'dns01':
      return $gettext('Validation uses a DNS TXT record created with the selected DNS credential. Required for wildcard domains.')
    case 'existing':
      return $gettext('Use a certificate from the certificate manager. Nothing is requested from a certificate authority, and the certificate keeps its own renewal settings.')
    default:
      return props.skipDescription || $gettext('Continue without HTTPS. You can enable it later from the site editor.')
  }
})

watch(method, value => {
  if (isChallengeMethod(value))
    options.value.challenge_method = value
})

watch(() => [props.skippable, props.existingCertificate], () => {
  if ((!props.skippable && method.value === 'skip') || (!props.existingCertificate && method.value === 'existing'))
    method.value = 'http01'
})

// Selecting "Skip for now" only previews the choice; the caller acts on the
// explicit confirmation, since skipping usually saves and enables the site.
function confirmSkip() {
  if (method.value === 'skip')
    emit('skip')
}

// ---- Existing certificate -----------------------------------------------

const selectedCertificate = ref<Cert>()
const pickerOpen = ref(false)
const pickerRows = ref<Cert[]>([])
const recommendationLocked = ref(false)
const recommendedCertificateId = ref<number>()

function openPicker() {
  pickerRows.value = selectedCertificate.value ? [selectedCertificate.value] : []
  pickerOpen.value = true
}

function confirmPicker() {
  if (pickerRows.value.length) {
    selectedCertificate.value = pickerRows.value[0]
    // An explicit choice is never replaced by a recommendation.
    recommendationLocked.value = true
  }
  pickerOpen.value = false
}

// ---- Recommended certificate --------------------------------------------
// While the user has not picked a method or a certificate themselves, the card
// asks the backend for the certificate in the manager that covers every domain
// best and starts on "Existing certificate" with it selected, which saves the
// picker step. The backend applies the coverage rule of the run itself.

let recommendationRequest = 0

function onMethodChange() {
  recommendationLocked.value = true
}

function canRecommend() {
  return props.existingCertificate && !recommendationLocked.value && !running.value && phase.value === 'idle'
}

async function refreshRecommendation() {
  if (!canRecommend())
    return

  const request = ++recommendationRequest
  let best: Cert | null = null
  if (domainList.value.length) {
    try {
      best = (await certApi.recommend(domainList.value)).certificate
    }
    catch {
      // The recommendation is a convenience; the picker still works without it.
      return
    }
  }
  // A later domain change or a choice of the user wins over this answer.
  if (request !== recommendationRequest || !canRecommend())
    return

  if (best) {
    selectedCertificate.value = best
    recommendedCertificateId.value = best.id
    method.value = 'existing'
  }
  else if (recommendedCertificateId.value !== undefined) {
    // The domains changed and the recommendation no longer fits.
    selectedCertificate.value = undefined
    recommendedCertificateId.value = undefined
    method.value = 'http01'
  }
}

const isRecommendedCertificate = computed(() => selectedCertificate.value !== undefined
  && selectedCertificate.value.id === recommendedCertificateId.value)

watchDebounced(domainList, refreshRecommendation, { debounce: 300, deep: true, immediate: true })

const certificateName = computed(() => {
  const c = selectedCertificate.value
  return c ? (c.name || c.certificate_info?.subject_name || `#${c.id}`) : ''
})

const certificateNotAfter = computed(() => selectedCertificate.value?.certificate_info?.not_after)
const certificateExpired = computed(() => isCertificateExpired(certificateNotAfter.value))
const certificateExpiry = computed(() => {
  const notAfter = certificateNotAfter.value
  if (!notAfter || !dayjs(notAfter).isValid() || dayjs(notAfter).year() <= 1)
    return ''
  return dayjs(notAfter).format('YYYY-MM-DD HH:mm')
})

// A client-side preview; the backend reads the certificate file and decides.
// The record's own `domains` is not enough: an imported certificate stores only
// its subject name there, so the SANs come from the certificate file's info.
const certificateNames = computed(() => certificateNamesOf(selectedCertificate.value))
const coverage = computed(() => certificateCoverage(certificateNames.value, domainList.value))

// A certificate with many SANs would fill the card with tags.
const MAX_CERTIFICATE_TAGS = 6
const certificateTagsExpanded = ref(false)
const visibleCertificateNames = computed(() => certificateTagsExpanded.value
  ? certificateNames.value
  : certificateNames.value.slice(0, MAX_CERTIFICATE_TAGS))
const hiddenCertificateNameCount = computed(() => certificateNames.value.length - visibleCertificateNames.value.length)
watch(selectedCertificate, () => {
  certificateTagsExpanded.value = false
})

const certificateProblem = computed<{ type: 'error' | 'warning' | 'info', title: string } | undefined>(() => {
  if (method.value !== 'existing' || !selectedCertificate.value)
    return undefined
  if (certificateExpired.value)
    return { type: 'error', title: $gettext('This certificate expired on %{date}. Choose another certificate or renew this one first.', { date: certificateExpiry.value }) }

  const c = coverage.value
  if (!c.known)
    return { type: 'info', title: $gettext('This certificate record lists no domains. The pre-flight check reads them from the certificate file.') }
  if (domainList.value.length && !c.covered.length)
    return { type: 'error', title: $gettext('This certificate does not cover any of the domains: %{domains}', { domains: c.uncovered.join(', ') }) }
  if (c.uncovered.length)
    return { type: 'warning', title: $gettext('This certificate does not cover %{domains}. HTTPS is only enabled for the names it covers.', { domains: c.uncovered.join(', ') }) }
  return undefined
})

const { isAvailable: isDns01Available } = useDns01Plugin()

// ---- DNS-01 on the main node --------------------------------------------

const { canVerifyOnMain, mainDns01, nodeId, verifyOptions, verifyHint } = useMainNodeDns01()
const verifyOn = ref<DnsVerifyOn>('main')
const isOnMainNode = computed(() => canVerifyOnMain.value && method.value === 'dns01' && verifyOn.value === 'main')

// The DNS credential belongs to the node that validates, so a switch clears it.
watch(isOnMainNode, () => {
  options.value.dns_credential_id = undefined
})

const dns01Ready = computed(() => isOnMainNode.value ? mainDns01.value !== 'missing' : isDns01Available.value)

const methodProblem = computed(() => {
  if (method.value === 'dns01' && hasIPIdentifier.value)
    return $gettext('IP address certificates only support HTTP-01 validation.')
  if (method.value === 'http01' && hasWildcardIdentifier.value)
    return $gettext('Wildcard domains require DNS-01 validation.')
  return ''
})

const keyTypeOptions: SelectProps['options'] = PrivateKeyTypeList.map(t => ({
  value: t.key,
  label: t.name,
}))

// ---- Request / actions --------------------------------------------------

const formLocked = computed(() => running.value || phase.value === 'success')

const isExisting = computed(() => method.value === 'existing')

// ---- Collapse -----------------------------------------------------------

const collapsedPreference = useLocalStorage('nginx-ui-https-card-collapsed', false)

// Once a check has run or the run has started the card has something to show.
const collapsed = computed(() => props.collapsible
  && collapsedPreference.value
  && phase.value === 'idle'
  && !checking.value
  && !checks.value.length)

const collapsedSummary = computed(() => {
  const label = {
    http01: $gettext('HTTP-01'),
    dns01: $gettext('DNS-01'),
    existing: $gettext('Existing certificate'),
    skip: $gettext('Skip for now'),
  }[method.value]

  return [label, ...(method.value === 'skip' ? [] : domainList.value)].join(' · ')
})

function toggleCollapsed() {
  if (props.collapsible && phase.value === 'idle')
    collapsedPreference.value = !collapsedPreference.value
}

// "Check only" needs the certificate for the existing method; a challenge
// method can be checked before its credential is chosen.
const canCheck = computed(() => method.value !== 'skip'
  && domainList.value.length > 0
  && (!isExisting.value || !!selectedCertificate.value)
  && !props.disabled)

const canSubmit = computed(() => canCheck.value
  && !formLocked.value
  && (isExisting.value || (!methodProblem.value
    && (method.value !== 'dns01' || (dns01Ready.value && !!options.value.dns_credential_id)))))

function buildRequest(): HTTPSRequest {
  return buildHTTPSRequest({
    method: method.value,
    domains: domainList.value,
    redirectHTTPToHTTPS: redirectHTTPToHTTPS.value,
    options: options.value,
    certificateId: selectedCertificate.value?.id,
  })
}

function runCheck() {
  if (!canCheck.value)
    return

  onboarding.check(buildHTTPSCheckRequest(buildRequest()))
}

// The main node issues the certificate for the node, then the run on the node
// installs it like an existing certificate.
function delegatedIssue(request: HTTPSRequest): HTTPSDelegatedIssue | undefined {
  if (!isOnMainNode.value)
    return undefined

  const { domains: _domains, redirect_http_to_https: _redirect, certificate_id: _id, ...payload } = request
  return { nodeId: nodeId.value, payload }
}

function submit() {
  if (!canSubmit.value)
    return

  const request = buildRequest()
  onboarding.start(request, delegatedIssue(request))
}

// Stale check results would describe a different request.
watch([domainList, method, selectedCertificate], () => onboarding.clearChecks(), { deep: true })

watch(result, value => {
  if (value)
    emit('success', value)
})

// ---- Checklist ----------------------------------------------------------

function translate(message?: string, args?: HTTPSMessageArgs) {
  return message ? T({ message, args }) : ''
}

function stepLabel(step: HTTPSStep) {
  switch (step) {
    case 'delegate':
      return $gettext('Issue the certificate on the main node')
    case 'plan':
      return $gettext('Plan the configuration changes')
    case 'stage':
      return $gettext('Stage the HTTP configuration')
    case 'probe':
      return $gettext('Verify the challenge route')
    case 'issue':
      return $gettext('Issue the certificate')
    case 'finalize':
      return $gettext('Enable HTTPS')
    case 'rollback':
      return $gettext('Roll back the changes')
  }
}

function formatParams(params?: HTTPSMessageArgs) {
  if (!params)
    return ''

  return Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `${k}: ${Array.isArray(v) ? v.join(', ') : v}`)
    .join('  ')
}

const stepRows = computed<ChecklistRow[]>(() => {
  // A run with a certificate from the main node issues nothing on the node.
  const list: HTTPSStep[] = steps.value.delegate
    ? ['delegate', ...HTTPS_MAIN_STEPS.filter(step => step !== 'issue')]
    : [...HTTPS_MAIN_STEPS]
  if (steps.value.rollback)
    list.push('rollback')

  return list.map(step => {
    const s = steps.value[step]

    return {
      key: step,
      label: stepLabel(step),
      status: s?.status ?? 'pending',
      detail: translate(s?.message, s?.args),
    }
  })
})

const checkRows = computed<ChecklistRow[]>(() => checks.value.map((c, index) => ({
  key: `${c.code}-${index}`,
  label: translate(c.message, c.params as HTTPSMessageArgs | undefined),
  status: c.status,
  detail: c.detail || formatParams(c.params as HTTPSMessageArgs | undefined),
})))

const checklist = computed(() => phase.value === 'idle' ? checkRows.value : stepRows.value)

function statusText(status: RowStatus) {
  switch (status) {
    case 'running':
      return $gettext('Running')
    case 'success':
      return $gettext('Passed')
    case 'warning':
      return $gettext('Warning')
    case 'error':
      return $gettext('Failed')
    case 'skipped':
      return $gettext('Skipped')
    default:
      return $gettext('Waiting')
  }
}

// ---- Result -------------------------------------------------------------

const failedStepLabel = computed(() => error.value?.step ? stepLabel(error.value.step) : '')

const errorTitle = computed(() => {
  const e = error.value
  if (!e)
    return ''
  if (e.hint?.message)
    return translate(e.hint.message, e.hint.params)
  if (e.code === 'connection_closed')
    return $gettext('The connection closed before HTTPS setup finished. Retry to run it again.')
  return $gettext('HTTPS setup failed')
})

const errorDetail = computed(() => {
  const e = error.value
  if (!e || e.code === 'connection_closed')
    return ''
  return translate(e.message, e.args)
})

// The failed step is already marked in the progress list, usually with the raw
// error as its detail, so the alert only repeats what the list cannot show.
const failedStepInList = computed(() => stepRows.value.some(r => r.status === 'error'))

const errorDetailInList = computed(() => Boolean(errorDetail.value)
  && stepRows.value.some(r => r.status === 'error' && r.detail === errorDetail.value))

const visibleDiagnostics = computed(() => phase.value === 'error'
  ? diagnosticsWithoutHint(diagnostics.value, error.value?.hint?.code)
  : diagnostics.value)

function diagnosticType(level: string) {
  if (level === 'error')
    return 'error'
  if (level === 'warning')
    return 'warning'
  return 'info'
}

const logText = computed(() => logs.value.map(l => translate(l.message, l.args)).join('\n'))

const logItems = computed(() => [{ key: 'log', label: $gettext('Log') }])
const certificateOptionItems = computed(() => [{ key: 'options', label: $gettext('Certificate options') }])

function restart() {
  onboarding.reset()
}

// The action of the primary button: skip, or run (and retry) the setup.
function confirm() {
  if (method.value === 'skip')
    confirmSkip()
  else
    submit()
}

const canConfirm = computed(() => method.value === 'skip' || canSubmit.value)

// With an external confirm button, only "Check only" (and the slot) is left.
const hasActions = computed(() => Boolean(slots.actions)
  || (method.value !== 'skip' && phase.value === 'idle')
  || (!props.externalConfirm && (method.value === 'skip' || phase.value !== 'success')))

defineExpose({
  running,
  method,
  phase,
  canConfirm,
  reset: restart,
  check: runCheck,
  submit,
  confirm,
})
</script>

<template>
  <ACard
    class="https-card"
    :size="compact ? 'small' : 'medium'"
  >
    <template #title>
      <AFlex
        align="center"
        gap="small"
        :class="{ 'https-card-toggle': collapsible }"
        :role="collapsible ? 'button' : undefined"
        :tabindex="collapsible ? 0 : undefined"
        :aria-expanded="collapsible ? !collapsed : undefined"
        @click="toggleCollapsed"
        @keydown.enter.prevent="toggleCollapsed"
        @keydown.space.prevent="toggleCollapsed"
      >
        <DownOutlined
          v-if="collapsible"
          class="https-card-chevron"
          :class="{ 'is-collapsed': collapsed }"
        />
        <SafetyCertificateOutlined />
        <span>{{ $gettext('HTTPS') }}</span>
      </AFlex>
    </template>

    <p v-if="collapsed" class="https-muted m-0 break-words text-sm">
      {{ collapsedSummary }}
    </p>

    <AFlex
      v-show="!collapsed"
      vertical
      :gap="compact ? 12 : 16"
    >
      <p v-if="!compact" class="https-muted m-0">
        {{ $gettext('Nginx UI stages the challenge route, verifies it, requests the certificate and switches the site to HTTPS in one run. If a step fails, the site keeps serving plain HTTP.') }}
      </p>

      <!-- Method -->
      <div>
        <div class="https-label">
          {{ $gettext('Method') }}
        </div>
        <div class="max-w-full overflow-x-auto">
          <ASegmented
            v-model:value="method"
            :options="methodOptions"
            :disabled="formLocked"
            :vertical="isNarrow"
            :block="isNarrow"
            @change="onMethodChange"
          />
        </div>
        <p class="https-muted mb-0 mt-2 text-sm">
          {{ methodDescription }}
        </p>
      </div>

      <template v-if="method !== 'skip'">
        <!-- Domains -->
        <div>
          <div class="https-label">
            {{ $gettext('Domains') }}
          </div>
          <AFlex wrap gap="small" align="center">
            <ATag
              v-for="d in domainList"
              :key="d"
              class="m-0 font-mono"
              variant="outlined"
              :closable="!formLocked"
              @close="(e: MouseEvent) => removeDomain(e, d)"
            >
              {{ d }}
            </ATag>
            <span v-if="!domainList.length" class="https-muted text-sm">
              {{ $gettext('Add at least one domain or IP address.') }}
            </span>
          </AFlex>
          <ASpaceCompact
            v-if="!formLocked"
            block
            class="mt-2 max-w-100"
          >
            <AInput
              v-model:value="domainInput"
              :status="domainInputError ? 'error' : undefined"
              :placeholder="$gettext('e.g. example.com www.example.com')"
              @press-enter="addDomains"
            />
            <AButton @click="addDomains">
              <PlusOutlined />
              {{ $gettext('Add') }}
            </AButton>
          </ASpaceCompact>
          <div v-if="domainInputError" class="https-error-text mt-1 text-sm">
            {{ domainInputError }}
          </div>
          <AAlert
            v-if="methodProblem"
            class="mt-2"
            type="warning"
            show-icon
            :title="methodProblem"
          />
        </div>

        <!-- Existing certificate -->
        <div v-if="isExisting">
          <div class="https-label">
            {{ $gettext('Certificate') }}
          </div>
          <div v-if="selectedCertificate" class="https-certificate">
            <AFlex justify="space-between" align="flex-start" gap="small">
              <div class="min-w-0">
                <AFlex align="center" gap="small" wrap>
                  <span class="break-words font-medium">{{ certificateName }}</span>
                  <ATag
                    v-if="isRecommendedCertificate"
                    class="m-0"
                    color="success"
                  >
                    {{ $gettext('Recommended') }}
                  </ATag>
                </AFlex>
                <div
                  v-if="certificateExpiry"
                  class="text-sm"
                  :class="certificateExpired ? 'https-error-text' : 'https-muted'"
                >
                  {{ certificateExpired
                    ? $gettext('Expired on %{date}', { date: certificateExpiry })
                    : $gettext('Expires on %{date}', { date: certificateExpiry }) }}
                </div>
              </div>
              <AButton
                size="small"
                :disabled="formLocked"
                @click="openPicker"
              >
                {{ $gettext('Change') }}
              </AButton>
            </AFlex>
            <AFlex
              v-if="certificateNames.length"
              wrap
              gap="small"
              class="mt-2"
            >
              <ATag
                v-for="d in visibleCertificateNames"
                :key="d"
                class="m-0 font-mono"
                variant="filled"
              >
                {{ d }}
              </ATag>
              <AButton
                v-if="hiddenCertificateNameCount > 0"
                size="small"
                type="link"
                @click="certificateTagsExpanded = true"
              >
                {{ $gettext('%{count} more', { count: String(hiddenCertificateNameCount) }) }}
              </AButton>
            </AFlex>
          </div>
          <AButton
            v-else
            :disabled="formLocked"
            @click="openPicker"
          >
            <PlusOutlined />
            {{ $gettext('Select certificate') }}
          </AButton>
          <AAlert
            v-if="certificateProblem"
            class="mt-2"
            :type="certificateProblem.type"
            show-icon
            :title="certificateProblem.title"
          />
        </div>

        <!-- DNS credential -->
        <div v-if="canVerifyOnMain && method === 'dns01'">
          <div class="https-label">
            {{ $gettext('DNS Validation') }}
          </div>
          <ASegmented
            v-model:value="verifyOn"
            :options="verifyOptions"
            :disabled="formLocked"
          />
          <p class="https-muted mb-0 mt-2 text-sm">
            {{ verifyHint(verifyOn) }}
          </p>
        </div>
        <template v-if="isOnMainNode">
          <AAlert
            v-if="mainDns01 === 'missing'"
            type="warning"
            show-icon
            :title="$gettext('The main node has no DNS-01 plugin. Install it on the main node, or validate on this node.')"
          />
          <div v-else class="max-w-100">
            <DNSChallenge v-model:options="options" main-node />
          </div>
        </template>
        <Dns01PluginNotice v-else-if="method === 'dns01' && !isDns01Available" variant="https" />
        <div v-else-if="method === 'dns01'" class="max-w-100">
          <PluginSlot name="certificate.challenge.form:dns01" :context="{ options }">
            <DNSChallenge v-model:options="options" />
          </PluginSlot>
        </div>

        <ACheckbox v-model:checked="redirectHTTPToHTTPS" :disabled="formLocked">
          {{ $gettext('Redirect HTTP to HTTPS after the certificate is ready') }}
        </ACheckbox>

        <p v-if="hasPendingTLSServer" class="https-muted m-0 text-sm">
          {{ $gettext('The HTTPS server block without a certificate is left out until the certificate is issued, then enabled with it.') }}
        </p>

        <!-- Certificate options (ACME only) -->
        <ACollapse
          v-if="!isExisting"
          v-model:active-key="certificateOptionKeys"
          :items="certificateOptionItems"
          :size="compact ? 'small' : 'middle'"
        >
          <template #contentRender>
            <div class="max-w-150">
              <AForm layout="vertical" :disabled="formLocked">
                <AFormItem :label="$gettext('Key Type')">
                  <ASelect
                    v-model:value="options.key_type"
                    class="max-w-100"
                    :options="keyTypeOptions"
                  />
                </AFormItem>
              </AForm>
              <ACMEUserSelector v-model:options="options" />
              <AForm layout="vertical" :disabled="formLocked">
                <AFormItem
                  :label="$gettext('OCSP Must Staple')"
                  :help="$gettext('OCSP Must Staple may cause errors for some users on first access using Firefox.')"
                >
                  <ASwitch v-model:checked="options.must_staple" />
                </AFormItem>
                <AFormItem
                  :label="$gettext('Enable Common Name')"
                  :help="$gettext('Enable the certificate Common Name field for private CAs that still require it.')"
                >
                  <ASwitch v-model:checked="options.enable_common_name" />
                </AFormItem>
                <AFormItem
                  :label="$gettext('Revoke Old Certificate')"
                  :help="$gettext('If you want to automatically revoke the old certificate, please enable this option.')"
                >
                  <ASwitch v-model:checked="options.revoke_old" />
                </AFormItem>
              </AForm>
            </div>
          </template>
        </ACollapse>

        <!-- Checklist -->
        <div>
          <div class="https-label">
            {{ phase === 'idle' ? $gettext('Pre-flight check') : $gettext('Progress') }}
          </div>
          <p
            v-if="phase === 'idle' && !checklist.length"
            class="https-muted m-0 text-sm"
          >
            {{ checking
              ? $gettext('Checking...')
              : isExisting
                ? $gettext('Run "Check only" to verify the configuration and the certificate without changing anything.')
                : $gettext('Run "Check only" to verify the configuration and DNS without changing anything.') }}
          </p>
          <ul v-else class="https-checklist m-0 list-none p-0">
            <li
              v-for="row in checklist"
              :key="row.key"
              class="https-checklist-row"
              :class="`is-${row.status}`"
            >
              <span class="https-status-icon" :title="statusText(row.status)">
                <LoadingOutlined v-if="row.status === 'running'" />
                <CheckCircleFilled v-else-if="row.status === 'success'" />
                <ExclamationCircleFilled v-else-if="row.status === 'warning'" />
                <CloseCircleFilled v-else-if="row.status === 'error'" />
                <MinusCircleOutlined v-else-if="row.status === 'skipped'" />
                <ClockCircleOutlined v-else />
              </span>
              <div class="min-w-0 flex-1">
                <div>{{ row.label }}</div>
                <div v-if="row.detail" class="https-mono break-words text-xs">
                  {{ row.detail }}
                </div>
              </div>
            </li>
          </ul>
        </div>

        <!-- Diagnostics -->
        <AFlex v-if="visibleDiagnostics.length" vertical gap="small">
          <AAlert
            v-for="(d, index) in visibleDiagnostics"
            :key="`${d.code}-${index}`"
            :type="diagnosticType(d.level)"
            show-icon
            :title="translate(d.message, d.params)"
          />
        </AFlex>

        <!-- Error -->
        <AAlert
          v-if="phase === 'error' && error"
          type="warning"
          show-icon
          :title="errorTitle"
        >
          <template v-if="(failedStepLabel && !failedStepInList) || (errorDetail && !errorDetailInList)" #description>
            <div v-if="failedStepLabel && !failedStepInList">
              {{ $gettext('Failed step: %{step}', { step: failedStepLabel }) }}
            </div>
            <div v-if="errorDetail && !errorDetailInList" class="https-mono mt-1 break-words text-xs">
              {{ errorDetail }}
            </div>
          </template>
        </AAlert>

        <!-- Success -->
        <AAlert
          v-if="phase === 'success' && result"
          type="success"
          show-icon
          :title="$gettext('HTTPS is enabled')"
        >
          <template #description>
            <div class="text-sm">
              {{ $gettext('Certificate') }}
            </div>
            <ATypographyParagraph
              class="https-mono mb-2 break-words text-xs"
              :copyable="{ text: result.ssl_certificate, tooltips: false }"
            >
              {{ result.ssl_certificate }}
            </ATypographyParagraph>
            <div class="text-sm">
              {{ $gettext('Private Key') }}
            </div>
            <ATypographyParagraph
              class="https-mono mb-0 break-words text-xs"
              :copyable="{ text: result.ssl_certificate_key, tooltips: false }"
            >
              {{ result.ssl_certificate_key }}
            </ATypographyParagraph>
          </template>
        </AAlert>

        <!-- Log -->
        <ACollapse
          v-if="phase !== 'idle' && logs.length"
          :items="logItems"
          :size="compact ? 'small' : 'middle'"
        >
          <template #contentRender>
            <pre class="https-log m-0">{{ logText }}</pre>
          </template>
        </ACollapse>
      </template>

      <!-- Actions -->
      <AFlex v-if="hasActions" wrap gap="small" align="center">
        <template v-if="method !== 'skip' && phase !== 'success'">
          <AButton
            v-if="phase === 'idle'"
            :loading="checking"
            :disabled="!canCheck"
            @click="runCheck"
          >
            {{ $gettext('Check only') }}
          </AButton>
          <AButton
            v-if="!externalConfirm"
            type="primary"
            :loading="running"
            :disabled="!canSubmit && !running"
            @click="submit"
          >
            {{ phase === 'error'
              ? $gettext('Retry')
              : isExisting ? $gettext('Enable HTTPS') : $gettext('Issue and enable HTTPS') }}
          </AButton>
        </template>
        <AButton
          v-if="method === 'skip' && !externalConfirm"
          type="primary"
          @click="confirmSkip"
        >
          {{ $gettext('Continue without HTTPS') }}
        </AButton>
        <slot name="actions" />
      </AFlex>
    </AFlex>

    <AModal
      v-model:open="pickerOpen"
      :title="$gettext('Select certificate')"
      :width="800"
      :ok-button-props="{ disabled: !pickerRows.length }"
      @ok="confirmPicker"
    >
      <CertificatePicker
        v-model:selected-rows="pickerRows"
        selection-type="radio"
      />
    </AModal>
  </ACard>
</template>

<style scoped lang="less">
.https-label {
  font-weight: 500;
  margin-bottom: 8px;
  color: var(--ant-color-text);
}

.https-muted {
  color: var(--ant-color-text-secondary);
}

.https-card-toggle {
  cursor: pointer;
  user-select: none;
}

.https-card-chevron {
  font-size: 12px;
  color: var(--ant-color-text-secondary);
  transition: transform 0.2s;

  &.is-collapsed {
    transform: rotate(-90deg);
  }
}

.https-error-text {
  color: var(--ant-color-error);
}

.https-mono {
  font-family: var(--ant-font-family-code, ui-monospace, SFMono-Regular, Menlo, monospace);
  color: var(--ant-color-text-secondary);
}

.https-certificate {
  padding: 10px 12px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius-lg);
  background: var(--ant-color-fill-quaternary);
  color: var(--ant-color-text);
}

.https-checklist {
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius-lg);
  background: var(--ant-color-bg-container);
}

.https-checklist-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 12px;
  color: var(--ant-color-text);

  & + & {
    border-top: 1px solid var(--ant-color-split);
  }

  &.is-pending {
    color: var(--ant-color-text-tertiary);
  }
}

.https-status-icon {
  flex: none;
  line-height: 22px;
  font-size: 16px;
  color: var(--ant-color-text-quaternary);

  .is-running & {
    color: var(--ant-color-primary);
  }

  .is-success & {
    color: var(--ant-color-success);
  }

  .is-warning & {
    color: var(--ant-color-warning);
  }

  .is-error & {
    color: var(--ant-color-error);
  }
}

.https-log {
  max-height: 320px;
  overflow: auto;
  padding: 10px;
  border-radius: var(--ant-border-radius);
  background: var(--ant-color-fill-tertiary);
  color: var(--ant-color-text);
  font-family: var(--ant-font-family-code, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
