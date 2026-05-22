<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import type { Cert } from '@/api/cert'
import IssueCertModal from './IssueCertModal.vue'

const props = defineProps<{
  cert: Cert
}>()

const emit = defineEmits<{
  retried: []
}>()

const { message } = App.useApp()
const refModal = useTemplateRef('refModal')

const issueOptions = computed<AutoCertOptions>(() => ({
  name: props.cert.name,
  domains: props.cert.domains,
  key_type: props.cert.key_type,
  challenge_method: props.cert.challenge_method,
  dns_credential_id: props.cert.dns_credential_id,
  acme_user_id: props.cert.acme_user_id,
  revoke_old: props.cert.revoke_old,
}))

function openAndRetry() {
  // refModal is bound to a component rendered alongside this button,
  // so it is guaranteed to be mounted by the time @click fires.
  refModal.value!
    .start()
    .then(() => {
      message.success($gettext('Certificate issued successfully'))
      emit('retried')
    })
    .catch(() => {
      // Error already surfaced inside ObtainCertLive's log.
    })
}
</script>

<template>
  <AButton
    type="link"
    size="small"
    @click="openAndRetry"
  >
    {{ $gettext('Retry') }}
  </AButton>
  <IssueCertModal
    ref="refModal"
    :title="$gettext('Retry Certificate Issuance')"
    :options="issueOptions"
  />
</template>
