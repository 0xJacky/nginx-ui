<script setup lang="ts">
import type { Cert } from '@/api/cert'
import dayjs from 'dayjs'
import { AutoCertState } from '@/constants'
import { certStateLabel, certStateTone } from '../certState'

const props = defineProps<{
  cert: Cert
  /** Automatic renewal can be switched for this certificate. */
  canSwitchAutoRenewal: boolean
  switching?: boolean
}>()

const emit = defineEmits<{
  switchAutoRenewal: [enabled: boolean]
}>()

const tone = computed(() => certStateTone(props.cert.state))
const stateLabel = computed(() => certStateLabel(props.cert))

const expiry = computed(() => {
  const notAfter = props.cert.certificate_info?.not_after
  return notAfter ? dayjs(notAfter).format('YYYY-MM-DD HH:mm') : '-'
})

const autoRenewalOn = computed(() => props.cert.auto_cert === AutoCertState.Enable
  || props.cert.auto_cert === AutoCertState.SelfSigned)

const autoRenewalText = computed(() => {
  const c = props.cert
  if (c.auto_cert === AutoCertState.Sync)
    return $gettext('Renewed on the node it is synced from')
  if (c.auto_cert === AutoCertState.Paused)
    return $gettext('Renewal paused')
  if (autoRenewalOn.value) {
    return c.renew_at
      ? $gettext('Renews around %{date}', { date: shortDate(c.renew_at) })
      : $gettext('On')
  }
  if (!props.canSwitchAutoRenewal)
    return $gettext('Replace the files to renew')
  return $gettext('Off')
})

const issuer = computed(() => {
  const info = props.cert.certificate_info
  if (!info)
    return '-'
  const parts = [info.issuer_organization, info.issuer_name].filter(Boolean)
  return parts.length ? [...new Set(parts)].join(' · ') : '-'
})

// The year is left out while it is the current one, which keeps the tile short.
function shortDate(value: string) {
  const date = dayjs(value)
  return date.format(date.isSame(dayjs(), 'year') ? 'MM-DD' : 'YYYY-MM-DD')
}
</script>

<template>
  <div class="cert-facts">
    <div class="cert-fact">
      <div class="cert-fact-label">
        {{ $gettext('Status') }}
      </div>
      <div class="cert-fact-value" :class="`is-${tone}`">
        <span class="cert-fact-dot" />
        <span class="truncate">{{ stateLabel }}</span>
      </div>
    </div>
    <div class="cert-fact">
      <div class="cert-fact-label">
        {{ $gettext('Expiry date') }}
      </div>
      <div class="cert-fact-value">
        {{ expiry }}
      </div>
    </div>
    <div class="cert-fact">
      <div class="cert-fact-label">
        {{ $gettext('Auto renewal') }}
      </div>
      <div class="cert-fact-value justify-between">
        <span class="min-w-0">{{ autoRenewalText }}</span>
        <ASwitch
          v-if="canSwitchAutoRenewal"
          :checked="cert.auto_cert === AutoCertState.Enable"
          :loading="switching"
          :aria-label="$gettext('Auto renewal')"
          @change="checked => emit('switchAutoRenewal', !!checked)"
        />
      </div>
    </div>
    <div class="cert-fact">
      <div class="cert-fact-label">
        {{ $gettext('Issuer') }}
      </div>
      <div class="cert-fact-value">
        <span class="truncate" :title="issuer">{{ issuer }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.cert-facts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.cert-fact {
  min-width: 0;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--ant-color-fill-quaternary);
  border: 1px solid var(--ant-color-border-secondary);
}

.cert-fact-label {
  font-size: 12px;
  color: var(--ant-color-text-secondary);
  margin-bottom: 4px;
}

.cert-fact-value {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 24px;
  font-size: 15px;
  color: var(--ant-color-text);

  &.is-success {
    color: var(--ant-color-success);
  }

  &.is-warning {
    color: var(--ant-color-warning);
  }

  &.is-error {
    color: var(--ant-color-error);
  }

  &.is-processing {
    color: var(--ant-color-primary);
  }

  &.is-default {
    color: var(--ant-color-text-secondary);
  }
}

.cert-fact-dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

@media (max-width: 991px) {
  .cert-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
