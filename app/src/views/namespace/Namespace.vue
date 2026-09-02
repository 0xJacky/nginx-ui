<script setup lang="ts">
import type { UpdateOrderRequest } from '@/api/curd'
import type { Namespace } from '@/api/namespace'
import { StdCurd } from '@uozi-admin/curd'
import namespace from '@/api/namespace'
import NodeSelector from '@/components/NodeSelector'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useClusterSync } from '@/composables/useClusterSync'
import columns from './columns'

const table = useTemplateRef('table')
const { message } = useGlobalApp()
const { report } = useClusterSync()
const syncingId = ref(0)

async function handleDragEnd(data: UpdateOrderRequest) {
  await namespace.updateOrder(data)
  table.value?.refresh()
}

// Replicates the namespace record and all of its sites and streams so every
// member node ends up with the same content.
function syncNamespace(record: Namespace) {
  if (!record.sync_node_ids?.length) {
    message.warning($gettext('This namespace has no sync node'))
    return
  }

  const otpModal = use2FAModal()

  otpModal.open().then(() => {
    syncingId.value = record.id
    namespace.sync(record.id)
      .then(summary => report(summary))
      .finally(() => {
        syncingId.value = 0
      })
  })
}
</script>

<template>
  <StdCurd
    ref="table"
    :title="$gettext('Namespaces')"
    :api="namespace"
    :columns="columns"
    :scroll-x="600"
    disable-export
    row-draggable
    :row-draggable-options="{
      onEnd: handleDragEnd,
    }"
  >
    <template #beforeActions="{ record }">
      <AButton
        type="link"
        size="small"
        :loading="syncingId === record.id"
        @click="syncNamespace(record)"
      >
        {{ $gettext('Sync') }}
      </AButton>
    </template>

    <template #afterForm="{ record }">
      <div class="mb-2">
        {{ $gettext('Sync Nodes') }}
      </div>
      <NodeSelector
        v-model:target="record.sync_node_ids"
        hidden-local
      />
    </template>
  </StdCurd>
</template>

<style scoped lang="less">
</style>
