<script setup lang="ts">
import type { Cert } from '@/api/cert'
import AutoCertForm from '@/components/AutoCertForm'
import CertInfo from '@/components/CertInfo'
import CertificateDownload from './CertificateDownload.vue'
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
      <div
        v-if="!data.filename"
        class="mt-4 mb-4"
      >
        <AAlert
          :title="$gettext('This Auto Cert item is invalid, please remove it.')"
          type="error"
          show-icon
        />
      </div>
      <div
        v-else-if="!data.domains"
        class="mt-4 mb-4"
      >
        <AAlert
          :title="$gettext('Domains list is empty, try to reopen Auto Cert for %{config}', { config: data.filename })"
          type="error"
          show-icon
        />
      </div>
    </div>

    <div class="auto-cert-layout">
      <AForm
        v-if="data.certificate_info"
        layout="vertical"
        class="mb-0 status-form"
      >
        <AFormItem>
          <CertInfo
            :cert="data.certificate_info"
            :ssl-certificate-path="data.ssl_certificate_path"
            :ssl-certificate-key-path="data.ssl_certificate_key_path"
            class="status-card"
          >
            <template #name-extra>
              <CertificateDownload :data="data" inline />
            </template>
            <template #extra>
              <RenewCert
                v-if="isManaged"
                :options="{
                  name: data.filename || data.name,
                  domains: data.domains,
                  key_type: data.key_type,
                  challenge_method: data.challenge_method,
                  profile: data.profile,
                  dns_credential_id: data.dns_credential_id,
                  acme_user_id: data.acme_user_id,
                  must_staple: data.must_staple,
                  lego_disable_cname_support: data.lego_disable_cname_support,
                  disable_authoritative_ns_propagation: data.disable_authoritative_ns_propagation,
                  enable_common_name: data.enable_common_name,
                  revoke_old: data.revoke_old,
                }"
                @renewed="handleRenewed"
              />
            </template>
          </CertInfo>
        </AFormItem>
      </AForm>

      <div v-if="isManaged" class="managed-actions">
        <AutoCertForm
          v-model:options="data"
          key-type-read-only
          class="settings-form"
          hide-note
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.status-card {
  max-width: 100%;
}

.status-form {
  width: 100%;
}

.status-card :deep(.mb-6) {
  margin-bottom: 0 !important;
}

.managed-actions {
  width: 100%;
}

.settings-form {
  width: 100%;
}

</style>
