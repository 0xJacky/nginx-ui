<script setup lang="ts">
import type { Cert } from '@/api/cert'
import ObtainCertLive from '@/views/site/site_edit/components/Cert/ObtainCertLive.vue'

const props = defineProps<{
  cert: Cert
}>()

const emit = defineEmits<{
  retried: []
}>()

const { message } = App.useApp()

const modalVisible = ref(false)
const modalClosable = ref(true)
const refObtainCertLive = useTemplateRef('refObtainCertLive')

function openAndRetry() {
  modalVisible.value = true
  nextTick(() => {
    refObtainCertLive.value
      ?.issue_cert(props.cert.name, props.cert.domains, props.cert.key_type)
      .then(() => {
        message.success($gettext('Certificate issued successfully'))
        emit('retried')
      })
      .catch(() => {
        // Error already surfaced inside ObtainCertLive's log.
      })
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
  <AModal
    v-model:open="modalVisible"
    :title="$gettext('Retry Certificate Issuance')"
    :mask-closable="modalClosable"
    :closable="modalClosable"
    :footer="null"
    :width="600"
    force-render
  >
    <ObtainCertLive
      ref="refObtainCertLive"
      v-model:modal-visible="modalVisible"
      v-model:modal-closable="modalClosable"
      :options="{
        name: cert.name,
        domains: cert.domains,
        key_type: cert.key_type,
        challenge_method: cert.challenge_method,
        dns_credential_id: cert.dns_credential_id,
        acme_user_id: cert.acme_user_id,
        revoke_old: cert.revoke_old,
      }"
    />
  </AModal>
</template>
