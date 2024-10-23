<script setup lang="ts">
import Draggable from 'vuedraggable'
import { DeleteOutlined, HolderOutlined } from '@ant-design/icons-vue'
import type { Settings } from '@/api/settings'

const data: Settings = inject('data') as Settings
const errors: Record<string, Record<string, string>> = inject('errors') as Record<string, Record<string, string>>
</script>

<template>
  <AForm layout="vertical">
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
      <AInput v-model:value="data.cert.ca_dir" />
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
  </AForm>
</template>

<style lang="less" scoped>

</style>
