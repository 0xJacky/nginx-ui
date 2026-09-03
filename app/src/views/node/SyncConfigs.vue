<script setup lang="ts">
import type { AnalyticNode } from '@/api/node'
import nodeApi from '@/api/node'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useClusterSync } from '@/composables/useClusterSync'

const { report } = useClusterSync()

const visible = ref(false)
const loading = ref(false)
const nodeIds = ref<number[]>([])
const nodeNames = ref<string[]>([])

const scope = ref({
  configs: true,
  sites: true,
  streams: true,
  overwrite: true,
})

function open(ids: number[], nodes: AnalyticNode[]) {
  visible.value = true
  nodeIds.value = [...ids]
  nodeNames.value = nodes.map(node => node.name)
  scope.value = { configs: true, sites: true, streams: true, overwrite: true }
}

defineExpose({
  open,
})

function ok() {
  const otpModal = use2FAModal()

  otpModal.open().then(() => {
    loading.value = true
    nodeApi.syncConfigs(nodeIds.value, scope.value)
      .then(summary => {
        visible.value = false
        report(summary)
      })
      .finally(() => {
        loading.value = false
      })
  })
}
</script>

<template>
  <AModal
    v-model:open="visible"
    :mask="false"
    :title="$gettext('Sync Configs')"
    :confirm-loading="loading"
    @ok="ok"
  >
    <AAlert
      class="mb-4"
      type="warning"
      show-icon
      :title="$gettext('The selected content will be pushed to: %{nodes}', { nodes: nodeNames.join(', ') })"
    />
    <ASpace orientation="vertical">
      <ACheckbox v-model:checked="scope.configs">
        {{ $gettext('Configurations') }}
      </ACheckbox>
      <ACheckbox v-model:checked="scope.sites">
        {{ $gettext('Sites') }}
      </ACheckbox>
      <ACheckbox v-model:checked="scope.streams">
        {{ $gettext('Streams') }}
      </ACheckbox>
      <ACheckbox v-model:checked="scope.overwrite">
        {{ $gettext('Overwrite exist file') }}
      </ACheckbox>
    </ASpace>
  </AModal>
</template>

<style scoped lang="less">
</style>
