<script setup lang="ts">
import type { Ref } from 'vue'
import type { Cert, SelfSignedCertPayload } from '@/api/cert'
import cert, { toSelfSignedPayload } from '@/api/cert'
import { AutoCertState, normalizePrivateKeyType } from '@/constants'

import AutoCertManagement from './components/AutoCertManagement.vue'
import CertificateActions from './components/CertificateActions.vue'
import CertificateBasicInfo from './components/CertificateBasicInfo.vue'
import CertificateContentEditor from './components/CertificateContentEditor.vue'
import CertificateDeployTargets from './components/CertificateDeployTargets.vue'
import SelfSignedCertManagement from './components/SelfSignedCertManagement.vue'
import { useCertStore } from './store'

const { message } = App.useApp()

const route = useRoute()
const certStore = useCertStore()
const router = useRouter()
const errors = ref({}) as Ref<Record<string, string>>

const id = computed(() => {
  return Number.parseInt(route.params.id as string)
})

const { data } = storeToRefs(certStore)
const leftTopContent = useTemplateRef('leftTopContent')
const logCardRef = useTemplateRef('logCardRef')
const logContentHeight = ref<number | null>(null)

let layoutObserver: ResizeObserver | null = null
let handleWindowResize: (() => void) | null = null

function scheduleLogHeightUpdate() {
  requestAnimationFrame(updateLogContentHeight)
}

function reconnectLayoutObserver() {
  layoutObserver?.disconnect()
  layoutObserver = null

  const leftElement = resolveHTMLElement(leftTopContent.value)
  const cardElement = resolveHTMLElement(logCardRef.value)
  if (!leftElement || !cardElement)
    return

  layoutObserver = new ResizeObserver(scheduleLogHeightUpdate)
  layoutObserver.observe(leftElement)
  layoutObserver.observe(cardElement)
  if (cardElement.parentElement)
    layoutObserver.observe(cardElement.parentElement)
}

const isManaged = computed(() => {
  return data.value.auto_cert === AutoCertState.Enable
    || data.value.auto_cert === AutoCertState.Paused
    || data.value.auto_cert === AutoCertState.Sync
})

const isSelfSigned = computed(() => {
  return data.value.auto_cert === AutoCertState.SelfSigned
})

const isGeneral = computed(() => {
  if (typeof data.value.auto_cert !== 'number')
    return false

  return !isManaged.value && !isSelfSigned.value
})

const selfSignedPayload = ref<SelfSignedCertPayload>()

watch(data, value => {
  if (value.auto_cert === AutoCertState.SelfSigned)
    selfSignedPayload.value = toSelfSignedPayload(value)
}, { immediate: true })

// The store is a singleton, so editing another certificate starts out holding
// the previous one's data. Every load takes a ticket so a slow response cannot
// land after the operator moved on to another record.
let loadSeq = 0

function init() {
  const seq = ++loadSeq
  const target = id.value

  // Keep the form filled when this is a reload of the record already on screen,
  // e.g. after a renewal.
  if (data.value.id !== target) {
    data.value = {} as Cert
    selfSignedPayload.value = undefined
  }

  if (!(target > 0))
    return

  cert.getItem(target).then(r => {
    if (seq !== loadSeq)
      return

    // Backend stores key_type in its canonical form (EC256, RSA2048…); the
    // ACME form's ASelect options use the legacy keys (P256, 2048…). Normalize
    // on load so the dropdown highlights the right option when editing.
    data.value = { ...r, key_type: normalizePrivateKeyType(r.key_type) }
  })
}

// Vue Router can reuse this component when only the id changes, so reload on
// the route parameter instead of on mount alone.
watch(id, init, { immediate: true })

