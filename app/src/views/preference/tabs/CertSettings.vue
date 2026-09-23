<script setup lang="ts">
import { DeleteOutlined, HolderOutlined } from '@antdv-next/icons'
import Draggable from 'vuedraggable'
import { CA_SERVER_OPTIONS } from '@/constants/acme'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
</script>

<template>
  <AForm layout="vertical" class="max-w-150">
    <AFormItem :label="$gettext('Email')">
      <p>{{ data.cert.email }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('HTTP Challenge Port')">
      <AInputNumber v-model:value="data.cert.http_challenge_port" class="w-30" />
    </AFormItem>
    <AFormItem
      :label="$gettext('CADir')"
      :validate-status="errors?.cert?.ca_dir ? 'error' : ''"
      :help="errors?.cert?.ca_dir === 'url'
        ? $gettext('The url is invalid')
        : ''"
    >
      <AAutoComplete
        v-model:value="data.cert.ca_dir"
        :options="CA_SERVER_OPTIONS"
        :placeholder="$gettext('Select or enter a CA directory URL')"
        allow-clear
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('Certificate Renewal Threshold')"
      :help="$gettext('Renew certificates when their remaining validity is less than or equal to this value.')"
    >
      <ASpaceCompact>
        <AInputNumber
          v-model:value="data.cert.renewal_interval"
          :min="1"
          :max="90"
          class="w-30"
        />
        <ASpaceAddon>{{ $gettext('Days') }}</ASpaceAddon>
      </ASpaceCompact>
    </AFormItem>
    <AFormItem
      :help="$gettext('Set the recursive nameservers to override the systems nameservers '
        + 'for the step of DNS challenge.')"
    >
      <template #label>
        {{ $gettext('Recursive Nameservers') }}
        <AButton
          type="link"
          @click="data.cert.recursive_nameservers.push('')"
        >
          {{ $gettext('Add') }}
        </AButton>
      </template>

      <Draggable
        :list="data.cert.recursive_nameservers"
        item-key="name"
        class="list-group"
        ghost-class="ghost"
        handle=".anticon-holder"
      >
        <template #item="{ index }">
          <AFlex
            align="center"
            gap="small"
            class="mb-2 max-w-100"
          >
            <HolderOutlined class="cursor-move p-1" />
            <AInput
              v-model:value="data.cert.recursive_nameservers[index]"
              :status="errors?.cert?.recursive_nameservers?.[index] ? 'error' : undefined"
              placeholder="8.8.8.8:53"
            />
            <APopconfirm
              :title="$gettext('Are you sure you want to remove this item?')"
              :ok-text="$gettext('Yes')"
              :cancel-text="$gettext('No')"
              @confirm="data.cert.recursive_nameservers.splice(index, 1)"
            >
              <AButton
                type="link"
                danger
              >
                <DeleteOutlined />
              </AButton>
            </APopconfirm>
          </AFlex>
        </template>
      </Draggable>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
