<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { CopyOutlined } from '@antdv-next/icons'
import { useClipboard } from '@vueuse/core'
import NodeSelector from '@/components/NodeSelector'

interface Props {
  data: Cert
  errors?: Record<string, string>
  isManaged: boolean
  showNameField?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showNameField: true,
})

const { message } = App.useApp()

// Use defineModel for two-way binding
const data = defineModel<Cert>('data', { required: true })

const { copy } = useClipboard()

async function copyToClipboard(text: string, label: string) {
  if (!text) {
    message.warning($gettext('Nothing to copy'))
    return
  }
  try {
    await copy(text)
    message.success($gettext(`{label} copied to clipboard`).replace('{label}', label))
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to copy to clipboard'))
  }
}
</script>

<template>
  <AForm
    layout="vertical"
    :model="data"
    class="basic-info-form"
  >
    <AFormItem
      v-if="props.showNameField && !isManaged"
      name="name"
      :label="$gettext('Name')"
      :validate-status="errors?.name ? 'error' : ''"
      :help="errors?.name?.includes('required')
        ? $gettext('This field is required')
        : ''"
    >
      <div class="input-with-copy">
        <AInput v-model:value="data.name" />
        <AButton
          v-if="data.name"
          type="text"
          size="small"
          class="copy-button"
          @click="copyToClipboard(data.name, $gettext('Name'))"
        >
          <CopyOutlined />
        </AButton>
      </div>
    </AFormItem>

    <ACard size="small" class="sync-target-card" :title="$gettext('Sync to')">
      <AFormItem class="mb-0">
        <NodeSelector
          v-model:target="data.sync_node_ids"
          hidden-local
        />
      </AFormItem>
    </ACard>
  </AForm>
</template>

<style scoped lang="less">
.basic-info-form {
  width: 100%;
}

.copy-container {
  display: flex;
  align-items: center;
  gap: 8px;

  .copy-text {
    margin: 0;
    flex: 1;
    word-break: break-all;
  }
}

.input-with-copy {
  display: flex;
  align-items: center;
  gap: 8px;

  .ant-input {
    flex: 1;
  }

  .copy-button {
    flex-shrink: 0;
  }
}

.sync-target-card {
  margin-top: 4px;
  width: 100%;
}

.sync-target-card :deep(.ant-card-body) {
  padding: 12px;
}
</style>
