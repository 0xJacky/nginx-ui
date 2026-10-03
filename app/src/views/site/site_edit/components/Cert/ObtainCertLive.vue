<script setup lang="ts">
import type { Ref } from 'vue'
import type { IssueHint } from './issueFailure'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { CertificateResult } from '@/api/cert'
import { AutoCertChallengeMethod } from '@/api/auto_cert'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useWebSocket } from '@/lib/websocket'
import { useSettingsStore } from '@/pinia'
import { useSiteEditorStore } from '../SiteEditor/store'
import { issueFailureDetail, issueHintTitle } from './issueFailure'

const props = defineProps<{
  options: AutoCertOptions
}>()

const emit = defineEmits<{
  retry: []
}>()

const settings = useSettingsStore()

const modalVisible = defineModel<boolean>('modalVisible')
const modalClosable = defineModel<boolean>('modalClosable')

const editorStore = useSiteEditorStore()
const { issuingCert } = storeToRefs(editorStore)
const otpModal = use2FAModal()

const progressStrokeColor = {
  from: '#108ee9',
  to: '#87d068',
}

const progressPercent = ref(0)
const progressStatus = ref('active') as Ref<'success' | 'active' | 'normal' | 'exception'>
const failureDetail = ref('')

const logContainer = useTemplateRef('logContainer')

const hint = ref<IssueHint>()
const hintTitle = computed(() => issueHintTitle(hint.value))
const hasFailed = computed(() => progressStatus.value === 'exception')

const logLevelLabels: Record<string, string> = {
  INFO: 'Info',
  WARN: 'Warning',
  ERROR: 'Error',
  DEBUG: 'Debug',
}

const structuredLogKeys = [
  'time',
  'level',
  'msg',
  'domain',
  'domains',
  'type',
  'timeout',
  'interval',
  'hoursRemaining',
]

function localizeStructuredFieldKeys(raw: string) {
  let localized = raw
  for (const key of structuredLogKeys) {
    const translatedKey = $gettext(key)
    if (translatedKey === key)
      continue

    const pattern = new RegExp(`(^|\\s)${key}=`, 'g')
    localized = localized.replace(pattern, `$1${translatedKey}=`)
  }

  return localized
}

function localizeStructuredLevelValue(raw: string) {
  return raw.replace(/(^|\s)(level|等级|層級)=([A-Z]+)/g, (_, prefix: string, key: string, level: string) => {
    const mapped = logLevelLabels[level] || level
    return `${prefix}${key}=${$gettext(mapped)}`
  })
}

function applyKeywordLineBreaks(raw: string) {
  return raw.replace(/\s+(消息|msg|訊息|域名列表|domains|網域列表|域名|domain|網域)=/g, '\n$1=')
}

function applyDomainListValueLineBreaks(raw: string) {
  return raw.replace(/(域名列表|domains|網域列表)=("([^"]*)"|(\S+))/g, (_, key: string, full: string, quoted: string | undefined, plain: string | undefined) => {
    const value = (quoted ?? plain ?? '').trim()
    const domains = value
      .split(/[\s,，;；]+/)
      .map(item => item.trim())
      .filter(Boolean)

    if (domains.length <= 1)
      return `${key}：${full}`

    return `${key}：\n${domains.map(domain => `- ${domain}`).join('\n')}`
  })
}

function applyNginxUILineBreaks(raw: string) {
  // Keep the Nginx UI prefix on the first line and split long ACME user lines.
  return raw
    .replace(/，邮箱：/g, '\n邮箱：')
    .replace(/,\s*Email:/g, '\nEmail:')
    .replace(/，CA 目录：/g, '\nCA 目录：')
    .replace(/,\s*CA Dir:/g, '\nCA Dir:')
}

function localizeLogLine(raw: string) {
  if (raw.includes('[Nginx UI]'))
    return applyNginxUILineBreaks(raw)

  const translatedWhole = $gettext(raw)
  if (translatedWhole !== raw)
    return translatedWhole

  // lego emits structured logs like:
  // time=... level=INFO msg="Trying renewal." domains="..."
  // Translate structured keys and level values while preserving dynamic values.
  let localized = localizeStructuredFieldKeys(raw)
  localized = localizeStructuredLevelValue(localized)

  const match = raw.match(/msg="([^"]+)"/)
  if (!match)
    return localized

  const originalMessage = match[1]
  const translatedMessage = $gettext(originalMessage)

  if (translatedMessage !== originalMessage) {
    localized = localized.replace(`msg="${originalMessage}"`, `msg="${translatedMessage}"`)
    localized = localized.replace(`消息="${originalMessage}"`, `消息="${translatedMessage}"`)
    localized = localized.replace(`訊息="${originalMessage}"`, `訊息="${translatedMessage}"`)
  }

  localized = applyKeywordLineBreaks(localized)
  return applyDomainListValueLineBreaks(localized)
}

