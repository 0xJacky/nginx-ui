<script setup lang="ts">
import { DeleteOutlined, HolderOutlined } from '@antdv-next/icons'
import Draggable from 'vuedraggable'
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import { CA_SERVER_OPTIONS } from '@/constants/acme'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
</script>

<template>
  <div>
    <SettingPanel :title="$gettext('Issuing')">
      <SettingRow
        :title="$gettext('Email')"
        :description="$gettext('Contact address registered with the certificate authority.')"
        path="cert.email"
        config-file="cert"
        :value="data.cert.email"
      />
      <SettingRow
        :title="$gettext('CADir')"
        :description="$gettext('Service that issues certificates. Leave empty to use the default.')"
        path="cert.ca_dir"
        :error="errors?.cert?.ca_dir === 'url' ? $gettext('The url is invalid') : undefined"
      >
        <AAutoComplete
          v-model:value="data.cert.ca_dir"
          :options="CA_SERVER_OPTIONS"
          :placeholder="$gettext('Select or enter a CA directory URL')"
          :status="errors?.cert?.ca_dir ? 'error' : undefined"
          allow-clear
          class="w-80"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('HTTP Challenge Port')"
        :description="$gettext('Port that answers validation requests while a certificate is being issued.')"
        path="cert.http_challenge_port"
      >
        <AInputNumber v-model:value="data.cert.http_challenge_port" class="w-30" />
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Renewal')">
      <SettingRow
        :title="$gettext('Certificate Renewal Threshold')"
        :description="$gettext('Renew certificates when their remaining validity is less than or equal to this value.')"
        path="cert.renewal_interval"
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
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('DNS Challenge')">
      <SettingRow
        :title="$gettext('Recursive Nameservers')"
        :description="$gettext('Set the recursive nameservers to override the systems nameservers for the step of DNS challenge.')"
        path="cert.recursive_nameservers"
        stacked
      >
        <template #tags>
          <AButton
            type="link"
            size="small"
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
      </SettingRow>
    </SettingPanel>
  </div>
</template>
