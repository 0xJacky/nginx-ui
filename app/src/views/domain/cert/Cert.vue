<script setup lang="ts">
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import IssueCert from '@/views/domain/cert/IssueCert.vue'
import ChangeCert from '@/views/domain/cert/components/ChangeCert/ChangeCert.vue'
import type { CertificateInfo } from '@/api/cert'

const props = defineProps<{
  configName: string
  enabled: boolean
  currentServerIndex: number
  certInfo?: CertificateInfo
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
    <h2>
      {{ $gettext('Certificate Status') }}
    </h2>
    <CertInfo
      :cert="certInfo"
      class="mb-4"
    />

    <ChangeCert />

    <IssueCert
      v-model:enabled="enabled"
      :config-name="configName"
    />
  </div>
</template>

<style scoped>

</style>
