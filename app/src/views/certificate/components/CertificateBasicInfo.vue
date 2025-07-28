<script setup lang="ts">
import type { Cert } from '@/api/cert'
import NodeSelector from '@/components/NodeSelector'

interface Props {
  data: Cert
  errors: Record<string, string>
  isManaged: boolean
}

defineProps<Props>()

// Use defineModel for two-way binding
const data = defineModel<Cert>('data', { required: true })
</script>

<template>
  <AForm
    layout="vertical"
    style="max-width: 600px"
  >
    <AFormItem
      :label="$gettext('Name')"
      :validate-status="errors.name ? 'error' : ''"
      :help="errors.name === 'required'
        ? $gettext('This field is required')
        : ''"
    >
      <p v-if="isManaged">
        {{ data.name }}
      </p>
      <AInput
        v-else
        v-model:value="data.name"
      />
    </AFormItem>

    <AFormItem
      :label="$gettext('SSL Certificate Path')"
      :validate-status="errors.ssl_certificate_path ? 'error' : ''"
      :help="errors.ssl_certificate_path === 'required' ? $gettext('This field is required')
        : errors.ssl_certificate_path === 'certificate_path'
          ? $gettext('The path exists, but the file is not a certificate') : ''"
    >
      <p v-if="isManaged">
        {{ data.ssl_certificate_path }}
      </p>
      <AInput
        v-else
        v-model:value="data.ssl_certificate_path"
      />
    </AFormItem>

    <AFormItem
      :label="$gettext('SSL Certificate Key Path')"
      :validate-status="errors.ssl_certificate_key_path ? 'error' : ''"
      :help="errors.ssl_certificate_key_path === 'required' ? $gettext('This field is required')
        : errors.ssl_certificate_key_path === 'privatekey_path'
          ? $gettext('The path exists, but the file is not a private key') : ''"
    >
      <p v-if="isManaged">
        {{ data.ssl_certificate_key_path }}
      </p>
      <AInput
        v-else
        v-model:value="data.ssl_certificate_key_path"
      />
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
</style>
