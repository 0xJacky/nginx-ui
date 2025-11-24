<script setup lang="ts">
import type { Cert } from '@/api/cert'
import AutoCertForm from '@/components/AutoCertForm'
import CertInfo from '@/components/CertInfo'
import RenewCert from './RenewCert.vue'

interface Props {
  data: Cert
  isManaged: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  renewed: []
}>()

// Use defineModel for two-way binding
const data = defineModel<Cert>('data', { required: true })

function handleRenewed() {
  emit('renewed')
}
</script>

<template>
  <div class="auto-cert-management">
    <!-- Auto Cert Status Alerts -->
    <div
      v-if="isManaged"
      class="mb-4"
    >
      <div class="mb-2">
        <AAlert
          :message="$gettext('This certificate is managed by Nginx UI')"
          type="success"
          show-icon
        />
      </div>
      <div
        v-if="!data.filename"
        class="mt-4 mb-4"
      >
        <AAlert
          :message="$gettext('This Auto Cert item is invalid, please remove it.')"
          type="error"
          show-icon
        />
      </div>
      <div
        v-else-if="!data.domains"
        class="mt-4 mb-4"
      >
        <AAlert
          :message="$gettext('Domains list is empty, try to reopen Auto Cert for %{config}', { config: data.filename })"
          type="error"
          show-icon
        />
      </div>
    </div>

    <!-- Certificate Status -->
    <AForm
      v-if="data.certificate_info"
      layout="vertical"
    >
      <AFormItem :label="$gettext('Certificate Status')">
        <CertInfo
          :cert="data.certificate_info"
          class="max-w-96"
        />
      </AFormItem>
    </AForm>

    <!-- Auto Cert Management -->
    <template v-if="isManaged">
      <RenewCert
        :options="{
          name: data.name,
          domains: data.domains,
          key_type: data.key_type,
          challenge_method: data.challenge_method,
          dns_credential_id: data.dns_credential_id,
          acme_user_id: data.acme_user_id,
          revoke_old: data.revoke_old,
        }"
        @renewed="handleRenewed"
      />

      <AutoCertForm
        v-model:options="data"
        key-type-read-only
        style="max-width: 600px"
        hide-note
      />
    </template>
  </div>
</template>

<style scoped lang="less">

</style>
