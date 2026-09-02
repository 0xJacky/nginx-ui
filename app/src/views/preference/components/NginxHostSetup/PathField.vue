<script setup lang="ts">
import type { DetectablePathField } from './useHostSetupWizard'
import { AimOutlined, EditOutlined, UndoOutlined } from '@ant-design/icons-vue'
import { computed } from 'vue'
import { useHostSetupWizard } from './useHostSetupWizard'

const props = defineProps<{
  field: DetectablePathField
  label: string
  placeholder?: string
  required?: boolean
}>()

const { params, detectedValue, fieldOrigin, restoreDetected } = useHostSetupWizard()

const value = computed({
  get: () => params.value[props.field] ?? '',
  set: next => {
    params.value = { ...params.value, [props.field]: next }
  },
})

const origin = computed(() => fieldOrigin(props.field))
const detected = computed(() => detectedValue(props.field))
const isEmpty = computed(() => Boolean(props.required) && !value.value.trim())
</script>

<template>
  <AFormItem :required="required" :validate-status="isEmpty ? 'error' : undefined">
    <template #label>
      <ASpace :size="4" wrap>
        <span>{{ label }}</span>
        <ATag v-if="origin === 'detected'" color="success" :bordered="false">
          <AimOutlined />
          {{ $gettext('Auto-detected') }}
        </ATag>
        <ATag v-else-if="origin === 'overridden'" color="warning" :bordered="false">
          <EditOutlined />
          {{ $gettext('Manual override') }}
        </ATag>
      </ASpace>
    </template>

    <AInput v-model:value="value" :placeholder="placeholder" allow-clear />

    <template v-if="origin === 'overridden' || isEmpty" #extra>
      <ASpace v-if="origin === 'overridden'" :size="4" wrap>
        <ATypographyText type="secondary" class="text-xs">
          {{ $gettext('Detected:') }}
        </ATypographyText>
        <ATypographyText type="secondary" code copyable class="text-xs">
          {{ detected }}
        </ATypographyText>
        <AButton type="link" size="small" @click="restoreDetected(field)">
          <UndoOutlined />
          {{ $gettext('Restore detected value') }}
        </AButton>
      </ASpace>
      <ATypographyText v-else type="danger" class="text-xs">
        {{ $gettext('This field is required') }}
      </ATypographyText>
    </template>
  </AFormItem>
</template>
