<script setup lang="ts">
import type { Cert } from '@/api/cert'
import ChangeCert from '@/views/site/site_edit/components/Cert/ChangeCert.vue'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

function handleCertChange(certs: Cert[]) {
  if (certs.length > 0 && data.value?.server) {
    data.value.server.ssl_cert = certs[0].ssl_certificate_path
    data.value.server.ssl_key = certs[0].ssl_certificate_key_path
  }
}
</script>

<template>
  <AForm v-if="data?.server" layout="vertical">
    <AFormItem :label="$gettext('Host')">
      <p>{{ data.server.host }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Port')">
      <p>{{ data.server.port }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Run Mode')">
      <p>{{ data.server.run_mode }}</p>
    </AFormItem>

    <!-- HTTPS Settings -->
    <AFormItem :label="$gettext('Enable HTTPS')">
      <ASwitch v-model:checked="data.server.enable_https" />
    </AFormItem>

    <div v-if="data.server.enable_https">
      <ChangeCert class="mb-6" selection-type="radio" @change="handleCertChange" />

      <AFormItem :label="$gettext('SSL Certificate Path')">
        <p>{{ data.server.ssl_cert }}</p>
      </AFormItem>

      <AFormItem :label="$gettext('SSL Key Path')">
        <p>{{ data.server.ssl_key }}</p>
      </AFormItem>
    </div>
  </AForm>
</template>

<style lang="less" scoped></style>
