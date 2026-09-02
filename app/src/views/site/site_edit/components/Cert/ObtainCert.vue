<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import type { CertificateResult } from '@/api/cert'
import type { PrivateKeyType } from '@/constants'
import { Modal } from 'antdv-next'
import { AutoCertChallengeMethod } from '@/api/auto_cert'
import site from '@/api/site'
import AutoCertStepOne from '@/components/AutoCertForm'
import { PrivateKeyTypeEnum } from '@/constants'
import { isIPAddress } from '@/utils/certificate'
import { useTLSDirectives } from '../../composables/useTLSDirectives'
import { useSiteEditorStore } from '../SiteEditor/store'
import ObtainCertLive from './ObtainCertLive.vue'

const props = defineProps<{
  configName: string
  noServerName?: boolean
}>()

const editorStore = useSiteEditorStore()
const { message } = useGlobalApp()
const { ngxConfig, issuingCert, curDirectivesMap, isDefaultServer, hasWildcardServerName, certificateIdentifiers, needsManualIpInput } = storeToRefs(editorStore)

const autoCert = defineModel<boolean>('autoCert')

const modalVisible = ref(false)
const step = ref(1)

const [modal, ContextHolder] = Modal.useModal()

const data = ref({
  domains: [],
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
const manualIpAddress = ref('')

const requestIdentifiers = computed(() => {
  if (needsManualIpInput.value) {
    const manualIdentifier = manualIpAddress.value.trim()
    return manualIdentifier ? [manualIdentifier] : []
  }

  return [...certificateIdentifiers.value]
})

const isIpCertificate = computed(() => requestIdentifiers.value.some(isIPAddress))

const { ensureTLSDirectives } = useTLSDirectives()

function issueCert() {
  const live = refObtainCertLive.value
  if (!live) {
    modalClosable.value = true
    issuingCert.value = false
    message.error($gettext('Certificate issuance component is not ready'))
    return
  }

  live.issue_cert(
    props.configName,
    data.value.domains,
    data.value.key_type,
  ).then(resolveCert).catch(() => {
    // The live log already shows the issuance failure details.
    modalClosable.value = true
    issuingCert.value = false
  })
}

async function resolveCert({ ssl_certificate, ssl_certificate_key, key_type, profile }: CertificateResult) {
  data.value.profile = profile
  ensureTLSDirectives(ssl_certificate, ssl_certificate_key)
  await editorStore.save()
  changeAutoCert(true, key_type)
  autoCert.value = true
}

function changeAutoCert(status: boolean, key_type?: PrivateKeyType) {
  if (status) {
    site.add_auto_cert(props.configName, {
      domains: data.value.domains,
      challenge_method: data.value.challenge_method!,
      profile: data.value.profile,
      dns_credential_id: data.value.dns_credential_id!,
      key_type: key_type!,
      acme_user_id: data.value.acme_user_id,
      must_staple: data.value.must_staple,
      lego_disable_cname_support: data.value.lego_disable_cname_support,
      disable_authoritative_ns_propagation: data.value.disable_authoritative_ns_propagation,
      enable_common_name: data.value.enable_common_name,
      revoke_old: data.value.revoke_old,
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
    step.value = 1
    manualIpAddress.value = ''
    data.value.domains = [...certificateIdentifiers.value]
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
  else if (requestIdentifiers.value.length === 0) {
    return false
  }
  else if (needsManualIpInput.value && !isIPAddress(manualIpAddress.value)) {
    return false
  }
  else if (data.value.challenge_method === AutoCertChallengeMethod.http01) {
    return true
  }
  else if (data.value.challenge_method === AutoCertChallengeMethod.dns01) {
    return Boolean(data.value.dns_credential_id)
  }
  return false
})

async function next() {
  try {
    await refAutoCertForm.value?.validateManualIpAddress()
  }
  catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
    return
  }

  data.value.domains = [...requestIdentifiers.value]
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
          v-model:manual-ip-address="manualIpAddress"
          :no-server-name="noServerName"
          :is-default-server="isDefaultServer"
          :has-wildcard-server-name="hasWildcardServerName"
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
