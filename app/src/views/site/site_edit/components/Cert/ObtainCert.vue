<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import type { CertificateResult } from '@/api/cert'
import type { PrivateKeyType } from '@/constants'
import { Modal } from 'ant-design-vue'
import { AutoCertChallengeMethod } from '@/api/auto_cert'
import site from '@/api/site'
import AutoCertStepOne from '@/components/AutoCertForm'
import { PrivateKeyTypeEnum } from '@/constants'
import { useSiteEditorStore } from '../SiteEditor/store'
import ObtainCertLive from './ObtainCertLive.vue'

const props = defineProps<{
  configName: string
  noServerName?: boolean
}>()

const editorStore = useSiteEditorStore()
const { message } = useGlobalApp()
const { ngxConfig, issuingCert, curServerDirectives, curDirectivesMap, isDefaultServer, hasWildcardServerName, hasExplicitIpAddress, isIpCertificate, needsManualIpInput } = storeToRefs(editorStore)

const autoCert = defineModel<boolean>('autoCert')

const modalVisible = ref(false)
const step = ref(1)

const [modal, ContextHolder] = Modal.useModal()

const data = ref({
  dns_credential_id: null,
  challenge_method: AutoCertChallengeMethod.http01,
  code: '',
  configuration: {
    credentials: {},
    additional: {},
  },
  key_type: PrivateKeyTypeEnum.P256,
}) as Ref<AutoCertOptions>

const modalClosable = ref(true)

const name = computed(() => {
  return curDirectivesMap.value.server_name[0].params.trim()
})

const refObtainCertLive = useTemplateRef('refObtainCertLive')
const refAutoCertForm = useTemplateRef('refAutoCertForm')

function hasTLSListen(params: string) {
  return params.includes('443') && params.includes('ssl')
}

function ensureDirective(directive: string, params: string, insertIndex?: number) {
  if (!curServerDirectives.value)
    curServerDirectives.value = []

  const existingDirective = curServerDirectives.value.find(v => v.directive === directive)

  if (existingDirective) {
    existingDirective.params = params
    return
  }

  const directiveItem = { directive, params }

  if (insertIndex === undefined || insertIndex < 0 || insertIndex > curServerDirectives.value.length) {
    curServerDirectives.value.push(directiveItem)
    return
  }

  curServerDirectives.value.splice(insertIndex, 0, directiveItem)
}

function ensureTLSDirectives(sslCertificate: string, sslCertificateKey: string) {
  if (!curServerDirectives.value)
    curServerDirectives.value = []

  const hasIPv4TLSListen = curServerDirectives.value.some(v => v.directive === 'listen' && hasTLSListen(v.params) && !v.params.includes('[::]'))
  const hasIPv6TLSListen = curServerDirectives.value.some(v => v.directive === 'listen' && hasTLSListen(v.params) && v.params.includes('[::]'))

  if (!hasIPv6TLSListen) {
    curServerDirectives.value.splice(0, 0, {
      directive: 'listen',
      params: '[::]:443 ssl',
    })
  }

  if (!hasIPv4TLSListen) {
    curServerDirectives.value.splice(0, 0, {
      directive: 'listen',
      params: '443 ssl',
    })
  }

  const serverNameIdx = curDirectivesMap.value.server_name?.[0]?.idx ?? (curServerDirectives.value.length - 1)

  ensureDirective('ssl_certificate', sslCertificate, serverNameIdx + 1)

  const sslCertificateIndex = curServerDirectives.value.findIndex(v => v.directive === 'ssl_certificate')
  ensureDirective('ssl_certificate_key', sslCertificateKey, sslCertificateIndex + 1)
}

function issueCert() {
  refObtainCertLive.value?.issue_cert(
    props.configName,
    name.value.trim().split(' '),
    data.value.key_type,
  ).then(resolveCert)
}

async function resolveCert({ ssl_certificate, ssl_certificate_key, key_type }: CertificateResult) {
  ensureTLSDirectives(ssl_certificate, ssl_certificate_key)
  await editorStore.save()
  changeAutoCert(true, key_type)
  autoCert.value = true
}

