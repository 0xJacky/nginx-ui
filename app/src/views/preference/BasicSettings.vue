<script setup lang="ts">
import { inject } from 'vue'
import Draggable from 'vuedraggable'
import { DeleteOutlined, HolderOutlined } from '@ant-design/icons-vue'
import type { Settings } from '@/views/preference/typedef'

const data: Settings = inject('data') as Settings
const errors: Record<string, Record<string, string>> = inject('errors') as Record<string, Record<string, string>>
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('HTTP Host')">
      <p>{{ data.server.http_host }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('HTTP Port')">
      <p>{{ data.server.http_port }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Run Mode')">
      <p>{{ data.server.run_mode }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Jwt Secret')">
      <p>{{ data.server.jwt_secret }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Node Secret')">
      <p>{{ data.server.node_secret }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('Terminal Start Command')">
      <p>{{ data.server.start_cmd }}</p>
    </AFormItem>
    <AFormItem :label="$gettext('HTTP Challenge Port')">
      <AInputNumber v-model:value="data.server.http_challenge_port" />
    </AFormItem>
    <AFormItem
      :label="$gettext('Github Proxy')"
      :validate-status="errors?.server?.github_proxy ? 'error' : ''"
      :help="errors?.server?.github_proxy === 'url'
        ? $gettext('The url is not valid')
        : ''"
    >
      <AInput
        v-model:value="data.server.github_proxy"
        :placeholder="$gettext('For Chinese user: https://mirror.ghproxy.com/')"
      />
    </AFormItem>
    <AFormItem
      :label="$gettext('CADir')"
      :validate-status="errors?.server?.ca_dir ? 'error' : ''"
      :help="errors?.server?.ca_dir === 'url'
        ? $gettext('The url is not valid')
        : ''"
    >
      <AInput v-model:value="data.server.ca_dir" />
    </AFormItem>
    <AFormItem :label="$gettext('Certificate Renewal Interval')">
      <AInputNumber
        v-model:value="data.server.cert_renewal_interval"
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
          @click="data.server.recursive_nameservers.push('')"
        >
          {{ $gettext('Add') }}
        </AButton>
      </template>

      <Draggable
        :list="data.server.recursive_nameservers"
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
                v-model:value="data.server.recursive_nameservers[index]"
                :status="errors?.server?.recursive_nameservers?.[index] ? 'error' : undefined"
                placeholder="8.8.8.8:53"
                class="mb-4"
              />
            </ACol>
            <ACol :span="2">
              <APopconfirm
                :title="$gettext('Are you sure you want to remove this item?')"
                :ok-text="$gettext('Yes')"
                :cancel-text="$gettext('No')"
                @confirm="data.server.recursive_nameservers.splice(index, 1)"
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
