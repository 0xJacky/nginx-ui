<script setup lang="ts">
import site from '@/api/site'
import { ConfigStatus } from '@/constants'
import { message, Modal } from 'ant-design-vue'

/**
 * Component props interface
 */
interface Props {
  /**
   * The name of the site configuration
   */
  siteName: string
  /**
   * Whether the site is enabled
   */
  enabled: boolean
}

// Define props with TypeScript
const props = defineProps<Props>()

// Define event for status change notification
const emit = defineEmits<{
  statusChanged: [{ status: string, enabled: boolean }]
}>()

// Use defineModel for v-model binding
const status = defineModel<string>({
  default: ConfigStatus.Disabled,
})

const [modal, ContextHolder] = Modal.useModal()

/**
 * Enable the site
 */
function enable() {
  site.enable(props.siteName).then(() => {
    message.success($gettext('Enabled successfully'))
    status.value = ConfigStatus.Enabled
    emit('statusChanged', {
      status: ConfigStatus.Enabled,
      enabled: true,
    })
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

/**
 * Disable the site
 */
function disable() {
  site.disable(props.siteName).then(() => {
    message.success($gettext('Disabled successfully'))
    status.value = ConfigStatus.Disabled
    emit('statusChanged', {
      status: ConfigStatus.Disabled,
      enabled: false,
    })
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

/**
 * Enable maintenance mode for the site
 */
function enableMaintenance() {
  site.enableMaintenance(props.siteName).then(() => {
    message.success($gettext('Maintenance mode enabled successfully'))
    status.value = ConfigStatus.Maintenance
    emit('statusChanged', {
      status: ConfigStatus.Maintenance,
      enabled: true,
    })
  }).catch(r => {
    message.error($gettext('Failed to enable maintenance mode %{msg}', { msg: r.message ?? '' }))
  })
}

/**
 * Disable maintenance mode for the site
 */
function disableMaintenance() {
  site.enable(props.siteName).then(() => {
    message.success($gettext('Maintenance mode disabled successfully'))
    status.value = ConfigStatus.Enabled
    emit('statusChanged', {
      status: ConfigStatus.Enabled,
      enabled: true,
    })
  }).catch(r => {
    message.error($gettext('Failed to disable maintenance mode %{msg}', { msg: r.message ?? '' }))
  })
}

/**
 * Handle status change from segmented control
 */
function onChangeStatus(value: string | number) {
  const statusValue = value as string
  if (statusValue === status.value) {
    return
  }

  // Save original status to restore if user cancels
  const originalStatus = status.value

  const statusMap = {
    [ConfigStatus.Enabled]: $gettext('enable'),
    [ConfigStatus.Disabled]: $gettext('disable'),
    [ConfigStatus.Maintenance]: $gettext('set to maintenance mode'),
  }

  modal.confirm({
    title: $gettext('Do you want to %{action} this site?', { action: statusMap[statusValue] }),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    async onOk() {
      if (statusValue === ConfigStatus.Enabled) {
        if (status.value === ConfigStatus.Maintenance) {
          disableMaintenance()
        }
        else {
          enable()
        }
      }
      else if (statusValue === ConfigStatus.Disabled) {
        disable()
      }
      else if (statusValue === ConfigStatus.Maintenance) {
        enableMaintenance()
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
  <div class="site-status-segmented">
    <ContextHolder />
    <ASegmented
      v-model:value="status"
      :options="[
        {
          value: ConfigStatus.Enabled,
          label: $gettext('Enabled'),
        },
        {
          value: ConfigStatus.Disabled,
          label: $gettext('Disabled'),
        },
        {
          value: ConfigStatus.Maintenance,
          label: $gettext('Maintenance'),
        },
      ]"
      @change="onChangeStatus"
    />
  </div>
</template>

<style scoped>
.site-status-segmented {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

:deep(.ant-segmented-item:nth-child(1).ant-segmented-item-selected) {
  background: #1890ff;
  color: white;
}

:deep(.ant-segmented-item:nth-child(2).ant-segmented-item-selected) {
  background: #ff4d4f;
  color: white;
}

:deep(.ant-segmented-item:nth-child(3).ant-segmented-item-selected) {
  background: #faad14;
  color: white;
}

:deep(.ant-segmented-item-selected) {
  border-radius: 6px;
}
</style>
