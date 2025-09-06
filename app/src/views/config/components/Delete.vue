<script setup lang="ts">
import config from '@/api/config'
import NodeSelector from '@/components/NodeSelector'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { urlJoin } from '@/lib/helper'
import { isProtectedPath } from '@/views/config/configUtils'

const emit = defineEmits(['deleted'])
const { message } = useGlobalApp()
const visible = ref(false)
const confirmText = ref('')

const data = ref({
  basePath: '',
  name: '',
  isDir: false,
  sync_node_ids: [] as number[],
  fullPath: '',
})

async function open(basePath: string, name: string, isDir: boolean) {
  visible.value = true
  confirmText.value = ''
  data.value.basePath = basePath
  data.value.name = name
  data.value.isDir = isDir
  data.value.sync_node_ids = []

  const { base_path: configBasePath } = await config.get_base_path()

  // Build full path
  const relativePath = urlJoin(basePath, name)
  data.value.fullPath = urlJoin(configBasePath, relativePath)

  // Load config details to get sync nodes
  try {
    // For files, try to get their specific sync configuration
    if (!isDir) {
      const configDetail = await config.getItem(relativePath)
      if (configDetail?.sync_node_ids && configDetail.sync_node_ids.length > 0) {
        data.value.sync_node_ids = [...configDetail.sync_node_ids]
      }
    }
    // For directories, we could potentially get sync nodes from any file within
    // but for simplicity, we'll leave it empty and let user choose
  }
  catch (error) {
    // Silently ignore errors if config details cannot be loaded
    // This might happen for files that are not tracked in the database
    console.error('Config details not available for sync nodes:', error)
  }
}

defineExpose({
  open,
})

// Check if the item is protected
const isProtected = computed(() => {
  return isProtectedPath(data.value.name)
})

// Expected confirmation text
const expectedConfirmText = computed(() => {
  return $gettext('Delete')
})

function ok() {
  if (confirmText.value !== expectedConfirmText.value) {
    message.error($gettext('Please type the exact confirmation text'))
    return
  }

  const { basePath, name, sync_node_ids } = data.value
  const otpModal = use2FAModal()

  otpModal.open().then(() => {
    config.delete(basePath, name, sync_node_ids).then(() => {
      visible.value = false
      message.success($gettext('Deleted successfully'))
      emit('deleted')
    })
  })
}

function cancel() {
  visible.value = false
  confirmText.value = ''
}
</script>

<template>
  <AModal
    v-model:open="visible"
    :mask="false"
    :title="$gettext('Delete Confirmation')"
    :ok-text="$gettext('Delete')"
    :cancel-text="$gettext('Cancel')"
    :ok-button-props="{ danger: true, disabled: confirmText !== expectedConfirmText || isProtected }"
    @ok="ok"
    @cancel="cancel"
  >
    <AForm layout="vertical" class="delete-modal-content">
      <AAlert
        v-if="isProtected"
        type="error"
        :message="$gettext('Protected Directory')"
        :description="$gettext('This directory is protected and cannot be deleted for system safety.')"
        show-icon
        class="mb-4"
      />

      <AAlert
        v-else
        type="warning"
        :message="$gettext('This will permanently delete the %{type}.', { type: data.isDir ? $gettext('folder') : $gettext('file') })"
        show-icon
        class="mb-4"
      />

      <div class="item-info mb-4">
        <p><strong>{{ $gettext('Type') }}:</strong> {{ data.isDir ? $gettext('Folder') : $gettext('File') }}</p>
        <p><strong>{{ $gettext('Name') }}:</strong> {{ data.name }}</p>
        <p><strong>{{ $gettext('Path') }}:</strong> {{ data.fullPath }}</p>
      </div>

      <AFormItem
        v-if="!isProtected"
        :label="$gettext('Type %{delete} to confirm', { delete: expectedConfirmText })"
        class="mb-4"
      >
        <AInput
          v-model:value="confirmText"
          :placeholder="expectedConfirmText"
          :disabled="isProtected"
        />
      </AFormItem>

      <AFormItem
        v-if="!isProtected"
        :label="$gettext('Sync')"
      >
        <NodeSelector
          v-model:target="data.sync_node_ids"
          hidden-local
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>

<style scoped lang="less">
.delete-modal-content {
  .item-info {
    background-color: #fafafa;
    padding: 12px;
    border-radius: 6px;
    border: 1px solid #e8e8e8;

    p {
      margin: 0;
      line-height: 1.5;

      &:not(:last-child) {
        margin-bottom: 8px;
      }
    }
  }
}
</style>