async function save() {
  try {
    let savedId = data.value.id
    if (isSelfSigned.value && selfSignedPayload.value && data.value.id) {
      const payload = selfSignedPayload.value
      const name = payload.name.trim()
      const domains = payload.domains.map(d => d.trim()).filter(Boolean)
      const ip_addresses = payload.ip_addresses.map(s => s.trim()).filter(Boolean)

      if (!name) {
        message.error($gettext('Please enter a name for the certificate'))
        return
      }
      if (domains.length === 0 && ip_addresses.length === 0) {
        message.error($gettext('Please enter at least one domain or IP address'))
        return
      }

      const currentId = data.value.id
      const result = await cert.modify_self_signed(currentId, {
        ...payload,
        name,
        domains,
        ip_addresses,
      })
      savedId = result.id || currentId
      data.value = { ...result, id: savedId }
    }
    else {
      await certStore.save()
      savedId = data.value.id
    }
    if (!savedId) {
      message.error($gettext('Saved certificate response is missing an ID'))
      return
    }
    message.success($gettext('Save successfully'))
    errors.value = {}
    await router.push(`/certificates/${savedId}`)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    errors.value = e.errors ?? {}
    message.error(e.message ?? $gettext('Server error'))
  }
}

function handleBack() {
  router.push('/certificates/list')
}

const logLevelLabels: Record<string, string> = {
  INFO: 'Info',
  WARN: 'Warning',
  ERROR: 'Error',
  DEBUG: 'Debug',
}

// Keep these literals in source so gettext extraction includes structured log
// message keys that arrive dynamically from ACME libraries.
const structuredLogMessageI18nHints = [
  $gettext('Trying renewal.'),
  $gettext('Obtaining bundled SAN certificate.'),
  $gettext('Use solver.'),
  $gettext('http01: Trying to solve HTTP-01.'),
  $gettext('The server validated our request.'),
  $gettext('Validations succeeded; requesting certificates.'),
  $gettext('Waiting for certificates.'),
  $gettext('Server responded with a certificate.'),
]
void structuredLogMessageI18nHints

const structuredLogFieldI18nHints = [
  $gettext('domains'),
  $gettext('domain'),
  $gettext('type'),
  $gettext('hoursRemaining'),
  $gettext('timeout'),
  $gettext('interval'),
]
void structuredLogFieldI18nHints

function localizeStructuredLevelValue(raw: string) {
  return raw.replace(/(^|\s)(level|等级|層級)=([A-Z]+)/g, (_, prefix: string, key: string, level: string) => {
    const mapped = logLevelLabels[level] || level
    return `${prefix}${key}=${$gettext(mapped)}`
  })
}

function stripTimeKey(raw: string) {
  return raw.replace(/^(time|时间|時間)=(\S+)/, '$2')
}

function translateStructuredMessage(message: string) {
  return $gettext(message)
}

function applyKeywordLineBreaks(raw: string) {
  return raw.replace(/\s+(消息|msg|訊息|域名列表|domains|網域列表|域名|domain|網域|type|timeout|interval|hoursRemaining)=/g, '\n$1=')
}

function applyAuxiliaryFieldFormatting(raw: string) {
  let localized = raw.replace(/^(domain|type|hoursRemaining|timeout|interval)=([^\n]*)$/gm, (_, key: string, value: string) => {
    return `${$gettext(key)}:${value}`
  })

  localized = localized.replace(/(domains)=("([^"]*)"|(\S+))/g, (_, key: string, full: string, quoted: string | undefined, plain: string | undefined) => {
    const value = (quoted ?? plain ?? '').trim()
    const domains = value
      .split(/[\s,，;；]+/)
      .map(item => item.trim())
      .filter(Boolean)
    const label = $gettext(key)

    if (domains.length <= 1)
      return `${label}:${full}`

    return `${label}:\n${domains.map(domain => `- ${domain}`).join('\n')}`
  })

  return localized
}

function applyNginxUILineBreaks(raw: string) {
  return raw
    .replace(/，邮箱：/g, '\n邮箱：')
    .replace(/,\s*Email:/g, '\nEmail:')
    .replace(/，CA 目录：/g, '\nCA 目录：')
    .replace(/,\s*CA Dir:/g, '\nCA Dir:')
}

