<script setup lang="ts">
import SensitiveString from '@/components/SensitiveString'
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
</script>

<template>
  <div v-if="data?.app">
    <SettingPanel :title="$gettext('General')">
      <SettingRow
        :title="$gettext('Jwt Secret')"
        path="app.jwt_secret"
        config-file="app"
      >
        <SensitiveString path="app.jwt_secret" :value="data.app.jwt_secret" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Page Size')"
        path="app.page_size"
        config-file="app"
        :value="data.app.page_size"
      />
    </SettingPanel>
  </div>
</template>
