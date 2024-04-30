<script setup lang="ts">
import { computed } from 'vue'
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import IssueCert from '@/views/domain/cert/IssueCert.vue'
import ChangeCert from '@/views/domain/cert/ChangeCert.vue'
import type { CertificateInfo } from '@/api/cert'

const props = defineProps<{
  configName: string
  enabled: boolean
  currentServerIndex: number
  certInfo?: CertificateInfo
}>()

const emit = defineEmits(['callback', 'update:enabled'])
function callback() {
  emit('callback')
}

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
      @callback="callback"
    />
  </div>
</template>

<style scoped>

</style>
