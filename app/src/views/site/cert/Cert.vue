<script setup lang="ts">
import type { Cert, CertificateInfo } from '@/api/cert'
import CertInfo from '@/views/site/cert/CertInfo.vue'
import ChangeCert from '@/views/site/cert/components/ChangeCert/ChangeCert.vue'
import IssueCert from '@/views/site/cert/IssueCert.vue'

const props = defineProps<{
  configName: string
  currentServerIndex: number
  certInfo?: CertificateInfo[]
}>()

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
      v-model:enabled="enabled"
      :config-name="configName"
    />
  </div>
</template>

<style scoped>

</style>
