<script setup lang="ts">
import type { Dayjs } from 'dayjs'
import type { MaintenancePayload } from '@/api/site'

const props = defineProps<{
  open: boolean
  title?: string
  okText?: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'confirm': [payload: MaintenancePayload]
}>()

const { message } = useGlobalApp()

const startAt = ref<Dayjs | null>(null)
const endAt = ref<Dayjs | null>(null)
const contact = ref('')
const additioninfomation = ref('')

watch(() => props.open, open => {
  if (!open) {
    startAt.value = null
    endAt.value = null
    contact.value = ''
    additioninfomation.value = ''
  }
})

function handleCancel() {
  emit('update:open', false)
}

function handleOk() {
  if (startAt.value && endAt.value && endAt.value.isBefore(startAt.value)) {
    message.error($gettext('End time must be later than start time'))
    return
  }

  emit('confirm', {
    start_time: startAt.value ? startAt.value.format('YYYY-MM-DD HH:mm:ss') : '',
    end_time: endAt.value ? endAt.value.format('YYYY-MM-DD HH:mm:ss') : '',
    contact: contact.value.trim(),
    additioninfomation: additioninfomation.value.trim(),
  })
  emit('update:open', false)
}

function disabledEndDate(current: Dayjs) {
  if (!startAt.value) {
    return false
  }
  return current.isBefore(startAt.value)
}
</script>

<template>
  <AModal
    :open="open"
    :title="title || $gettext('Set maintenance information')"
    :ok-text="okText || $gettext('Continue')"
    :cancel-text="$gettext('Cancel')"
    :mask="false"
    centered
    @ok="handleOk"
    @cancel="handleCancel"
  >
    <AForm layout="vertical" class="pt-2">
      <AFormItem :label="$gettext('Start time')">
        <ADatePicker
          v-model:value="startAt"
          class="w-full"
          show-time
          :placeholder="$gettext('Select maintenance start time')"
        />
      </AFormItem>

      <AFormItem :label="$gettext('End time')">
        <ADatePicker
          v-model:value="endAt"
          class="w-full"
          show-time
          :disabled-date="disabledEndDate"
          :placeholder="$gettext('Select maintenance end time')"
        />
      </AFormItem>

      <AFormItem :label="$gettext('Contact')">
        <AInput
          v-model:value="contact"
          :maxlength="128"
          :placeholder="$gettext('For example: ops@example.com')"
        />
      </AFormItem>

      <AFormItem :label="$gettext('Additional information')">
        <ATextarea
          v-model:value="additioninfomation"
          :maxlength="500"
          :auto-size="{ minRows: 2, maxRows: 5 }"
          :placeholder="$gettext('For example: expected recovery plan or incident reference')"
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>
