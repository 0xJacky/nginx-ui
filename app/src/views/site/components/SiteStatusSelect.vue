<script setup lang="ts">
import type { SelectValue } from 'ant-design-vue/es/select'
import type { SiteStatus } from '@/api/site'
import { message, Modal } from 'ant-design-vue'
import site from '@/api/site'
import { ConfigStatus } from '@/constants'

// Define props with TypeScript
const props = defineProps<{
  siteName: string
}>()

// Define event for status change notification
const emit = defineEmits<{
  statusChanged: [{ status: SiteStatus }]
}>()

// Use defineModel for v-model binding
const status = defineModel<string>({
  default: ConfigStatus.Disabled,
})

const [modal, ContextHolder] = Modal.useModal()

// Computed property for select style based on current status
const selectStyle = computed(() => {
  const statusStyles = {
    [ConfigStatus.Enabled]: {
      '--ant-select-bg': '#1890ff',
      '--ant-select-border': '#1890ff',
      '--ant-select-color': '#ffffff',
      'color': '#ffffff',
    },
    [ConfigStatus.Disabled]: {
      '--ant-select-bg': '#ff4d4f',
      '--ant-select-border': '#ff4d4f',
      '--ant-select-color': '#ffffff',
      'color': '#ffffff',
    },
    [ConfigStatus.Maintenance]: {
      '--ant-select-bg': '#faad14',
      '--ant-select-border': '#faad14',
      '--ant-select-color': '#ffffff',
      'color': '#ffffff',
    },
  }
  return statusStyles[status.value] || {}
})

// Enable the site
function enable() {
  site.enable(props.siteName).then(() => {
    message.success($gettext('Enabled successfully'))
    status.value = ConfigStatus.Enabled
    emit('statusChanged', {
      status: ConfigStatus.Enabled,
    })
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

// Disable the site
function disable() {
  site.disable(props.siteName).then(() => {
    message.success($gettext('Disabled successfully'))
    status.value = ConfigStatus.Disabled
    emit('statusChanged', {
      status: ConfigStatus.Disabled,
    })
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

// Enable maintenance mode for the site
function enableMaintenance() {
  site.enableMaintenance(props.siteName).then(() => {
    message.success($gettext('Maintenance mode enabled successfully'))
    status.value = ConfigStatus.Maintenance
    emit('statusChanged', {
      status: ConfigStatus.Maintenance,
    })
  }).catch(r => {
    message.error($gettext('Failed to enable maintenance mode %{msg}', { msg: r.message ?? '' }))
  })
}

// Disable maintenance mode for the site
function disableMaintenance() {
  site.enable(props.siteName).then(() => {
    message.success($gettext('Maintenance mode disabled successfully'))
    status.value = ConfigStatus.Enabled
    emit('statusChanged', {
      status: ConfigStatus.Enabled,
    })
  }).catch(r => {
    message.error($gettext('Failed to disable maintenance mode %{msg}', { msg: r.message ?? '' }))
  })
}

// Handle status change from select
function onChangeStatus(value: SelectValue) {
  const statusValue = value as string
  if (!statusValue || statusValue === status.value) {
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
  <div class="site-status-select">
    <ContextHolder />
    <ASelect
      :value="status"
      class="status-select"
      :style="selectStyle"
      @change="onChangeStatus"
    >
      <ASelectOption :value="ConfigStatus.Enabled">
        {{ $gettext('Enabled') }}
      </ASelectOption>
      <ASelectOption :value="ConfigStatus.Disabled">
        {{ $gettext('Disabled') }}
      </ASelectOption>
      <ASelectOption :value="ConfigStatus.Maintenance">
        {{ $gettext('Maintenance') }}
      </ASelectOption>
    </ASelect>
  </div>
</template>

<style scoped>
.site-status-select {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  max-width: 200px;
}

.status-select {
  min-width: 120px;
}

:deep(.ant-select-selector) {
  transition: all 0.3s ease !important;
  font-weight: 500 !important;
  border-radius: 6px !important;
}

:deep(.ant-select-selection-item) {
  font-weight: 500 !important;
}

/* Ensure custom background colors are applied correctly */
:deep(.status-select .ant-select-selector) {
  background-color: var(--ant-select-bg) !important;
  color: var(--ant-select-color) !important;
}

:deep(.status-select .ant-select-selection-item) {
  color: var(--ant-select-color) !important;
}

:deep(.status-select .ant-select-arrow) {
  color: var(--ant-select-color) !important;
}

/* Override focus and hover styles to maintain custom colors */
:deep(.ant-select:not(.ant-select-disabled):hover .ant-select-selector) {
  border-color: var(--ant-select-border) !important;
  background-color: var(--ant-select-bg) !important;
}

:deep(.ant-select-focused .ant-select-selector) {
  border-color: var(--ant-select-border) !important;
  background-color: var(--ant-select-bg) !important;
  box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.1) !important;
}

/* Make sure dropdown options also have appropriate styling */
:deep(.ant-select-dropdown .ant-select-item-option) {
  padding: 8px 12px !important;
}

:deep(.ant-select-dropdown .ant-select-item-option:hover) {
  background-color: rgba(0, 0, 0, 0.04) !important;
}

:deep(.ant-select-dropdown .ant-select-item-option-selected) {
  background-color: rgba(24, 144, 255, 0.1) !important;
  font-weight: 600 !important;
}
</style>
