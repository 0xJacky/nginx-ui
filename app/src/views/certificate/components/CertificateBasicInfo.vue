<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { CopyOutlined } from '@ant-design/icons-vue'
import { useClipboard } from '@vueuse/core'
import NodeSelector from '@/components/NodeSelector'

interface Props {
  data: Cert
  errors?: Record<string, string>
  isManaged: boolean
}

defineProps<Props>()

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
    style="max-width: 600px"
  >
    <AFormItem
      :label="$gettext('Name')"
      :validate-status="errors?.name ? 'error' : ''"
      :help="errors?.name?.includes('required')
        ? $gettext('This field is required')
        : ''"
    >
      <div v-if="isManaged" class="copy-container">
        <p class="copy-text">
          {{ data.name }}
        </p>
        <AButton
          v-if="data.name"
          type="text"
          size="small"
          @click="copyToClipboard(data.name, $gettext('Name'))"
        >
          <CopyOutlined />
        </AButton>
      </div>
      <div v-else class="input-with-copy">
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

    <AFormItem
      :label="$gettext('SSL Certificate Path')"
      :validate-status="errors?.ssl_certificate_path ? 'error' : ''"
      :help="errors?.ssl_certificate_path?.includes('required') ? $gettext('This field is required')
        : errors?.ssl_certificate_path?.includes('certificate_path')
          ? $gettext('The path exists, but the file is not a certificate') : ''"
    >
      <div v-if="isManaged" class="copy-container">
        <p class="copy-text">
          {{ data.ssl_certificate_path }}
        </p>
        <AButton
          v-if="data.ssl_certificate_path"
          type="text"
          size="small"
          @click="copyToClipboard(data.ssl_certificate_path, $gettext('SSL Certificate Path'))"
        >
          <CopyOutlined />
        </AButton>
      </div>
      <div v-else class="input-with-copy">
        <AInput v-model:value="data.ssl_certificate_path" />
        <AButton
          v-if="data.ssl_certificate_path"
          type="text"
          size="small"
          class="copy-button"
          @click="copyToClipboard(data.ssl_certificate_path, $gettext('SSL Certificate Path'))"
        >
          <CopyOutlined />
        </AButton>
      </div>
    </AFormItem>

    <AFormItem
      :label="$gettext('SSL Certificate Key Path')"
      :validate-status="errors?.ssl_certificate_key_path ? 'error' : ''"
      :help="errors?.ssl_certificate_key_path?.includes('required') ? $gettext('This field is required')
        : errors?.ssl_certificate_key_path?.includes('privatekey_path')
          ? $gettext('The path exists, but the file is not a private key') : ''"
    >
      <div v-if="isManaged" class="copy-container">
        <p class="copy-text">
          {{ data.ssl_certificate_key_path }}
        </p>
        <AButton
          v-if="data.ssl_certificate_key_path"
          type="text"
          size="small"
          @click="copyToClipboard(data.ssl_certificate_key_path, $gettext('SSL Certificate Key Path'))"
        >
          <CopyOutlined />
        </AButton>
      </div>
      <div v-else class="input-with-copy">
        <AInput v-model:value="data.ssl_certificate_key_path" />
        <AButton
          v-if="data.ssl_certificate_key_path"
          type="text"
          size="small"
          class="copy-button"
          @click="copyToClipboard(data.ssl_certificate_key_path, $gettext('SSL Certificate Key Path'))"
        >
          <CopyOutlined />
        </AButton>
      </div>
    </AFormItem>

    <AFormItem :label="$gettext('Sync to')">
      <NodeSelector
        v-model:target="data.sync_node_ids"
        hidden-local
      />
    </AFormItem>
  </AForm>
</template>

<style scoped lang="less">
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
</style>
