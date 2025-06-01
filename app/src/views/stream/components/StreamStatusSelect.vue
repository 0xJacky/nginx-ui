<script setup lang="ts">
import type { SiteStatus } from '@/api/site'
import type { CheckedType } from '@/types'
import { message, Modal } from 'ant-design-vue'
import stream from '@/api/stream'
import { ConfigStatus } from '@/constants'

// Define props with TypeScript
const props = defineProps<{
  streamName: string
}>()

// Define event for status change notification
const emit = defineEmits<{
  statusChanged: [{ status: SiteStatus }]
}>()

// Use defineModel for v-model binding
const status = defineModel<SiteStatus>('status')

const [modal, ContextHolder] = Modal.useModal()

// Enable the stream
function enable() {
  stream.enable(props.streamName).then(() => {
    message.success($gettext('Enabled successfully'))
    status.value = ConfigStatus.Enabled
    emit('statusChanged', {
      status: ConfigStatus.Enabled,
    })
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

// Disable the stream
function disable() {
  stream.disable(props.streamName).then(() => {
    message.success($gettext('Disabled successfully'))
    status.value = ConfigStatus.Disabled
    emit('statusChanged', {
      status: ConfigStatus.Disabled,
    })
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

function onChangeStatus(checked: CheckedType) {
  const isChecked = checked === true || checked === 'true'
  // Save original status to restore if user cancels
  const originalStatus = status.value

  const action = isChecked ? $gettext('enable') : $gettext('disable')

  modal.confirm({
    title: $gettext('Do you want to %{action} this stream?', { action }),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    async onOk() {
      if (isChecked) {
        enable()
      }
      else {
        disable()
      }
    },
    onCancel() {
      // Restore original status if user cancels
      status.value = originalStatus
    },
  })
}
</script>

<template>
  <div class="stream-status-select">
    <ContextHolder />
    <div class="status-display">
      <ASwitch
        :checked="status === ConfigStatus.Enabled"
        @change="onChangeStatus"
      />
    </div>
  </div>
</template>

<style scoped>
.stream-status-select {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

.status-display {
  display: flex;
  align-items: center;
}
</style>
