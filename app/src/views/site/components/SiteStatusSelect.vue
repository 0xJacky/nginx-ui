<script setup lang="ts">
import type { SelectProps, SelectValue } from 'antdv-next'
import type { MaintenancePayload, SiteStatus } from '@/api/site'
import { Modal } from 'antdv-next'
import site from '@/api/site'
import { ConfigStatus } from '@/constants'
import MaintenanceConfigModal from '@/views/site/components/MaintenanceConfigModal.vue'

// Define props with TypeScript
const props = defineProps<{
  siteName: string
  status: SiteStatus
}>()

// Define event for status change notification
const emit = defineEmits<{
  statusChanged: [{ status: SiteStatus }]
}>()

const { message } = useGlobalApp()
const [modal, ContextHolder] = Modal.useModal()
const maintenanceModalOpen = ref(false)
const pendingMaintenanceOriginalStatus = ref<SiteStatus>(ConfigStatus.Disabled)
const displayStatus = ref<SiteStatus>(props.status)
const selectRenderKey = ref(0)

watch(() => props.status, val => {
  displayStatus.value = val
  selectRenderKey.value += 1
}, { immediate: true })

function restoreDisplayStatus(statusValue: SiteStatus) {
  displayStatus.value = statusValue
  selectRenderKey.value += 1
}

const statusOptions = computed<SelectProps['options']>(() => [
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
])

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
  return statusStyles[displayStatus.value] || {}
})

// Enable the site
function enable() {
  site.enable(props.siteName).then(() => {
    message.success($gettext('Enabled successfully'))
    restoreDisplayStatus(ConfigStatus.Enabled)
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
    restoreDisplayStatus(ConfigStatus.Disabled)
    emit('statusChanged', {
      status: ConfigStatus.Disabled,
    })
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

// Enable maintenance mode for the site
function enableMaintenance(payload: MaintenancePayload) {
  site.enableMaintenance(props.siteName, payload).then(() => {
    message.success($gettext('Maintenance mode enabled successfully'))
    restoreDisplayStatus(ConfigStatus.Maintenance)
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
    restoreDisplayStatus(ConfigStatus.Enabled)
    emit('statusChanged', {
      status: ConfigStatus.Enabled,
    })
  }).catch(r => {
    message.error($gettext('Failed to disable maintenance mode %{msg}', { msg: r.message ?? '' }))
  })
}

// Handle status change from select
function onChangeStatus(value: SelectValue) {
  const statusValue = value as SiteStatus
  if (!statusValue || statusValue === displayStatus.value) {
    return
  }

  const originalStatus = displayStatus.value
  restoreDisplayStatus(originalStatus)

  if (statusValue === ConfigStatus.Maintenance) {
    pendingMaintenanceOriginalStatus.value = originalStatus
    maintenanceModalOpen.value = true
    return
  }

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
        if (displayStatus.value === ConfigStatus.Maintenance) {
          disableMaintenance()
        }
        else {
          enable()
        }
      }
      else if (statusValue === ConfigStatus.Disabled) {
        disable()
      }
    },
    onCancel() {
      restoreDisplayStatus(originalStatus)
    },
  })
}

function onMaintenanceConfirm(payload: MaintenancePayload) {
  modal.confirm({
    title: $gettext('Do you want to set this site to maintenance mode?'),
    mask: false,
    centered: true,
    okText: $gettext('Maintenance'),
    cancelText: $gettext('Cancel'),
    onOk: () => enableMaintenance(payload),
    onCancel: () => restoreDisplayStatus(pendingMaintenanceOriginalStatus.value),
  })
}

function onMaintenanceModalOpenChange(open: boolean) {
  maintenanceModalOpen.value = open
  if (!open) {
    restoreDisplayStatus(pendingMaintenanceOriginalStatus.value)
  }
}
</script>

<template>
  <div class="site-status-select">
    <ContextHolder />
    <MaintenanceConfigModal
      :open="maintenanceModalOpen"
      :title="$gettext('Set maintenance information for this site')"
      :ok-text="$gettext('Next')"
      @update:open="onMaintenanceModalOpenChange"
      @confirm="onMaintenanceConfirm"
    />
    <ASelect
      :key="selectRenderKey"
      :value="displayStatus"
      class="status-select"
      :popup-match-select-width="false"
      :styles="{
        popup: {
          root: {
            minWidth: '164px',
          },
        },
      }"
      :classes="{
        root: 'status-select-root',
        content: 'status-select-content',
        suffix: 'status-select-suffix',
        popup: {
          listItem: 'status-select-list-item',
        },
      }"
      :style="selectStyle"
      :options="statusOptions"
      @change="onChangeStatus"
    />
  </div>
</template>

<style scoped>
.site-status-select {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

.status-select {
  min-width: 164px;
}

:deep(.status-select-root) {
  transition: all 0.3s ease !important;
  font-weight: 500 !important;
  border-radius: 6px !important;
}

:deep(.status-select-content) {
  font-weight: 500 !important;
}

/* Ensure custom background colors are applied correctly */
:deep(.status-select-root) {
  background-color: var(--ant-select-bg) !important;
  color: var(--ant-select-color) !important;
}

:deep(.status-select-content) {
  color: var(--ant-select-color) !important;
}

:deep(.status-select-suffix) {
  color: var(--ant-select-color) !important;
}

/* Override focus and hover styles to maintain custom colors */
:deep(.status-select-root:not(.ant-select-disabled):hover) {
  border-color: var(--ant-select-border) !important;
  background-color: var(--ant-select-bg) !important;
}

:deep(.status-select-root.ant-select-focused) {
  border-color: var(--ant-select-border) !important;
  background-color: var(--ant-select-bg) !important;
  box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.1) !important;
}

/* Make sure dropdown options also have appropriate styling */
:deep(.ant-select-dropdown .status-select-list-item) {
  padding: 8px 12px !important;
}

:deep(.ant-select-dropdown .status-select-list-item:hover) {
  background-color: rgba(0, 0, 0, 0.04) !important;
}

:deep(.ant-select-dropdown .status-select-list-item.ant-select-item-option-selected) {
  background-color: rgba(24, 144, 255, 0.1) !important;
  font-weight: 600 !important;
}
</style>
