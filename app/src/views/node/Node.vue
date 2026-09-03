<script setup lang="tsx">
import type { SelectProps } from 'antdv-next'
import type { AnalyticNode } from '@/api/node'
import { StdCurd } from '@uozi-admin/curd'
import nodeApi from '@/api/node'
import FooterToolBar from '@/components/FooterToolbar'
import BatchUpgrader from './BatchUpgrader.vue'
import envColumns from './nodeColumns'
import SyncConfigs from './SyncConfigs.vue'

const route = useRoute()
const { message } = useGlobalApp()
const curd = ref()
const loadingFromSettings = ref(false)
const loadingReload = ref(false)
const loadingRestart = ref(false)

// Auto refresh logic
const isAutoRefresh = ref(true)
const autoRefreshInterval = ref(5) // seconds
const autoRefreshTimer = ref<NodeJS.Timeout | null>(null)
const autoRefreshIntervalOptions: SelectProps['options'] = [
  { label: '5s', value: 5 },
  { label: '10s', value: 10 },
  { label: '30s', value: 30 },
  { label: '60s', value: 60 },
]

function startAutoRefresh() {
  if (autoRefreshTimer.value) {
    clearInterval(autoRefreshTimer.value)
  }

  autoRefreshTimer.value = setInterval(() => {
    if (curd.value) {
      curd.value.refresh()
    }
  }, autoRefreshInterval.value * 1000)
}

function stopAutoRefresh() {
  if (autoRefreshTimer.value) {
    clearInterval(autoRefreshTimer.value)
    autoRefreshTimer.value = null
  }
}

// Watch for auto refresh state changes
watch(isAutoRefresh, newValue => {
  if (newValue) {
    startAutoRefresh()
    message.success($gettext('Auto refresh enabled'))
  }
  else {
    stopAutoRefresh()
    message.success($gettext('Auto refresh disabled'))
  }
})

// Initialize auto refresh on mount if enabled
onMounted(() => {
  if (isAutoRefresh.value) {
    startAutoRefresh()
  }
})

// Clean up timer on component unmount
onBeforeUnmount(() => {
  stopAutoRefresh()
})

function loadFromSettings() {
  loadingFromSettings.value = true
  nodeApi.load_from_settings().then(() => {
    curd.value.getList()
    message.success($gettext('Load successfully'))
  }).finally(() => {
    loadingFromSettings.value = false
  })
}
const selectedNodeIds = ref<number[]>([])
const selectedNodes = ref<AnalyticNode[]>([])
const refUpgrader = ref()
const refSyncConfigs = useTemplateRef('refSyncConfigs')

// A newly added node starts empty, so pushing everything at once is the fastest
// way to bring it in line with this instance.
function syncConfigs() {
  if (selectedNodeIds.value.length === 0) {
    message.warning($gettext('Please select at least one node to sync configs'))
    return
  }

  refSyncConfigs.value?.open(selectedNodeIds.value, selectedNodes.value)
}

function batchUpgrade() {
  refUpgrader.value.open(selectedNodeIds, selectedNodes)
}

function reloadNginx() {
  if (selectedNodeIds.value.length === 0) {
    message.warning($gettext('Please select at least one node to reload Nginx'))
    return
  }

  loadingReload.value = true
  nodeApi.reloadNginx(selectedNodeIds.value).then(() => {
    message.success($gettext('Nginx reload operations have been dispatched to remote nodes'))
  }).finally(() => {
    loadingReload.value = false
  })
}

function restartNginx() {
  if (selectedNodeIds.value.length === 0) {
    message.warning($gettext('Please select at least one node to restart Nginx'))
    return
  }

  loadingRestart.value = true
  nodeApi.restartNginx(selectedNodeIds.value).then(() => {
    message.success($gettext('Nginx restart operations have been dispatched to remote nodes'))
  }).finally(() => {
    loadingRestart.value = false
  })
}

const inTrash = computed(() => {
  return route.query.trash === 'true'
})
</script>

<template>
  <div>
    <StdCurd
      ref="curd"
      v-model:selected-row-keys="selectedNodeIds"
      v-model:selected-rows="selectedNodes"
      :scroll-x="1000"
      row-selection-type="checkbox"
      :table-props="{
        rowSelection: {
          type: 'checkbox',
          getCheckboxProps: (record) => ({
            disabled: !record.status && record.credential_status !== 'unpaired',
          }),
        },
        pagination: false,
      }"
      :title="$gettext('Nodes')"
      :api="nodeApi"
      :columns="envColumns"
      disable-export
    >
      <template #beforeAdd>
        <AButton size="small" type="link" :loading="loadingFromSettings" @click="loadFromSettings">
          {{ $gettext('Load from settings') }}
        </AButton>
      </template>

      <template #afterListActions>
        <div class="flex items-center gap-2">
          <ASelect
            v-model:value="autoRefreshInterval"
            size="small"
            class="w-16"
            :disabled="isAutoRefresh"
            :options="autoRefreshIntervalOptions"
            @change="isAutoRefresh && startAutoRefresh()"
          />

          <span>{{ $gettext('Auto Refresh') }}</span>
          <ASwitch
            v-model:checked="isAutoRefresh"
            size="small"
          />
        </div>
      </template>
    </StdCurd>

    <BatchUpgrader ref="refUpgrader" @success="curd.refresh()" />

    <SyncConfigs ref="refSyncConfigs" />

    <FooterToolBar v-if="!inTrash">
      <ASpace>
        <ATooltip
          v-if="selectedNodeIds.length === 0"
          :title="$gettext('Please select at least one node to upgrade')"
          placement="topLeft"
        >
          <AButton
            :disabled="selectedNodeIds.length === 0"
            type="primary"
            @click="batchUpgrade"
          >
            {{ $gettext('Upgrade') }}
          </AButton>
        </ATooltip>
        <AButton
          v-else
          type="primary"
          @click="batchUpgrade"
        >
          {{ $gettext('Upgrade') }}
        </AButton>

        <ATooltip
          v-if="selectedNodeIds.length === 0"
          :title="$gettext('Please select at least one node to sync configs')"
          placement="topLeft"
        >
          <AButton
            :disabled="selectedNodeIds.length === 0"
            @click="syncConfigs"
          >
            {{ $gettext('Sync Configs') }}
          </AButton>
        </ATooltip>
        <AButton
          v-else
          @click="syncConfigs"
        >
          {{ $gettext('Sync Configs') }}
        </AButton>

        <ATooltip
          v-if="selectedNodeIds.length === 0"
          :title="$gettext('Please select at least one node to reload Nginx')"
          placement="topLeft"
        >
          <AButton
            :disabled="selectedNodeIds.length === 0"
            :loading="loadingReload"
            @click="reloadNginx"
          >
            {{ $gettext('Reload Nginx') }}
          </AButton>
        </ATooltip>
        <AButton
          v-else
          :loading="loadingReload"
          @click="reloadNginx"
        >
          {{ $gettext('Reload Nginx') }}
        </AButton>

        <ATooltip
          v-if="selectedNodeIds.length === 0"
          :title="$gettext('Please select at least one node to restart Nginx')"
          placement="topLeft"
        >
          <AButton
            :disabled="selectedNodeIds.length === 0"
            :loading="loadingRestart"
            @click="restartNginx"
          >
            {{ $gettext('Restart Nginx') }}
          </AButton>
        </ATooltip>
        <AButton
          v-else
          :loading="loadingRestart"
          @click="restartNginx"
        >
          {{ $gettext('Restart Nginx') }}
        </AButton>
      </ASpace>
    </FooterToolBar>
  </div>
</template>

<style lang="less" scoped>

</style>
