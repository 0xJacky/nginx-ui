<script setup lang="ts">
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
</script>

<template>
  <div>
    <SettingPanel
      :title="$gettext('Sites')"
      :description="$gettext('A global pause stops network probes without changing individual site or upstream selections. Discovery remains active so configured targets stay visible.')"
    >
      <SettingRow
        :title="$gettext('Enable site health checks')"
        path="site_check.enabled"
      >
        <ASwitch
          v-model:checked="data.site_check.enabled"
          data-testid="site-check-global-enabled"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Concurrency')"
        :description="$gettext('Sites checked at the same time.')"
        path="site_check.concurrency"
      >
        <AInputNumber
          v-model:value="data.site_check.concurrency"
          :min="1"
          :max="20"
          class="w-30"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Interval')"
        :description="$gettext('Time between two checks of the same site.')"
        path="site_check.interval_seconds"
      >
        <ASpaceCompact>
          <AInputNumber
            v-model:value="data.site_check.interval_seconds"
            :min="30"
            class="w-30"
          />
          <ASpaceAddon>{{ $gettext('Seconds') }}</ASpaceAddon>
        </ASpaceCompact>
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Proxy Targets')">
      <SettingRow
        :title="$gettext('Enable upstream health checks')"
        path="upstream_check.enabled"
      >
        <ASwitch
          v-model:checked="data.upstream_check.enabled"
          data-testid="upstream-check-global-enabled"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('Interval')"
        :description="$gettext('Time between two checks of the same proxy target.')"
        path="upstream_check.interval_seconds"
      >
        <ASpaceCompact>
          <AInputNumber
            v-model:value="data.upstream_check.interval_seconds"
            :min="5"
            class="w-30"
          />
          <ASpaceAddon>{{ $gettext('Seconds') }}</ASpaceAddon>
        </ASpaceCompact>
      </SettingRow>
    </SettingPanel>
  </div>
</template>
