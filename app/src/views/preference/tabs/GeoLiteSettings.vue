<script setup lang="ts">
import GeoLiteDownload from '@/components/GeoLiteDownload'
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
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
  <div>
    <SettingPanel :title="$gettext('Database')">
      <SettingRow
        :title="$gettext('GeoLite2 Database')"
        :description="$gettext('The GeoLite2 database provides geographic information for IP addresses. This is used for offline geographic analysis in log analytics.')"
        path="nginx_log.geolite_database"
        stacked
      >
        <GeoLiteDownload :hide-redownload="isCustomMMDBEnabled" />
      </SettingRow>
      <SettingRow
        v-if="isCustomMMDBEnabled"
        :title="$gettext('Custom MMDB')"
        :description="$gettext('A custom database replaces the GeoLite2 download.')"
        path="nginx_log.index_custom_mmdb"
        config-file="nginx_log"
        :value="customMMDBFileName"
      />
    </SettingPanel>

    <SettingPanel :title="$gettext('Maps')">
      <SettingRow
        :title="$gettext('Map Boundary Directory')"
        path="nginx_log.geo_map_path"
        config-file="nginx_log"
        :value="displayGeoMapPath"
      >
        <template #description>
          <div>{{ $gettext('Keep files with names like 100000_full.json in this directory. Only the world map is provided by default.') }}</div>
          <div>{{ $gettext('Please download China and province boundary files yourself and place them in this directory.') }}</div>
          <ATypographyLink
            :href="customMMDBTemplateURL"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ $gettext('Template reference') }}
          </ATypographyLink>
        </template>
      </SettingRow>
    </SettingPanel>
  </div>
</template>
