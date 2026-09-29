<script setup lang="ts">
import type { Cert } from '@/api/cert'
import dayjs from 'dayjs'
import { lastCertificateLogLines, renderCertificateLog } from '../certLog'

const props = defineProps<{
  cert: Cert
}>()

const PREVIEW_LINES = 6

const expanded = ref(false)

const time = computed(() => props.cert.last_renewal_at
  ? dayjs(props.cert.last_renewal_at).format('YYYY-MM-DD HH:mm')
  : '')

const result = computed<{ color: string, text: string } | undefined>(() => {
  if (props.cert.state === 'issuing')
    return { color: 'processing', text: $gettext('In progress') }
  if (!time.value)
    return undefined
  if (props.cert.renewal_failed)
    return { color: 'error', text: $gettext('Failed · %{time}', { time: time.value }) }
  return { color: 'success', text: $gettext('Succeeded · %{time}', { time: time.value }) }
})

const preview = computed(() => lastCertificateLogLines(props.cert.log, PREVIEW_LINES))
const fullLog = computed(() => expanded.value ? renderCertificateLog(props.cert.log) : '')
</script>

<template>
  <ACard size="small" :title="$gettext('Last renewal')">
    <template #extra>
      <ATag v-if="result" :color="result.color" class="m-0">
        {{ result.text }}
      </ATag>
    </template>

    <AAlert
      v-if="cert.renewal_failed && cert.last_renewal_error"
      class="mb-3"
      type="error"
      show-icon
      :title="cert.last_renewal_error"
    />

    <template v-if="preview.length">
      <pre v-if="!expanded" class="renewal-log">{{ preview.join('\n') }}</pre>
      <pre v-else class="renewal-log is-full">{{ fullLog }}</pre>
      <AButton type="link" size="small" class="mt-2 px-0" @click="expanded = !expanded">
        {{ expanded ? $gettext('Show less') : $gettext('View full log') }}
      </AButton>
    </template>
    <p v-else class="renewal-empty">
      {{ $gettext('No renewal has run yet.') }}
    </p>
  </ACard>
</template>

<style scoped lang="less">
.renewal-log {
  margin: 0;
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--ant-color-fill-quaternary);
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;

  &.is-full {
    max-height: 480px;
    overflow-y: auto;
  }
}

.renewal-empty {
  margin: 0;
  color: var(--ant-color-text-secondary);
}
</style>
