<script setup lang="tsx">
import type { Ref } from 'vue'

import type { BannedIP } from '@/api/settings'
import dayjs from 'dayjs'
import setting from '@/api/settings'
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const { message } = App.useApp()

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
  render: value => {
    return dayjs.unix(value).format('YYYY-MM-DD HH:mm:ss')
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
    <SettingPanel :title="$gettext('MFA Policy')" :description="$gettext('Require MFA at the next sign-in. Users must enroll their own authenticator or passkey.')">
      <SettingRow :title="$gettext('Require MFA for all users')" path="auth.mfa_required">
        <ASwitch v-model:checked="data.auth.mfa_required" />
      </SettingRow>
      <SettingRow :title="$gettext('Require local MFA for SSO sign-in')" :description="$gettext('Apply local MFA verification and enrollment requirements to OIDC and Casdoor sign-in.')" path="auth.mfa_required_for_sso">
        <ASwitch v-model:checked="data.auth.mfa_required_for_sso" />
      </SettingRow>
    </SettingPanel>
    <SettingPanel
      v-if="data.webauthn.rpid
        && data.webauthn.rp_display_name
        && data.webauthn.rp_origins?.length > 0"
      :title="$gettext('Passkeys')"
    >
      <SettingRow
        :title="$gettext('RPID')"
        path="webauthn.rpid"
        config-file="webauthn"
        :value="data.webauthn.rpid"
      />
      <SettingRow
        :title="$gettext('RP Display Name')"
        path="webauthn.rp_display_name"
        config-file="webauthn"
        :value="data.webauthn.rp_display_name"
      />
      <SettingRow
        :title="$gettext('RP Origins')"
        path="webauthn.rp_origins"
        config-file="webauthn"
      >
        <div class="text-right text-gray-500">
          <div
            v-for="origin in data.webauthn.rp_origins"
            :key="origin"
          >
            {{ origin }}
          </div>
        </div>
      </SettingRow>
    </SettingPanel>

    <SettingPanel
      :title="$gettext('Throttle')"
      :description="$gettext('If the number of login failed attempts from a ip reach the max attempts in ban threshold minutes, the ip will be banned for a period of time.')"
    >
      <SettingRow
        :title="$gettext('Ban Threshold Minutes')"
        :description="$gettext('Window in which failed sign-in attempts from one address are counted.')"
        path="auth.ban_threshold_minutes"
      >
        <AInputNumber
          v-model:value="data.auth.ban_threshold_minutes"
          :min="1"
          class="w-30"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Max Attempts')"
        :description="$gettext('Failed attempts allowed inside the window before the address is banned for a while.')"
        path="auth.max_attempts"
      >
        <AInputNumber
          v-model:value="data.auth.max_attempts"
          :min="1"
          class="w-30"
        />
      </SettingRow>
    </SettingPanel>

    <SettingPanel
      :title="$gettext('Banned IPs')"
      :description="$gettext('Addresses that are currently blocked from signing in.')"
    >
      <div
        class="pb-3"
        data-setting-path="auth.banned_ips"
      >
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
    </SettingPanel>
  </div>
</template>
