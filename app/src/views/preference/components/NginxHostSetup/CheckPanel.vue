<script setup lang="ts">
import type { CheckGroup } from './checks'
import type { VerifyResult } from '@/api/host_setup'
import { SafetyCertificateOutlined } from '@ant-design/icons-vue'
import { computed, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'
import CheckResults from './CheckResults.vue'
import { hasBlockingFailure, toCheckRows } from './checks'
import { useHostSetupWizard } from './useHostSetupWizard'

const props = defineProps<{
  group: CheckGroup
  title: string
  hint?: string
  /** Disable the button while the step is not ready to be checked. */
  disabled?: boolean
}>()

const emit = defineEmits<{ failed: [] }>()

const passed = defineModel<boolean>('passed', { default: false })

const { params } = useHostSetupWizard()

const result = ref<VerifyResult | null>(null)
const runError = ref('')
const running = ref(false)
// A slow response must not report on params the user has since changed.
let runID = 0

const rows = computed(() => toCheckRows(result.value))

function reset() {
  runID++
  result.value = null
  runError.value = ''
  passed.value = false
}

watch(params, reset, { deep: true })

async function run() {
  reset()
  const currentRun = ++runID
  running.value = true
  try {
    const response = await hostSetup.verify(params.value, { groups: [props.group] })
    if (currentRun !== runID)
      return
    result.value = response
    passed.value = !hasBlockingFailure(toCheckRows(response))
    if (!passed.value)
      emit('failed')
  }
  catch (error) {
    if (currentRun !== runID)
      return
    runError.value = getErrorMessage(error)
  }
  finally {
    running.value = false
  }
}

defineExpose({ run })
</script>

<template>
  <ACard size="small" :title="title">
    <template #extra>
      <AButton size="small" :loading="running" :disabled="disabled" @click="run">
        <SafetyCertificateOutlined />
        {{ result ? $gettext('Check again') : $gettext('Run checks') }}
      </AButton>
    </template>

    <ASpace direction="vertical" size="middle" class="w-full">
      <ATypographyText v-if="hint" type="secondary" class="text-xs">
        {{ hint }}
      </ATypographyText>

      <AAlert
        v-if="runError"
        type="error"
        show-icon
        :message="$gettext('Could not run the checks')"
        :description="runError"
      />

      <CheckResults :rows="rows" />
    </ASpace>
  </ACard>
</template>
