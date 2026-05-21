<script setup lang="ts">
import type { StepOutcome, VerifyResult } from '@/api/host_setup'
import { CheckCircleOutlined, CloseCircleOutlined, ExclamationCircleOutlined } from '@ant-design/icons-vue'
import { useClipboard } from '@vueuse/core'
import { computed, ref } from 'vue'
import hostSetup from '@/api/host_setup'

const result = ref<VerifyResult | null>(null)
const running = ref(false)
const skipNginxT = ref(false)
const { copy } = useClipboard()

async function run() {
  running.value = true
  try {
    result.value = await hostSetup.verify(skipNginxT.value)
  }
  finally {
    running.value = false
  }
}

const allPassed = computed(() => {
  if (!result.value)
    return false
  return Object.values(result.value.steps).every(s => s.ok || s.level === 'warning')
})

const hasWarning = computed(() => {
  if (!result.value)
    return false
  return Object.values(result.value.steps).some(s => stepLevel(s) === 'warning')
})

function stepLevel(step: StepOutcome) {
  if (step.level)
    return step.level
  return step.ok ? 'success' : 'error'
}

function tagColor(step: StepOutcome) {
  switch (stepLevel(step)) {
    case 'success':
      return 'success'
    case 'warning':
      return 'warning'
    default:
      return 'error'
  }
}

function tagText(step: StepOutcome) {
  switch (stepLevel(step)) {
    case 'success':
      return 'OK'
    case 'warning':
      return 'WARN'
    default:
      return 'FAIL'
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center gap-3">
      <AButton type="primary" :loading="running" @click="run">
        {{ $gettext('Run verification') }}
      </AButton>
      <ACheckbox v-model:checked="skipNginxT">
        {{ $gettext('Skip nginx -t (no side effects)') }}
      </ACheckbox>
    </div>

    <AList v-if="result" :data-source="Object.entries(result.steps)">
      <template #renderItem="{ item }">
        <AListItem>
          <div class="w-full">
            <div class="flex items-center justify-between">
              <span>
                <CheckCircleOutlined v-if="stepLevel(item[1]) === 'success'" :style="{ color: 'green' }" />
                <ExclamationCircleOutlined v-else-if="stepLevel(item[1]) === 'warning'" :style="{ color: '#faad14' }" />
                <CloseCircleOutlined v-else :style="{ color: 'red' }" />
                <strong class="ml-2">{{ item[0] }}</strong>
              </span>
              <ATag :color="tagColor(item[1])">
                {{ tagText(item[1]) }}
              </ATag>
            </div>
            <div class="text-secondary text-sm mt-1">
              {{ item[1].detail }}
            </div>
            <div v-if="item[1].remediation" class="mt-2 flex items-start gap-2">
              <AButton size="small" @click="copy(item[1].remediation!)">
                {{ $gettext('Copy fix') }}
              </AButton>
              <span class="text-xs text-secondary">{{ item[1].remediation }}</span>
            </div>
          </div>
        </AListItem>
      </template>
    </AList>

    <AAlert
      v-if="allPassed && !hasWarning"
      type="success"
      show-icon
      :message="$gettext('All checks passed — you may save the configuration.')"
    />
    <AAlert
      v-else-if="allPassed && hasWarning"
      type="warning"
      show-icon
      :message="$gettext('Blocking checks passed, but warnings need review before saving.')"
    />
  </div>
</template>
