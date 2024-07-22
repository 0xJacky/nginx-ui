<script setup lang="tsx">
import { message } from 'ant-design-vue'
import environment from '@/api/environment'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import envColumns from '@/views/environment/envColumns'
import FooterToolBar from '@/components/FooterToolbar'
import BatchUpgrader from '@/views/environment/BatchUpgrader.vue'

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

    <FooterToolBar v-if="selectedNodes?.length > 0">
      <AButton
        type="primary"
        @click="batchUpgrade"
      >
        {{ $gettext('Upgrade') }}
      </AButton>
    </FooterToolBar>
  </div>
</template>

<style lang="less" scoped>

</style>
