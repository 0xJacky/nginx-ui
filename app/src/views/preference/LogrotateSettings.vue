<script setup lang="ts">
import type { Settings } from '@/api/settings'

const data: Settings = inject('data')!
</script>

<template>
  <AForm layout="vertical">
    <AAlert
      class="mb-4"
      :message="$gettext('Tips')"
      :description="$gettext('Logrotate, by default, is enabled in most mainstream '
        + 'Linux distributions for users who install Nginx UI on the host machine, '
        + 'so you don\'t need to modify the parameters on this page. '
        + 'For users who install Nginx UI using Docker containers, you can manually enable this option. '
        + 'The crontab task scheduler of Nginx UI will execute the logrotate command at the interval you set in minutes.')"
      type="info"
    />
    <AFormItem :label="$gettext('Enabled')">
      <ASwitch v-model:checked="data.logrotate.enabled" />
    </AFormItem>
    <AFormItem :label="$gettext('Command')">
      {{ data.logrotate.cmd }}
    </AFormItem>
    <AFormItem :label="$gettext('Interval')">
      <AInputNumber
        v-model:value="data.logrotate.interval"
        :min="1"
        :addon-after="$gettext('Minutes')"
      />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>

</style>
