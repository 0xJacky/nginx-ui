<script setup lang="ts">
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
</script>

<template>
  <AForm layout="vertical">
    <AAlert
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('Global health check controls')"
      :description="$gettext('A global pause stops network probes without changing individual site or upstream selections. Discovery remains active so configured targets stay visible.')"
    />

    <ADivider orientation="left">
      {{ $gettext('Sites') }}
    </ADivider>
    <AFormItem :label="$gettext('Enable site health checks')">
      <ASwitch v-model:checked="data.site_check.enabled" data-testid="site-check-global-enabled" />
    </AFormItem>
    <AFormItem :label="$gettext('Concurrency')">
      <AInputNumber v-model:value="data.site_check.concurrency" :min="1" :max="20" />
    </AFormItem>
    <AFormItem :label="$gettext('Interval')">
      <AInputNumber
        v-model:value="data.site_check.interval_seconds"
        :min="30"
        :addon-after="$gettext('Seconds')"
      />
    </AFormItem>

    <ADivider orientation="left">
      {{ $gettext('Proxy Targets') }}
    </ADivider>
    <AFormItem :label="$gettext('Enable upstream health checks')">
      <ASwitch v-model:checked="data.upstream_check.enabled" data-testid="upstream-check-global-enabled" />
    </AFormItem>
    <AFormItem :label="$gettext('Interval')">
      <AInputNumber
        v-model:value="data.upstream_check.interval_seconds"
        :min="5"
        :addon-after="$gettext('Seconds')"
      />
    </AFormItem>
  </AForm>
</template>
