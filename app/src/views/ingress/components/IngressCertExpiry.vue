<script setup lang="ts">
import type { Ingress } from '@/api/ingress'
import { formatDate, formatDateTime } from '@/lib/helper'
import { certExpiry } from '../ingressStatus'

const props = defineProps<{
  ingress: Ingress
}>()

const expiry = computed(() => certExpiry(props.ingress))

const colors = {
  none: 'default',
  pending: 'processing',
  valid: 'success',
  expiring: 'warning',
  expired: 'error',
} as const

const text = computed(() => {
  const { state, notAfter, daysLeft } = expiry.value
  switch (state) {
    case 'none':
      return $gettext('HTTP only')
    case 'pending':
      return $gettext('Not issued yet')
    case 'expired':
      return $gettext('Expired %{date}', {
        date: formatDate(notAfter!),
      })
    default:
      return $ngettext('%{date} (%{days} day left)', '%{date} (%{days} days left)', daysLeft!, {
        date: formatDate(notAfter!),
        days: String(daysLeft),
      })
  }
})
</script>

<template>
  <span
    v-if="expiry.state === 'none'"
    class="text-gray-500 dark:text-gray-400"
  >{{ text }}</span>
  <ATag
    v-else
    :color="colors[expiry.state]"
    class="me-0"
    :title="expiry.notAfter ? formatDateTime(expiry.notAfter) : undefined"
    data-testid="ingress-cert-expiry"
  >
    {{ text }}
  </ATag>
</template>
