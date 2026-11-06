<script setup lang="ts">
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data, errors } = storeToRefs(systemSettingsStore)
</script>

<template>
  <div v-if="data?.http">
    <SettingPanel :title="$gettext('Proxies')">
      <SettingRow
        :title="$gettext('Github Proxy')"
        :description="$gettext('Used for downloads from GitHub, such as upgrades and the GeoLite database.')"
        path="http.github_proxy"
        :error="errors?.http?.github_proxy === 'url' ? $gettext('The url is invalid') : undefined"
      >
        <AInput
          v-model:value="data.http.github_proxy"
          :placeholder="$gettext('For Chinese user: https://cloud.nginxui.com/')"
          class="w-80"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('HTTP Proxy')"
        :description="$gettext('Used for other outgoing requests made by Nginx UI.')"
        path="http.http_proxy"
        :error="errors?.http?.http_proxy === 'url' ? $gettext('The url is invalid') : undefined"
      >
        <AInput
          v-model:value="data.http.http_proxy"
          placeholder="http://127.0.0.1:8080"
          class="w-80"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Insecure Skip Verify')"
        path="http.insecure_skip_verify"
        config-file="http"
      >
        <ATag :color="data.http.insecure_skip_verify ? 'green' : 'red'">
          {{ data.http.insecure_skip_verify ? $gettext('Enabled') : $gettext('Disabled') }}
        </ATag>
      </SettingRow>
    </SettingPanel>
  </div>
</template>