function changeAutoCert(status: boolean, key_type?: PrivateKeyType) {
  if (status) {
    site.add_auto_cert(props.configName, {
      domains: name.value.trim().split(' '),
      challenge_method: data.value.challenge_method!,
      dns_credential_id: data.value.dns_credential_id!,
      key_type: key_type!,
      acme_user_id: data.value.acme_user_id,
    }).then(() => {
      message.success($gettext('Auto-renewal enabled for %{name}', { name: name.value }))
    }).catch(e => {
      message.error(e.message ?? $gettext('Enable auto-renewal failed for %{name}', { name: name.value }))
    })
  }
  else {
    site.remove_auto_cert(props.configName).then(() => {
      message.success($gettext('Auto-renewal disabled for %{name}', { name: name.value }))
    }).catch(e => {
      message.error(e.message ?? $gettext('Disable auto-renewal failed for %{name}', { name: name.value }))
    })
  }
}

async function onchange(status: boolean) {
  if (status) {
    job()
  }
  else {
    ngxConfig.value.servers.forEach(v => {
      v.locations = v?.locations?.filter(l => l.path !== '/.well-known/acme-challenge')
    })
    // Skip syncing the response so handleResponse() does not overwrite
    // our local autoCert back to the backend's still-enabled state, which
    // would leave the switch showing on until a page reload.
    await editorStore.save({ syncResponse: false })
    changeAutoCert(status)
  }

  autoCert.value = status
}

async function job() {
  modalClosable.value = false
  issuingCert.value = true

  if (props.noServerName) {
    message.error($gettext('server_name not found in directives'))
    issuingCert.value = false

    return
  }

  // Wait for Vue to mount ObtainCertLive after step transitions to 2; without
  // this tick refObtainCertLive.value is still null and issueCert() silently
  // no-ops via its optional-chain call.
  await nextTick()

  issueCert()
}
function toggle(status: boolean) {
  if (status) {
    modal.confirm({
      title: $gettext('Do you want to disable auto-cert renewal?'),
      content: $gettext('We will remove the HTTPChallenge configuration from '
        + 'this file and reload the Nginx. Are you sure you want to continue?'),
      okText: $gettext('OK'),
      cancelText: $gettext('Cancel'),
      mask: false,
      centered: true,
      onOk() {
        onchange(false)
      },
    })
  }
  else {
    modalVisible.value = true
    modalClosable.value = true
  }
}

defineExpose({
  toggle,
})

const canNext = computed(() => {
  if (step.value === 2) {
    return false
  }
  else if (data.value.challenge_method === AutoCertChallengeMethod.http01) {
    return true
  }
  else if (data.value.challenge_method === AutoCertChallengeMethod.dns01) {
    return data.value?.code ?? false
  }
  return false
})

function next() {
  // Apply manual IP address to domains before proceeding
  refAutoCertForm.value?.applyManualIpToDomains()

  step.value++
  onchange(true)
}
</script>

<template>
  <div>
    <ContextHolder />
    <AModal
      v-model:open="modalVisible"
      :title="$gettext('Obtain certificate')"
      :mask-closable="modalClosable"
      :footer="null"
      :closable="modalClosable"
      :width="600"
      force-render
    >
      <template v-if="step === 1">
        <AutoCertStepOne
          ref="refAutoCertForm"
          v-model:options="data"
          :no-server-name="noServerName"
          :is-default-server="isDefaultServer"
          :has-wildcard-server-name="hasWildcardServerName"
          :has-explicit-ip-address="hasExplicitIpAddress"
          :is-ip-certificate="isIpCertificate"
          :needs-manual-ip-input="needsManualIpInput"
        />
      </template>
      <template v-else-if="step === 2">
        <ObtainCertLive
          ref="refObtainCertLive"
          v-model:modal-closable="modalClosable"
          v-model:modal-visible="modalVisible"
          :options="data"
        />
      </template>
      <div
        v-if="canNext"
        class="control-btn"
      >
        <AButton
          type="primary"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
      </div>
    </AModal>
  </div>
</template>

<style lang="less" scoped>
.control-btn {
  display: flex;
  justify-content: flex-end;
}
</style>