function stripMessageWrapper(raw: string) {
  return raw
    .replace(/^(msg|消息|訊息)="([^"]*)"$/gm, '$2')
    .replace(/^(msg|消息|訊息)=(.+)$/gm, '$2')
}

function localizeStructuredLogLine(raw: string) {
  const translatedWhole = $gettext(raw)
  if (translatedWhole !== raw)
    return translatedWhole

  let localized = localizeStructuredLevelValue(raw)
  localized = stripTimeKey(localized)

  const match = raw.match(/msg="([^"]+)"/)
  if (!match)
    return localized

  const originalMessage = match[1]
  const translatedMessage = translateStructuredMessage(originalMessage)

  if (translatedMessage !== originalMessage) {
    localized = localized.replace(`msg="${originalMessage}"`, `msg="${translatedMessage}"`)
    localized = localized.replace(`消息="${originalMessage}"`, `消息="${translatedMessage}"`)
    localized = localized.replace(`訊息="${originalMessage}"`, `訊息="${translatedMessage}"`)
  }

  localized = applyKeywordLineBreaks(localized)
  localized = applyAuxiliaryFieldFormatting(localized)
  return stripMessageWrapper(localized)
}

function renderLocalizedLogMessage(raw: string) {
  const matches = raw.match(/\[Nginx UI\] (.*)/)
  if (matches?.[1])
    return applyNginxUILineBreaks(raw.replaceAll(matches[1], $gettext(matches[1])))

  return localizeStructuredLogLine(raw)
}

const log = computed(() => {
  if (!data.value.log)
    return ''

  const lines = data.value.log.split('\n').map(line => {
    try {
      return renderLocalizedLogMessage(T(JSON.parse(line)))
    }
    catch {
      return renderLocalizedLogMessage(line)
    }
  }).map(line => line.startsWith('[Nginx UI]') ? `${line}\n` : line)

  return lines.join('\n')
})

function resolveHTMLElement(target: unknown): HTMLElement | null {
  if (!target)
    return null

  if (target instanceof HTMLElement)
    return target

  const maybeEl = (target as { $el?: unknown }).$el
  if (maybeEl instanceof HTMLElement)
    return maybeEl

  return null
}

function parsePx(value: string) {
  const n = Number.parseFloat(value)
  return Number.isFinite(n) ? n : 0
}

function updateLogContentHeight() {
  const leftElement = resolveHTMLElement(leftTopContent.value)
  const cardElement = resolveHTMLElement(logCardRef.value)
  if (!leftElement || !cardElement)
    return

  // The deploy targets card sits above the log card in the same column.
  const leftRect = leftElement.getBoundingClientRect()
  const offset = Math.max(0, cardElement.getBoundingClientRect().top - leftRect.top)
  const totalHeight = leftRect.height - offset
  const head = cardElement.querySelector('.ant-card-head') as HTMLElement | null
  const body = cardElement.querySelector('.ant-card-body') as HTMLElement | null
  const headHeight = head?.getBoundingClientRect().height ?? 0

  let bodyPadding = 0
  if (body) {
    const styles = window.getComputedStyle(body)
    bodyPadding = parsePx(styles.paddingTop) + parsePx(styles.paddingBottom)
  }

  const next = Math.floor(totalHeight - headHeight - bodyPadding)
  if (next > 0)
    logContentHeight.value = next
}

onMounted(() => {
  scheduleLogHeightUpdate()
  reconnectLayoutObserver()

  handleWindowResize = scheduleLogHeightUpdate
  window.addEventListener('resize', handleWindowResize)
})

watch(
  () => data.value.auto_cert,
  async autoCertState => {
    if (autoCertState !== AutoCertState.Enable) {
      layoutObserver?.disconnect()
      layoutObserver = null
      logContentHeight.value = null
      return
    }

    await nextTick()
    reconnectLayoutObserver()
    scheduleLogHeightUpdate()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  layoutObserver?.disconnect()
  layoutObserver = null
  if (handleWindowResize)
    window.removeEventListener('resize', handleWindowResize)
  handleWindowResize = null
})
</script>

