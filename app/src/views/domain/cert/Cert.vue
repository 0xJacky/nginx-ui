<script setup lang="ts">
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import IssueCert from '@/views/domain/cert/IssueCert.vue'
import ChangeCert from '@/views/domain/cert/components/ChangeCert/ChangeCert.vue'
import type { Cert, CertificateInfo } from '@/api/cert'

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