function log(msg: string) {
  const para = document.createElement('p')

  para.appendChild(document.createTextNode(localizeLogLine(msg)))

  logContainer.value!.appendChild(para)

  logContainer.value?.scroll({ top: 100000, left: 0, behavior: 'smooth' })
}

async function issue_cert(config_name: string, server_name: string[], key_type: string) {
  const secureSessionId = await otpModal.open()

  return new Promise<CertificateResult>((resolve, reject) => {
    progressStatus.value = 'active'
    failureDetail.value = ''
    modalClosable.value = false
    modalVisible.value = true
    progressPercent.value = 0
    hint.value = undefined
    logContainer.value!.innerHTML = ''

    log($gettext('Getting the certificate, please wait...'))

    // With a node selected, DNS-01 can run on this main node instead, which
    // then sends the certificate to the node and keeps renewing it.
    // A certificate this instance issued for a node is renewed for it too.
    const nodeId = props.options.delegated_node_id || settings.node.id
    const onMainNode = !!props.options.delegated_node_id || (nodeId > 0
      && props.options.challenge_method === AutoCertChallengeMethod.dns01
      && (props.options.verify_on ?? 'main') === 'main')
    const endpoint = onMainNode
      ? `/api/nodes/${nodeId}/domain/${config_name}/cert`
      : `/api/domain/${config_name}/cert`
    const { ws } = useWebSocket(endpoint, false, undefined, {
      'X-Secure-Session-ID': secureSessionId,
    }, onMainNode)
    const socket = ws.value!
    let isSettled = false

    function fail(message?: string) {
      if (isSettled)
        return

      isSettled = true
      if (message)
        log(message)
      failureDetail.value = message || ''
      modalClosable.value = true
      progressStatus.value = 'exception'
      issuingCert.value = false
      reject(new Error(message || $gettext('Fail to obtain certificate')))
    }

    socket.onopen = () => {
      socket.send(JSON.stringify({
        server_name,
        ...props.options,
        key_type,
      }))
    }

    socket.onmessage = async m => {
      const r = JSON.parse(m.data)

      // The terminal error is logged once by fail(), translated.
      if (r.status !== 'error')
        log(T(r))

      switch (r.status) {
        case 'success':
          modalClosable.value = true
          issuingCert.value = false

          if (r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
            isSettled = true
            progressStatus.value = 'success'
            progressPercent.value = 100
            resolve({
              ssl_certificate: r.ssl_certificate,
              ssl_certificate_key: r.ssl_certificate_key,
              key_type: r.key_type,
              profile: r.profile,
            })
          }
          break
        case 'error':
          if (r.hint?.message)
            hint.value = r.hint
          fail(issueFailureDetail(r))
          break
        default:
          // If it is a nginx ui log, increase the percent.
          if (r.message.includes('[Nginx UI]'))
            progressPercent.value += 8
          break
      }
    }

    socket.onerror = () => {
      fail($gettext('Certificate issuance connection closed before completion.'))
    }

    socket.onclose = () => {
      fail($gettext('Certificate issuance connection closed before completion.'))
    }
  })
}

defineExpose({
  issue_cert,
})
</script>

<template>
  <div>
    <!-- One alert for the failure: the backend's hint when it has one, with the translated error below. -->
    <AAlert
      v-if="hasFailed && (hint || failureDetail)"
      class="mb-3"
      :type="hintTitle ? 'warning' : 'error'"
      show-icon
      :title="hintTitle || $gettext('Certificate issuance failed')"
      :description="failureDetail"
    />

    <AProgress
      :stroke-color="progressStrokeColor"
      :percent="progressPercent"
      :status="progressStatus"
    />

    <div
      ref="logContainer"
      class="issue-cert-log-container"
    />

    <div
      v-if="hasFailed"
      class="mt-4 flex justify-end"
    >
      <AButton
        type="primary"
        :loading="issuingCert"
        @click="emit('retry')"
      >
        {{ $gettext('Retry') }}
      </AButton>
    </div>
  </div>
</template>

<style lang="less">
.dark {
  .issue-cert-log-container {
    background-color: rgba(0, 0, 0, 0.84);
  }
}

.issue-cert-log-container {
  height: 320px;
  overflow-y: auto;
  overflow-x: hidden;
  background-color: #f3f3f3;
  border-radius: 4px;
  margin-top: 15px;
  padding: 10px;

  p {
    font-size: 12px;
    line-height: 1.5;
    margin: 10px 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
  }
}
</style>

<style scoped lang="less">

</style>
