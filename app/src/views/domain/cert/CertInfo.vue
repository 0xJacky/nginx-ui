<script setup lang="ts">
import dayjs from 'dayjs'
import type { CertificateInfo } from '@/api/cert'

const props = defineProps<{
  cert?: CertificateInfo
}>()

const isValid = computed(() => dayjs().isAfter(props.cert?.not_before) && dayjs().isBefore(props.cert?.not_after))
</script>

<template>
  <ACard
    v-if="cert"
    size="small"
  >
    <template #title>
      {{ cert.subject_name }}
      <ATag
        v-if="isValid"
        color="success"
        class="ml-2"
      >
        {{ $gettext('Valid') }}
      </ATag>
      <ATag
        v-else
        color="error"
        class="ml-2"
      >
        {{ $gettext('Expired') }}
      </ATag>
    </template>
    <p>
      {{ $gettext('Intermediate Certification Authorities: %{issuer}', { issuer: cert.issuer_name }) }}
    </p>
    <p>
      {{ $gettext('Expired At: %{date}', { date: dayjs(cert.not_after).format('YYYY-MM-DD HH:mm:ss').toString() }) }}
    </p>
    <p class="mb-0">
      {{ $gettext('Not Valid Before: %{date}', { date: dayjs(cert.not_before).format('YYYY-MM-DD HH:mm:ss').toString() }) }}
    </p>
  </ACard>
</template>

<style lang="less" scoped>

</style>
