<script setup lang="ts">
import GeoLiteDownload from '@/components/GeoLiteDownload'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const customMMDBPath = computed(() => data.value.nginx_log?.index_custom_mmdb?.trim() || '')
const customMMDBFileName = computed(() => {
  const path = customMMDBPath.value
  if (!path)
    return ''

  const normalized = path.replaceAll('\\', '/')
  return normalized.split('/').pop() || path
})
const isCustomMMDBEnabled = computed(() => customMMDBPath.value.length > 0)
</script>

<template>
  <AForm layout="vertical">
    <AAlert
      v-if="isCustomMMDBEnabled"
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('Custom MMDB is currently enabled')"
    >
      <template #description>
        <div>
          <p>{{ $gettext('IndexCustomMMDB File Name') }}: {{ customMMDBFileName }}</p>
        </div>
      </template>
    </AAlert>

    <AFormItem :label="$gettext('GeoLite2 Database')">
      <ATypographyParagraph type="secondary">
        {{ $gettext('The GeoLite2 database provides geographic information for IP addresses. This is used for offline geographic analysis in log analytics.') }}
      </ATypographyParagraph>
      <GeoLiteDownload :hide-redownload="isCustomMMDBEnabled" />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>
</style>
