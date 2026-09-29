<script setup lang="ts">
import type { PluginInfo } from '@/api/plugin'
import { InfoCircleOutlined } from '@antdv-next/icons'
import {
  packageTrustPreset,
  trustedOfferAction,
  trustedOfferSummary,
  unsignedExplanation,
} from './marketplace/trust'
import { useReplacePlugin, useTrustedOffer } from './replace'

const props = defineProps<{
  plugin: PluginInfo
}>()

const trust = computed(() => packageTrustPreset(props.plugin.trust))
const isUnsigned = computed(() => props.plugin.trust === 'unsigned')
const offer = useTrustedOffer(() => props.plugin)
const { replacingId, confirmReplace } = useReplacePlugin()

// Unsigned always explains itself, other levels only when there is a better package.
const hasPopover = computed(() => isUnsigned.value || Boolean(offer.value))
const explanation = computed(() => (isUnsigned.value ? unsignedExplanation() : trust.value?.hint() ?? ''))
const color = computed(() => (isUnsigned.value ? 'warning' : trust.value?.color))
</script>

<template>
  <span v-if="trust" class="inline-flex" @click.stop @keydown.enter.stop>
    <APopover
      v-if="hasPopover"
      :trigger="['hover', 'click']"
      :title="trust.label()"
      placement="bottomLeft"
    >
      <template #content>
        <div class="trust-popover">
          <p class="trust-popover-text">
            {{ explanation }}
            <template v-if="offer">
              {{ trustedOfferSummary(offer) }}
            </template>
          </p>
          <AButton
            v-if="offer"
            type="primary"
            size="small"
            :loading="replacingId === props.plugin.id"
            @click="confirmReplace(offer)"
          >
            {{ trustedOfferAction(offer) }}
          </AButton>
        </div>
      </template>
      <ATag :color="color" class="m-0 cursor-help" variant="outlined">
        {{ trust.label() }}
        <InfoCircleOutlined class="ms-1" />
      </ATag>
    </APopover>
    <ATooltip v-else :title="trust.hint()">
      <ATag :color="color" class="m-0" variant="outlined">
        {{ trust.label() }}
      </ATag>
    </ATooltip>
  </span>
</template>

<style lang="less" scoped>
.trust-popover {
  max-width: 300px;
}

.trust-popover-text {
  margin: 0 0 10px;
  line-height: 1.6;
  color: var(--ant-color-text-secondary);

  &:last-child {
    margin-bottom: 0;
  }
}
</style>
