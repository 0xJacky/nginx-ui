<script setup lang="ts">
import type { Ingress } from '@/api/ingress'
import { failingConditions, isGenerationBehind, phaseColor } from '../ingressStatus'
import { conditionLabel, phaseLabel } from './labels'

const props = defineProps<{
  ingress: Ingress
}>()

const phase = computed(() => props.ingress.status?.phase || 'Pending')

// The tooltip names what keeps the ingress from being ready.
const reasons = computed(() => failingConditions(props.ingress).map(condition => {
  const detail = [condition.reason, condition.message].filter(Boolean).join(': ')
  return `${conditionLabel(condition.type)} — ${detail || $gettext('Not ready')}`
}))

const tooltip = computed(() => {
  const lines = [...reasons.value]
  if (isGenerationBehind(props.ingress))
    lines.unshift($gettext('The latest spec has not been applied yet'))
  return lines.join('\n')
})
</script>

<template>
  <ATooltip
    :title="tooltip || undefined"
    :styles="{ root: { maxWidth: '420px' }, container: { maxWidth: '420px', whiteSpace: 'pre-line' } }"
  >
    <ATag
      :color="phaseColor(phase)"
      class="me-0"
      data-testid="ingress-phase"
      :data-phase="phase"
    >
      {{ phaseLabel(phase) }}
    </ATag>
  </ATooltip>
</template>
