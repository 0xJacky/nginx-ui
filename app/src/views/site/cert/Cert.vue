<script setup lang="ts">
import type { Cert, CertificateInfo } from '@/api/cert'
import type { NgxDirective } from '@/api/ngx'
import CertInfo from '@/views/site/cert/CertInfo.vue'
import ChangeCert from '@/views/site/cert/components/ChangeCert/ChangeCert.vue'
import IssueCert from '@/views/site/cert/IssueCert.vue'

const props = defineProps<{
  configName: string
  currentServerIndex: number
  certInfo?: CertificateInfo[]
  siteEnabled: boolean
}>()

const current_server_directives = defineModel<NgxDirective[]>('current_server_directives')

const enabled = defineModel<boolean>('enabled', {
  default: () => false,
})

const changedCerts: Ref<Cert[]> = ref([])

// if certInfo update, clear changedCerts
watch(() => props.certInfo, () => {
  changedCerts.value = []
})

function handleCertChange(certs: Cert[]) {
  changedCerts.value = certs

  // Update NgxDirective
  if (current_server_directives.value) {
    // Filter out existing certificate configurations
    const filteredDirectives = current_server_directives.value
      .filter(v => v.directive !== 'ssl_certificate' && v.directive !== 'ssl_certificate_key')

    // Add new certificate configuration
    const newDirectives = [...filteredDirectives]

    certs.forEach(cert => {
      newDirectives.push({
        directive: 'ssl_certificate',
        params: cert.ssl_certificate_path,
      })
      newDirectives.push({
        directive: 'ssl_certificate_key',
        params: cert.ssl_certificate_key_path,
      })
    })

    // Update directives
    current_server_directives.value = newDirectives
  }
}
</script>

<template>
  <div>
    <h3>
      {{ $ngettext('Certificate Status', 'Certificates Status', certInfo?.length || 1) }}
    </h3>

    <ARow
      :gutter="[16, 16]"
      class="mb-4"
    >
      <ACol
        v-for="(c, index) in certInfo"
        :key="index"
        :xs="24"
        :sm="12"
      >
        <CertInfo :cert="c" />
      </ACol>
    </ARow>

    <template v-if="changedCerts.length > 0">
      <h3>
        {{ $ngettext('Changed Certificate', 'Changed Certificates', changedCerts?.length || 1) }}
      </h3>
      <ARow
        :gutter="[16, 16]"
        class="mb-4"
      >
        <ACol
          v-for="(c, index) in changedCerts"
          :key="index"
          :xs="24"
          :sm="12"
        >
          <CertInfo :cert="c.certificate_info" />
        </ACol>
      </ARow>
    </template>

    <ChangeCert @change="handleCertChange" />

    <IssueCert
      v-if="siteEnabled"
      v-model:enabled="enabled"
      :config-name="configName"
    />
  </div>
</template>

<style scoped>

</style>
