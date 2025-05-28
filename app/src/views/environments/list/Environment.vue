<script setup lang="tsx">
import { StdCurd } from '@uozi-admin/curd'
import { message } from 'ant-design-vue'
import environment from '@/api/environment'
import node from '@/api/node'
import FooterToolBar from '@/components/FooterToolbar'
import BatchUpgrader from './BatchUpgrader.vue'
import envColumns from './envColumns'

const route = useRoute()
const curd = ref()
const loadingFromSettings = ref(false)
const loadingReload = ref(false)
const loadingRestart = ref(false)

function loadFromSettings() {
  loadingFromSettings.value = true
  environment.load_from_settings().then(() => {
    curd.value.getList()
    message.success($gettext('Load successfully'))
  }).finally(() => {
    loadingFromSettings.value = false
  })
}
const selectedNodeIds = ref([])
const selectedNodes = ref([])
const refUpgrader = ref()

function batchUpgrade() {
  refUpgrader.value.open(selectedNodeIds, selectedNodes)
}

function reloadNginx() {
  if (selectedNodeIds.value.length === 0) {
    message.warning($gettext('Please select at least one node to reload Nginx'))
    return
  }

  loadingReload.value = true
  node.reloadNginx(selectedNodeIds.value).then(() => {
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
  node.restartNginx(selectedNodeIds.value).then(() => {
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
            disabled: !record.status,
          }),
        },
      }"
      :title="$gettext('Environments')"
      :api="environment"
      :columns="envColumns"
      disable-export
    >
      <template #beforeAdd>
        <AButton size="small" type="link" :loading="loadingFromSettings" @click="loadFromSettings">
          {{ $gettext('Load from settings') }}
        </AButton>
      </template>
    </StdCurd>

    <BatchUpgrader ref="refUpgrader" />

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
