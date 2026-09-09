<script setup lang="tsx">
import type { MaintenancePayload, Site } from '@/api/site'
import { StdCurd } from '@uozi-admin/curd'
import { message, Modal } from 'antdv-next'
import nginxLog from '@/api/nginx_log'
import site from '@/api/site'
import FooterToolBar from '@/components/FooterToolbar'
import InspectConfig from '@/components/InspectConfig'
import NamespaceTabs from '@/components/NamespaceTabs'
import { ConfigStatus } from '@/constants'
import MaintenanceConfigModal from '@/views/site/components/MaintenanceConfigModal.vue'
import columns from '@/views/site/site_list/columns'
import SiteDuplicate from '@/views/site/site_list/SiteDuplicate.vue'

const route = useRoute()
const router = useRouter()

const curd = ref()
const inspectConfig = ref()
const selectedSiteNames = ref<string[]>([])
const selectedSites = ref<Site[]>([])
const loadingEnable = ref(false)
const loadingDisable = ref(false)
const loadingMaintenance = ref(false)
const [modal, ContextHolder] = Modal.useModal()
const maintenanceModalOpen = ref(false)
const pendingMaintenanceNames = ref<string[]>([])

const namespaceId = ref(Number.parseInt(route.query.namespace_id as string) || 0)

watch(route, () => {
  inspectConfig.value?.test()
})

function destroy(site_name: string) {
  site.deleteItem(site_name).then(() => {
    curd.value.refresh()
    message.success($gettext('Delete site: %{site_name}', { site_name }))
    inspectConfig.value?.test()
  })
}

const isIndexingEnabled = ref(false)

onMounted(async () => {
  try {
    const res = await nginxLog.getAdvancedIndexingStatus()
    isIndexingEnabled.value = !!res.enabled
  }
  catch {
    isIndexingEnabled.value = false
  }
})

async function handleClickAnalytics(name: string) {
  const { logs } = await site.getLogs(name)
  const accessLogs = (logs ?? []).filter(l => l.type === 'access' && l.valid)
  const target = accessLogs.find(l => !l.inherited) ?? accessLogs[0]

  if (!target) {
    message.warning($gettext('No valid access log path found for this site'))
    return
  }

  if (target.inherited) {
    message.info($gettext('This site uses the default access log, which may contain traffic from other sites'))
  }

  router.push({
    path: '/nginx_log/site',
    query: {
      path: target.path,
      view: 'dashboard',
    },
  })
}

const show_duplicator = ref(false)

const target = ref('')

function handle_click_duplicate(name: string) {
  show_duplicator.value = true
  target.value = name
}

function clearSelectedSites() {
  selectedSiteNames.value = []
  selectedSites.value = []
}

function refreshAfterBatchStatusChanged() {
  clearSelectedSites()
  curd.value.refresh()
  inspectConfig.value?.test()
}

type BatchStatusAction = 'enable' | 'disable' | 'maintenance'

function executeBatchStatusAction(action: BatchStatusAction, names: string[], payload: MaintenancePayload = {}) {
  const loadingMap = {
    enable: loadingEnable,
    disable: loadingDisable,
    maintenance: loadingMaintenance,
  }
  const requestMap: Record<BatchStatusAction, (names: string[], payload: MaintenancePayload) => Promise<unknown>> = {
    enable: site.batchEnable,
    disable: site.batchDisable,
    maintenance: (maintenanceNames: string[], maintenancePayload: MaintenancePayload) => site.batchEnableMaintenance(maintenanceNames, maintenancePayload),
  }
  const successMessageMap: Record<BatchStatusAction, string> = {
    enable: $gettext('Sites enabled successfully'),
    disable: $gettext('Sites disabled successfully'),
    maintenance: $gettext('Sites switched to maintenance mode successfully'),
  }

  const loading = loadingMap[action]
  const request = requestMap[action]
  const successMessage = successMessageMap[action]

  loading.value = true
  return request(names, payload).then(() => {
    message.success(successMessage)
    refreshAfterBatchStatusChanged()
  }).finally(() => {
    loading.value = false
  })
}

function confirmBatchStatusAction(action: BatchStatusAction, payload: MaintenancePayload = {}, namesOverride?: string[]) {
  const names = namesOverride && namesOverride.length > 0
    ? [...namesOverride]
    : [...selectedSiteNames.value]

  if (names.length === 0) {
    const warningMessageMap: Record<BatchStatusAction, string> = {
      enable: $gettext('Please select at least one site to enable'),
      disable: $gettext('Please select at least one site to disable'),
      maintenance: $gettext('Please select at least one site to switch to maintenance mode'),
    }
    message.warning(warningMessageMap[action])
    return
  }

  const titleMap: Record<BatchStatusAction, string> = {
    enable: $gettext('Do you want to enable selected sites?'),
    disable: $gettext('Do you want to disable selected sites?'),
    maintenance: $gettext('Do you want to switch selected sites to maintenance mode?'),
  }
  const introMap: Record<BatchStatusAction, string> = {
    enable: $gettext('The following sites will be enabled:'),
    disable: $gettext('The following sites will be disabled:'),
    maintenance: $gettext('The following sites will be switched to maintenance mode:'),
  }
  const okTextMap: Record<BatchStatusAction, string> = {
    enable: $gettext('Enable'),
    disable: $gettext('Disable'),
    maintenance: $gettext('Maintenance'),
  }

  modal.confirm({
    title: titleMap[action],
    content: () => h('div', [
      h('p', introMap[action]),
      h('ul', { class: 'max-h-60 overflow-auto pl-5' }, names.map(name => h('li', { key: name }, name))),
    ]),
    mask: false,
    centered: true,
    okText: okTextMap[action],
    okButtonProps: {
      danger: action === 'disable',
    },
    cancelText: $gettext('Cancel'),
    onOk: () => executeBatchStatusAction(action, names, payload),
  })
}

