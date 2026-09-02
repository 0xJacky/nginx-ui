<script setup lang="ts">
import type { CheckGroup, CheckRow } from './checks'
import type { VerifyResult } from '@/api/host_setup'
import { SafetyCertificateOutlined } from '@ant-design/icons-vue'
import { computed, onDeactivated, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import CheckResults from './CheckResults.vue'
import { hasBlockingFailure, toCheckRows } from './checks'
import { useHostSetupWizard } from './useHostSetupWizard'
import { useLatestRequest } from './useLatestRequest'

const props = defineProps<{
  groups: CheckGroup[]
  title: string
  hint?: string
  disabled?: boolean
  /** Lists only these checks. A failed ssh_connect is always listed. */
  checkKeys?: string[]
  /** Heading of the alert shown when the request itself fails. */
  errorTitle?: string
  /** Renders the alerts and results below the card instead of inside it. */
  resultsOutsideCard?: boolean
}>()

const passed = defineModel<boolean>('passed', { default: false })
const { params } = useHostSetupWizard()

const result = ref<VerifyResult | null>(null)
const { error: runError, invalidate, isLoading: running, reset: resetRequest, run: runRequest } = useLatestRequest()

const allRows = computed(() => toCheckRows(result.value))
const rows = computed(() => allRows.value.filter(isListed))

// SSH connectivity is a prerequisite of every group rather than one of its
// results, so it only appears when it failed.
function isListed(row: CheckRow) {
  if (row.key === 'ssh_connect')
    return row.level === 'error'
  return !props.checkKeys || props.checkKeys.includes(row.key)
}

function hasFailed(key: string) {
  return rows.value.some(row => row.key === key && row.level === 'error')
}

function reset() {
  resetRequest()
  result.value = null
  passed.value = false
}

watch(params, reset, { deep: true })

async function run(options: { skipNginxT?: boolean } = {}) {
  reset()
  await runRequest(
    () => hostSetup.verify({ ...params.value }, { ...options, groups: props.groups }),
    {
      onSuccess: response => {
        result.value = response
        passed.value = !hasBlockingFailure(allRows.value)
      },
    },
  )
}

onDeactivated(invalidate)
</script>

<template>
  <div class="space-y-4">
    <ACard size="small" :title="title">
      <template #extra>
        <slot name="action" :run="run" :running="running" :result="result">
          <AButton size="small" :loading="running" :disabled="disabled" @click="run()">
            <SafetyCertificateOutlined />
            {{ result ? $gettext('Check again') : $gettext('Run checks') }}
          </AButton>
        </slot>
      </template>

      <ASpace direction="vertical" size="middle" class="w-full">
        <ATypographyText v-if="hint" type="secondary" class="text-xs">
          {{ hint }}
        </ATypographyText>

        <template v-if="!resultsOutsideCard">
          <AAlert
            v-if="runError"
            type="error"
            show-icon
            :message="errorTitle ?? $gettext('Could not run the checks')"
            :description="runError"
          />

          <CheckResults :rows="rows" />
        </template>
      </ASpace>
    </ACard>

    <template v-if="resultsOutsideCard">
      <slot
        :run="run"
        :running="running"
        :result="result"
        :run-error="runError"
        :has-failed="hasFailed"
      />

      <AAlert
        v-if="runError"
        type="error"
        show-icon
        :message="errorTitle ?? $gettext('Could not run the checks')"
        :description="runError"
      />

      <CheckResults :rows="rows" />
    </template>
  </div>
</template>
