<script setup lang="ts">
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
</script>

<template>
  <div v-if="data?.terminal">
    <SettingPanel :title="$gettext('General')">
      <SettingRow
        :title="$gettext('Terminal Start Command')"
        path="terminal.start_cmd"
        config-file="terminal"
        :value="data.terminal.start_cmd"
      />
    </SettingPanel>
  </div>
</template>
