<script setup lang="tsx">
import type { Ref } from 'vue'

import type { BannedIP } from '@/api/settings'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import setting from '@/api/settings'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const bannedIPColumns = [{
  title: $gettext('IP'),
  dataIndex: 'ip',
}, {
  title: $gettext('Attempts'),
  dataIndex: 'attempts',
}, {
  title: $gettext('Banned Until'),
  dataIndex: 'expired_at',
  customRender: args => {
    return dayjs.unix(args.text).format('YYYY-MM-DD HH:mm:ss')
  },
}, {
  title: $gettext('Action'),
  dataIndex: 'action',
}]

const bannedIPs: Ref<BannedIP[]> = ref([])

function getBannedIPs() {
  setting.get_banned_ips().then(r => {
    bannedIPs.value = r
  })
}

getBannedIPs()

defineExpose({
  getBannedIPs,
})

function removeBannedIP(ip: string) {
  setting.remove_banned_ip(ip).then(() => {
    bannedIPs.value = bannedIPs.value.filter(v => v.ip !== ip)
    message.success($gettext('Remove successfully'))
  })
}
</script>

<template>
  <div>
    <h2>
      {{ $gettext('Authentication Settings') }}
    </h2>
    <div
      v-if="data.webauthn.rpid
        && data.webauthn.rp_display_name
        && data.webauthn.rp_origins?.length > 0"
      class="mb-4"
    >
      <h3>
        {{ $gettext('Webauthn') }}
      </h3>
      <div class="mb-4">
        <h4>
          {{ $gettext('RPID') }}
        </h4>
        <p>{{ data.webauthn.rpid }}</p>
      </div>
      <div class="mb-4">
        <h4>
          {{ $gettext('RP Display Name') }}
        </h4>
        <p>{{ data.webauthn.rp_display_name }}</p>
      </div>
      <div>
        <h4>
          {{ $gettext('RP Origins') }}
        </h4>
        <div
          v-for="origin in data.webauthn.rp_origins"
          :key="origin"
          class="mb-4"
        >
          {{ origin }}
        </div>
      </div>
    </div>
    <h3>{{ $gettext('Throttle') }}</h3>
    <AForm
      layout="horizontal"
    >
      <AFormItem :label="$gettext('Ban Threshold Minutes')">
        <AInputNumber
          v-model:value="data.auth.ban_threshold_minutes"
          min="1"
        />
      </AFormItem>
      <AFormItem :label="$gettext('Max Attempts')">
        <AInputNumber
          v-model:value="data.auth.max_attempts"
          min="1"
        />
      </AFormItem>
    </AForm>
    <AAlert
      class="mb-6"
      show-icon
      :message="$gettext('If the number of login failed attempts from a ip reach the max attempts in ban threshold minutes,'
        + ' the ip will be banned for a period of time.')"
      type="info"
    />
    <h3 class="mb-4">
      {{ $gettext('Banned IPs') }}
    </h3>
    <div class="mb-6">
      <ATable
        :columns="bannedIPColumns"
        row-key="ip"
        :data-source="bannedIPs"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'action'">
            <APopconfirm
              :title="$gettext('Are you sure to delete this banned IP immediately?')"
              :ok-text="$gettext('Yes')"
              :cancel-text="$gettext('No')"
              placement="bottom"
              @confirm="() => removeBannedIP(record.ip)"
            >
              <a>
                {{ $gettext('Remove') }}
              </a>
            </APopconfirm>
          </template>
        </template>
      </ATable>
    </div>
  </div>
</template>

<style lang="less" scoped>

</style>
