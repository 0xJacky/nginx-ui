<script setup lang="ts">
import type { SelfSignedCertPayload } from '@/api/cert'
import NodeSelector from '@/components/NodeSelector'
import { PrivateKeyTypeList } from '@/constants'

const data = defineModel<SelfSignedCertPayload>({ required: true })
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Name')">
      <AInput
        v-model:value="data.name"
        :placeholder="$gettext('Optional')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Domains')">
      <ASelect
        v-model:value="data.domains"
        mode="tags"
        :open="false"
        :token-separators="[',', ' ']"
        :placeholder="$gettext('Enter domain names')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('IP Addresses')">
      <ASelect
        v-model:value="data.ip_addresses"
        mode="tags"
        :open="false"
        :token-separators="[',', ' ']"
        :placeholder="$gettext('Enter IP addresses')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Key Type')">
      <ASelect v-model:value="data.key_type">
        <ASelectOption
          v-for="t in PrivateKeyTypeList"
          :key="t.key"
          :value="t.key"
        >
          {{ t.name }}
        </ASelectOption>
      </ASelect>
    </AFormItem>
    <AFormItem :label="$gettext('Valid For (days)')">
      <AInputNumber
        v-model:value="data.validity_days"
        :min="1"
        :max="3650"
        class="w-full"
      />
      <template #help>
        {{ $gettext('Some browsers reject TLS certificates valid for more than 398 days.') }}
      </template>
    </AFormItem>
    <AFormItem :label="$gettext('Sync to')">
      <NodeSelector
        v-model:target="data.sync_node_ids"
        hidden-local
      />
    </AFormItem>
  </AForm>
</template>
