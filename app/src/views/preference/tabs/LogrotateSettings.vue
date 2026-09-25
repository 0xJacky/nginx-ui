<script setup lang="ts">
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
</script>

<template>
  <div v-if="data?.logrotate">
    <SettingPanel
      :title="$gettext('Logrotate')"
      :description="$gettext('Logrotate, by default, is enabled in most mainstream Linux distributions for users who install Nginx UI on the host machine, so you don\'t need to modify the parameters on this page. For users who install Nginx UI using Docker containers, you can manually enable this option. The crontab task scheduler of Nginx UI will execute the logrotate command at the interval you set in minutes.')"
    >
      <SettingRow
        :title="$gettext('Enable Logrotate')"
        :description="$gettext('Runs the rotation command on a schedule from Nginx UI. Mainly needed inside a Docker container.')"
        path="logrotate.enabled"
      >
        <ASwitch v-model:checked="data.logrotate.enabled" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Command')"
        path="logrotate.cmd"
        config-file="logrotate"
        :value="data.logrotate.cmd"
      />
      <SettingRow
        :title="$gettext('Interval')"
        :description="$gettext('Minutes between two runs.')"
        path="logrotate.interval"
      >
        <ASpaceCompact>
          <AInputNumber
            v-model:value="data.logrotate.interval"
            :min="1"
            class="w-30"
          />
          <ASpaceAddon>{{ $gettext('Minutes') }}</ASpaceAddon>
        </ASpaceCompact>
      </SettingRow>
    </SettingPanel>
  </div>
</template>