<template>
  <ACard>
    <template #title>
      <div v-if="!isSelfSigned" class="editor-title-name">
        <AInput
          v-model:value="data.name"
          class="editor-title-input"
          :disabled="isManaged"
        />
      </div>
      <span v-else>{{ id > 0 ? $gettext('Modify Certificate') : $gettext('Import Certificate') }}</span>
    </template>
    <template #extra>
      <ATag v-if="isManaged" color="success" class="managed-cert-tag">
        {{ $gettext('This certificate is managed by Nginx UI') }}
      </ATag>
      <ATag v-else-if="isGeneral" color="purple" variant="filled" class="general-cert-tag">
        {{ $gettext('General Certificate') }} · {{ $gettext('This certificate is not managed by Nginx UI') }}
      </ATag>
    </template>

    <ARow :gutter="[16, 16]" class="main-top-row">
      <ACol
        :sm="24"
        :lg="14"
      >
        <div ref="leftTopContent" class="left-top-content">
          <!-- Self-signed Certificate Management -->
          <SelfSignedCertManagement
            v-if="isSelfSigned && selfSignedPayload"
            v-model:value="selfSignedPayload"
            :certificate-info="data.certificate_info"
          />

          <!-- Auto Certificate Management -->
          <AutoCertManagement
            v-else
            v-model:data="data"
            :is-managed="isManaged"
            @renewed="init"
          />

          <AForm layout="vertical">
            <!-- Certificate Basic Information -->
            <CertificateBasicInfo
              v-if="!isSelfSigned"
              v-model:data="data"
              :errors="errors"
              :is-managed="isManaged"
              :show-name-field="false"
            />
          </AForm>
        </div>
      </ACol>

      <!-- Deploy Targets and Log Column -->
      <ACol
        v-if="id > 0"
        :sm="24"
        :lg="10"
        class="log-col"
      >
        <CertificateDeployTargets :cert-id="id" />

        <ACard
          v-if="data.auto_cert === AutoCertState.Enable || data.auto_cert === AutoCertState.Paused"
          ref="logCardRef"
          size="small"
          :title="$gettext('Log')"
          class="log-card"
        >
          <pre
            v-dompurify-html="log"
            class="log-container"
            :style="logContentHeight ? { height: `${logContentHeight}px` } : undefined"
          />
        </ACard>
      </ACol>
    </ARow>

    <div class="content-editor-bottom">
      <CertificateContentEditor
        v-model:data="data"
        :errors="errors"
        :readonly="isManaged || isSelfSigned"
      />
    </div>

    <!-- Certificate Actions -->
    <CertificateActions
      @save="save"
      @back="handleBack"
    />
  </ACard>
</template>

<style scoped lang="less">
.main-top-row {
  align-items: stretch;
}

.editor-title-name {
  display: flex;
  align-items: center;
  width: 100%;
}

.editor-title-input {
  max-width: 560px;
}

.left-top-content {
  width: 100%;
}

.log-col {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.log-card {
  display: flex;
  flex: 1;
  flex-direction: column;
  width: 100%;
  min-height: 0;
  overflow: hidden;
}

.log-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.log-container {
  overflow-y: auto;
  overflow-x: hidden;
  padding: 5px;
  margin: 0;

  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.managed-cert-tag {
  font-size: 16px;
  line-height: 1.2;
}

.general-cert-tag {
  font-size: 16px;
  line-height: 1.2;
  border: none;
}

.content-editor-bottom {
  margin-top: 16px;
}

.code-editor-container {
  position: relative;

  .drag-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(24, 144, 255, 0.1);
    border: 2px dashed #1890ff;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10;

    .drag-content {
      text-align: center;
      color: #1890ff;

      .drag-icon {
        font-size: 48px;
        margin-bottom: 16px;
        display: block;
      }

      p {
        font-size: 16px;
        margin: 0;
        font-weight: 500;
      }
    }
  }
}
</style>
