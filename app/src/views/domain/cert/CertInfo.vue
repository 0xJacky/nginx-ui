<script setup lang="ts">
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { useGettext } from 'vue3-gettext'
import type { CertificateInfo } from '@/api/cert'

defineProps<{
  cert?: CertificateInfo
}>()

const { $gettext } = useGettext()

const now = computed(() => new Date().toISOString())
</script>

<template>
  <div
    v-if="cert"
    class="cert-info pb-4"
  >
    <p>
      {{ $gettext('Intermediate Certification Authorities: %{issuer}', { issuer: cert.issuer_name }) }}
    </p>
    <p>
      {{ $gettext('Subject Name: %{subject}', { subject: cert.subject_name }) }}
    </p>
    <p>
      {{ $gettext(' Expiration Date: %{date}', { date: dayjs(cert.not_after).format('YYYY-MM-DD HH:mm:ss').toString() }) }}
    </p>
    <p>
      {{ $gettext('Not Valid Before: %{date}', { date: dayjs(cert.not_before).format('YYYY-MM-DD HH:mm:ss').toString() }) }}
    </p>
    <div class="status">
      <template v-if="dayjs(now).isBefore(cert.not_before) || dayjs(now).isAfter(cert.not_after)">
        <CloseCircleOutlined class="text-red-600" />
        <span class="ml-2">{{ $gettext('Certificate has expired') }}</span>
      </template>
      <template v-else>
        <CheckCircleOutlined class="text-green-500" />
        <span class="ml-2">{{ $gettext('Certificate is valid') }}</span>
      </template>
    </div>
  </div>
</template>

<style lang="less" scoped>

</style>
