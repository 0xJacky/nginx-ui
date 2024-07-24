<script setup lang="ts">
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import IssueCert from '@/views/domain/cert/IssueCert.vue'
import ChangeCert from '@/views/domain/cert/components/ChangeCert/ChangeCert.vue'
import type { CertificateInfo } from '@/api/cert'

const props = defineProps<{
  configName: string
  enabled: boolean
  currentServerIndex: number
  certInfo?: CertificateInfo[]
}>()

const emit = defineEmits(['update:enabled'])

const enabled = computed({
  get() {
    return props.enabled
  },
  set(value) {
    emit('update:enabled', value)
  },
})
</script>

<template>
  <div>
    <h3>
      {{ $gettext('Certificate Status') }}
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

    <ChangeCert />

    <IssueCert
      v-model:enabled="enabled"
      :config-name="configName"
    />
  </div>
</template>

<style scoped>

</style>
