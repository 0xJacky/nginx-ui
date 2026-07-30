<script setup lang="ts">
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import config from '@/api/config'
import NodeSelector from '@/components/NodeSelector'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useClusterSync } from '@/composables/useClusterSync'

const { message } = useGlobalApp()
const { report } = useClusterSync()

const visible = ref(false)
const loading = ref(false)
const dir = ref('')
const name = ref('')
const syncNodeIds = ref<number[]>([])
const syncOverwrite = ref(true)

/**
 * @param path directory path relative to the Nginx configuration root, already
 * encoded the same way the configuration list encodes its `dir` query.
 */
function open(path: string, displayName: string) {
  visible.value = true
  dir.value = path
  name.value = displayName
  syncNodeIds.value = []
  syncOverwrite.value = true
}

defineExpose({
  open,
})

function ok() {
  if (syncNodeIds.value.length === 0) {
    message.warning($gettext('Please select at least one node'))
    return
  }

  const otpModal = use2FAModal()

  otpModal.open().then(() => {
    loading.value = true
    config.syncDirectory(dir.value, syncNodeIds.value, syncOverwrite.value)
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
    :title="$gettext('Deploy Directory')"
    :confirm-loading="loading"
    @ok="ok"
  >
    <AAlert
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('All configuration files in %{name} will be deployed to the selected nodes, and files created here later will follow the same targets.', { name })"
    />
    <NodeSelector
      v-model:target="syncNodeIds"
      hidden-local
    />
    <div class="flex items-center justify-end mt-3">
      <ACheckbox v-model:checked="syncOverwrite">
        {{ $gettext('Overwrite') }}
      </ACheckbox>
      <ATooltip placement="bottom">
        <template #title>
          {{ $gettext('Overwrite exist file') }}
        </template>
        <InfoCircleOutlined />
      </ATooltip>
    </div>
  </AModal>
</template>

<style scoped lang="less">
</style>
