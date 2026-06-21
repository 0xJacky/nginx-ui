<script setup lang="ts">
import { DeleteOutlined, HolderOutlined } from '@ant-design/icons-vue'
import Draggable from 'vuedraggable'
import { CA_SERVER_OPTIONS } from '@/constants/acme'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Email')">
      <p>{{ data.cert.email }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('HTTP Challenge Port')">
      <AInputNumber v-model:value="data.cert.http_challenge_port" />
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
    <AFormItem :label="$gettext('Certificate Renewal Interval')">
      <AInputNumber
        v-model:value="data.cert.renewal_interval"
        :min="7"
        :max="21"
        :addon-after="$gettext('Days')"
      />
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
          <ARow>
            <ACol :span="2">
              <HolderOutlined class="p-2" />
            </ACol>
            <ACol :span="20">
              <AInput
                v-model:value="data.cert.recursive_nameservers[index]"
                :status="errors?.cert?.recursive_nameservers?.[index] ? 'error' : undefined"
                placeholder="8.8.8.8:53"
                class="mb-4"
              />
            </ACol>
            <ACol :span="2">
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
            </ACol>
          </ARow>
        </template>
      </Draggable>
    </AFormItem>

    <AFormItem
      :help="$gettext('Discovery scans these glob patterns for certificate and key pairs. Only certificates not already imported by name, certificate path, key path, or fingerprint are shown.')"
    >
      <template #label>
        {{ $gettext('Discovery Patterns') }}
        <AButton
          type="link"
          @click="data.cert.discovery_patterns.push('')"
        >
          {{ $gettext('Add') }}
        </AButton>
      </template>

      <Draggable
        :list="data.cert.discovery_patterns"
        item-key="name"
        class="list-group"
        ghost-class="ghost"
        handle=".anticon-holder"
      >
        <template #item="{ index }">
          <ARow>
            <ACol :span="2">
              <HolderOutlined class="p-2" />
            </ACol>
            <ACol :span="20">
              <AInput
                v-model:value="data.cert.discovery_patterns[index]"
                placeholder="/etc/nginx/ssl/*"
                class="mb-4"
              />
            </ACol>
            <ACol :span="2">
              <APopconfirm
                :title="$gettext('Are you sure you want to remove this item?')"
                :ok-text="$gettext('Yes')"
                :cancel-text="$gettext('No')"
                @confirm="data.cert.discovery_patterns.splice(index, 1)"
              >
                <AButton
                  type="link"
                  danger
                >
                  <DeleteOutlined />
                </AButton>
              </APopconfirm>
            </ACol>
          </ARow>
        </template>
      </Draggable>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
