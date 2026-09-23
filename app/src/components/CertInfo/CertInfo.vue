<script setup lang="ts">
import type { CertificateInfo } from '@/api/cert'
import { CopyOutlined } from '@antdv-next/icons'
import dayjs from 'dayjs'
import { useClipboard } from '@vueuse/core'

const props = defineProps<{
  cert?: CertificateInfo
  sslCertificatePath?: string
  sslCertificateKeyPath?: string
}>()

const isValid = computed(() => dayjs().isAfter(props.cert?.not_before) && dayjs().isBefore(props.cert?.not_after))
const sanAliases = computed(() => props.cert?.subject_alt_names ?? [])

const { message } = App.useApp()
const { copy } = useClipboard()

async function copyToClipboard(text: string, label: string) {
  if (!text) {
    message.warning($gettext('Nothing to copy'))
    return
  }
  try {
    await copy(text)
    message.success($gettext(`{label} copied to clipboard`).replace('{label}', label))
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to copy to clipboard'))
  }
}
</script>

<template>
  <ACard
    v-if="cert"
    size="small"
    :styles="{ body: { padding: '12px' } }"
  >
    <template #title>
      {{ $ngettext('Certificate Status', 'Certificates Status', 1) }}
    </template>
    <template #extra>
      <slot name="extra" />
    </template>
    <div class="name-with-copy">
      <div class="name-primary">
        <p class="mb-0 name-text">
          {{ $gettext('Name: %{name}', { name: cert.subject_name }) }}
        </p>
        <AButton
          v-if="cert.subject_name"
          type="text"
          size="small"
          @click="copyToClipboard(cert.subject_name, $gettext('Name'))"
        >
          <CopyOutlined />
        </AButton>
      </div>
      <div class="name-extra-actions">
        <slot name="name-extra" />
      </div>
    </div>
    <p>
      {{ $gettext('Status:') }}
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
    </p>
    <p v-if="sanAliases.length > 0" class="break-words">
      {{ $gettext('SAN Aliases: %{aliases}', { aliases: sanAliases.join(', ') }) }}
    </p>
    <p>
      {{ $gettext('Issuer: %{issuer}', { issuer: cert.issuer_name }) }}
    </p>
    <p>
      {{ $gettext('Expires At: %{date}', { date: dayjs(cert.not_after).format('YYYY-MM-DD HH:mm:ss').toString() }) }}
    </p>
    <p class="mb-0">
      {{ $gettext('Not Valid Before: %{date}', { date: dayjs(cert.not_before).format('YYYY-MM-DD HH:mm:ss').toString() }) }}
    </p>

    <div v-if="sslCertificatePath" class="path-with-copy mt-2">
      <p class="mb-0 break-all">
        {{ $gettext('SSL Certificate Path') }}: {{ sslCertificatePath }}
      </p>
      <AButton
        type="text"
        size="small"
        @click="copyToClipboard(sslCertificatePath, $gettext('SSL Certificate Path'))"
      >
        <CopyOutlined />
      </AButton>
    </div>

    <div v-if="sslCertificateKeyPath" class="path-with-copy mt-1">
      <p class="mb-0 break-all">
        {{ $gettext('SSL Certificate Key Path') }}: {{ sslCertificateKeyPath }}
      </p>
      <AButton
        type="text"
        size="small"
        @click="copyToClipboard(sslCertificateKeyPath, $gettext('SSL Certificate Key Path'))"
      >
        <CopyOutlined />
      </AButton>
    </div>

  </ACard>
</template>

<style scoped lang="less">
.name-with-copy {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.name-text {
  margin-right: 2px;
}

.name-primary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.name-extra-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.path-with-copy {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

</style>
