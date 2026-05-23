<script setup lang="ts">
import type { SelfSignedCertPayload } from '@/api/cert'
import NodeSelector from '@/components/NodeSelector'
import StringListInput from '@/components/StringListInput'
import { PrivateKeyTypeList } from '@/constants'

const props = defineProps<{
  isKeyTypeReadonly?: boolean
  hideRenewalNote?: boolean
}>()

const data = defineModel<SelfSignedCertPayload>({ required: true })
</script>

<template>
  <AForm layout="vertical">
    <AAlert
      v-if="!props.hideRenewalNote"
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('Nginx UI will automatically renew this certificate as it approaches expiration, based on the global certificate renewal interval and this certificate\'s validity period.')"
    />
    <AFormItem
      :label="$gettext('Name')"
      required
    >
      <AInput
        v-model:value="data.name"
        :placeholder="$gettext('Enter certificate name')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Domains')">
      <StringListInput
        v-model="data.domains"
        :placeholder="$gettext('Enter domain name')"
        :add-button-text="$gettext('Add Domain')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('IP Addresses')">
      <StringListInput
        v-model="data.ip_addresses"
        :placeholder="$gettext('Enter IP address')"
        :add-button-text="$gettext('Add IP Address')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Key Type')">
      <ASelect
        v-model:value="data.key_type"
        :disabled="props.isKeyTypeReadonly"
      >
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
