<script setup lang="tsx">
import environment from '@/api/environment'
import FooterToolBar from '@/components/FooterToolbar'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import BatchUpgrader from '@/views/environment/BatchUpgrader.vue'
import envColumns from '@/views/environment/envColumns'
import { message } from 'ant-design-vue'

const curd = ref()
function loadFromSettings() {
  environment.load_from_settings().then(() => {
    curd.value.get_list()
    message.success($gettext('Load successfully'))
  }).catch(e => {
    message.error(`${$gettext('Server error')} ${e?.message}`)
  })
}
const selectedNodeIds = ref([])
const selectedNodes = ref([])
const refUpgrader = ref()

function batchUpgrade() {
  refUpgrader.value.open(selectedNodeIds, selectedNodes)
}
</script>

<template>
  <div>
    <StdCurd
      ref="curd"
      v-model:selected-row-keys="selectedNodeIds"
      v-model:selected-rows="selectedNodes"
      selection-type="checkbox"
      :title="$gettext('Environment')"
      :api="environment"
      :columns="envColumns"
    >
      <template #extra>
        <a @click="loadFromSettings">{{ $gettext('Load from settings') }}</a>
      </template>
    </StdCurd>

    <BatchUpgrader ref="refUpgrader" />

    <FooterToolBar>
      <ATooltip
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
    </FooterToolBar>
  </div>
</template>

<style lang="less" scoped>

</style>
