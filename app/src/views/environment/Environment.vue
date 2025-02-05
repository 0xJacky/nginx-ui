<script setup lang="tsx">
import environment from '@/api/environment'
import FooterToolBar from '@/components/FooterToolbar'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import BatchUpgrader from '@/views/environment/BatchUpgrader.vue'
import envColumns from '@/views/environment/envColumns'
import { message } from 'ant-design-vue'

const route = useRoute()
const curd = ref()
const loadingFromSettings = ref(false)

function loadFromSettings() {
  loadingFromSettings.value = true
  environment.load_from_settings().then(() => {
    curd.value.get_list()
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
      selection-type="checkbox"
      :title="$gettext('Environments')"
      :api="environment"
      :columns="envColumns"
    >
      <template #beforeAdd>
        <AButton size="small" type="link" :loading="loadingFromSettings" @click="loadFromSettings">
          {{ $gettext('Load from settings') }}
        </AButton>
      </template>
    </StdCurd>

    <BatchUpgrader ref="refUpgrader" />

    <FooterToolBar v-if="!inTrash">
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