function batchEnableSites() {
  confirmBatchStatusAction('enable')
}

function batchDisableSites() {
  confirmBatchStatusAction('disable')
}

function batchEnableMaintenanceSites() {
  if (selectedSiteNames.value.length === 0) {
    message.warning($gettext('Please select at least one site to switch to maintenance mode'))
    return
  }
  pendingMaintenanceNames.value = [...selectedSiteNames.value]
  maintenanceModalOpen.value = true
}

function onMaintenanceModalOpenChange(open: boolean) {
  maintenanceModalOpen.value = open
  if (!open) {
    pendingMaintenanceNames.value = []
  }
}

function onMaintenanceConfirm(payload: MaintenancePayload) {
  const names = pendingMaintenanceNames.value.length > 0 ? pendingMaintenanceNames.value : [...selectedSiteNames.value]
  pendingMaintenanceNames.value = []
  confirmBatchStatusAction('maintenance', payload, names)
}
</script>

<template>
  <div>
    <StdCurd
      ref="curd"
      v-model:selected-row-keys="selectedSiteNames"
      v-model:selected-rows="selectedSites"
      :title="$gettext('Manage Sites')"
      :api="site"
      :columns="columns"
      :table-props="{
        rowKey: 'name',
      }"
      disable-add
      disable-delete
      disable-trash
      disable-view
      disable-export
      row-selection-type="checkbox"
      :custom-query-params="{
        namespace_id: namespaceId,
      }"
      :scroll-x="1600"
      @edit-item="record => router.push({
        path: `/sites/${encodeURIComponent(record.name)}`,
      })"
    >
      <template #beforeListActions>
        <AButton
          type="link"
          size="small"
          @click="router.push({
            path: '/sites/add',
          })"
        >
          {{ $gettext('Add') }}
        </AButton>
      </template>
      <template #beforeCardBody>
        <InspectConfig ref="inspectConfig" :namespace-id="namespaceId" />
        <NamespaceTabs v-model:active-key="namespaceId" />
      </template>
      <template #afterActions="{ record }">
        <AButton
          v-if="isIndexingEnabled"
          type="link"
          size="small"
          @click="handleClickAnalytics(record.name)"
        >
          {{ $gettext('Analytics') }}
        </AButton>
        <AButton
          type="link"
          size="small"
          @click="handle_click_duplicate(record.name)"
        >
          {{ $gettext('Duplicate') }}
        </AButton>
        <APopconfirm
          :cancel-text="$gettext('No')"
          :ok-text="$gettext('OK')"
          :title="$gettext('Are you sure you want to delete?')"
          :disabled="record.status !== ConfigStatus.Disabled"
          @confirm="destroy(record.name)"
        >
          <AButton
            type="link"
            size="small"
            :disabled="record.status !== ConfigStatus.Disabled"
          >
            {{ $gettext('Delete') }}
          </AButton>
        </APopconfirm>
      </template>
    </StdCurd>
    <SiteDuplicate
      v-model:visible="show_duplicator"
      :name="target"
      @duplicated="() => curd.refresh()"
    />
    <ContextHolder />
    <MaintenanceConfigModal
      :open="maintenanceModalOpen"
      :title="$gettext('Set maintenance information for selected sites')"
      :ok-text="$gettext('Next')"
      @update:open="onMaintenanceModalOpenChange"
      @confirm="onMaintenanceConfirm"
    />

    <FooterToolBar v-if="selectedSiteNames.length > 0">
      <template #extra>
        {{ $gettext('%{count} sites selected', { count: String(selectedSiteNames.length) }) }}
      </template>

      <ASpace>
        <AButton
          :loading="loadingEnable"
          type="primary"
          @click="batchEnableSites"
        >
          {{ $gettext('Enable') }}
        </AButton>

        <AButton
          :loading="loadingDisable"
          danger
          @click="batchDisableSites"
        >
          {{ $gettext('Disable') }}
        </AButton>

        <AButton
          :loading="loadingMaintenance"
          type="primary"
          class="!bg-amber-500 !border-amber-500 hover:!bg-amber-600 hover:!border-amber-600"
          @click="batchEnableMaintenanceSites"
        >
          {{ $gettext('Maintenance') }}
        </AButton>
      </ASpace>
    </FooterToolBar>
  </div>
</template>

<style>
.std-curd-edit-modal.ant-modal,
.std-curd-edit-modal .ant-modal,
.ant-modal-wrap .std-curd-edit-modal.ant-modal {
  width: fit-content !important;
  min-width: min(760px, calc(100vw - 24px)) !important;
  max-width: calc(100vw - 24px) !important;
}
</style>
