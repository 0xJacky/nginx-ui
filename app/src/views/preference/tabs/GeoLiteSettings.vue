<script setup lang="ts">
import GeoLiteDownload from '@/components/GeoLiteDownload'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const customMMDBPath = computed(() => data.value.nginx_log?.index_custom_mmdb?.trim() || '')
const geoMapPath = computed(() => data.value.nginx_log?.geo_map_path?.trim() || '')
const displayGeoMapPath = computed(() => geoMapPath.value || 'maps')
const customMMDBTemplateURL = 'https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb'
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
  <AForm layout="vertical" class="max-w-150">
    <AAlert
      v-if="isCustomMMDBEnabled"
      class="mb-4"
      type="info"
      show-icon
      :title="$gettext('Custom MMDB is currently enabled')"
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

    <AFormItem :label="$gettext('Map Boundary Directory')">
      <ATypographyParagraph class="mb-1!" :ellipsis="{ tooltip: true }">
        {{ displayGeoMapPath }}
      </ATypographyParagraph>
      <ATypographyText type="secondary">
        {{ $gettext('Configured from [nginx_log].GeoMapPath. Keep files with names like 100000_full.json in this directory.') }}
      </ATypographyText>
      <br>
      <ATypographyText type="secondary">
        {{ $gettext('Only the world map is provided by default. Please download China and province boundary files yourself and place them in this directory.') }}
      </ATypographyText>
      <br>
      <ATypographyText type="secondary">
        <ATypographyLink
          :href="customMMDBTemplateURL"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ $gettext('Template reference') }}
        </ATypographyLink>
      </ATypographyText>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>
</style>
