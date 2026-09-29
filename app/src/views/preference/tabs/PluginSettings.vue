<script setup lang="ts">
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import TrustedPublishers from '../components/TrustedPublishers.vue'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const syncPolicyOptions = computed(() => [
  { value: 'manual', label: $gettext('Manual') },
  { value: 'auto', label: $gettext('Automatic') },
])

const resourceLimitsSupported = computed(() => data.value.plugin?.resource_limits_supported !== false)

// 0 means unlimited, shown as an empty field with a placeholder.
function limitModel(key: 'memory_limit_mb' | 'cpu_percent') {
  return computed<number | null>({
    get: () => {
      const value = data.value.plugin?.[key] ?? 0
      return value > 0 ? value : null
    },
    set: value => {
      if (data.value.plugin)
        data.value.plugin[key] = value ?? 0
    },
  })
}

const memoryLimit = limitModel('memory_limit_mb')
const cpuLimit = limitModel('cpu_percent')
</script>

<template>
  <div v-if="data.plugin">
    <SettingPanel :title="$gettext('General')">
      <SettingRow
        :title="$gettext('Plugin System')"
        :description="$gettext('Turning it off stops every plugin and hides the plugin pages.')"
        path="plugin.enabled"
        requires-restart
      >
        <ASwitch v-model:checked="data.plugin.enabled" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Plugin Directory')"
        path="plugin.dir"
        config-file="plugin"
        :value="data.plugin.dir"
      />
      <SettingRow
        :title="$gettext('Default Sync Policy')"
        :description="$gettext('Applied to newly installed plugins. Automatic keeps the plugin installed on the child nodes.')"
        path="plugin.default_sync_policy"
      >
        <ASelect
          v-model:value="data.plugin.default_sync_policy"
          :options="syncPolicyOptions"
          class="w-40"
        />
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Marketplace')">
      <SettingRow
        :title="$gettext('Enable Marketplace')"
        path="plugin.marketplace_enabled"
      >
        <ASwitch v-model:checked="data.plugin.marketplace_enabled" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Sources')"
        :description="$gettext('Managed on the marketplace page. Empty means the official catalog.')"
        path="plugin.marketplace_sources"
      >
        <div class="plugin-source-list">
          <span v-if="data.plugin.marketplace_sources.length === 0">{{ $gettext('Official catalog') }}</span>
          <span
            v-for="source in data.plugin.marketplace_sources"
            :key="source"
          >{{ source }}</span>
          <RouterLink :to="{ path: '/system/plugins', query: { tab: 'marketplace' } }">
            {{ $gettext('Manage') }}
          </RouterLink>
        </div>
      </SettingRow>
      <SettingRow
        :title="$gettext('Allow Community Plugins')"
        :description="$gettext('Community plugins are published by third parties and ask for a confirmation before they install.')"
        path="plugin.allow_community_plugins"
      >
        <ASwitch v-model:checked="data.plugin.allow_community_plugins" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Automatic Updates')"
        :description="$gettext('Updates official and partner plugins on their own while the permissions they ask for stay the same.')"
        path="plugin.auto_update"
      >
        <ASwitch v-model:checked="data.plugin.auto_update" />
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Packages')">
      <SettingRow
        :title="$gettext('Allow Uploads')"
        :description="$gettext('Allows installing packages uploaded from the browser or the command line.')"
        path="plugin.allow_uploads"
      >
        <ASwitch v-model:checked="data.plugin.allow_uploads" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Trusted Publishers')"
        :description="$gettext('Plugins from these publishers install as community plugins.')"
        path="plugin.trusted_public_keys"
        stacked
      >
        <TrustedPublishers v-model="data.plugin.trusted_public_keys" />
      </SettingRow>
    </SettingPanel>

    <SettingPanel
      :title="$gettext('Resources')"
      :description="resourceLimitsSupported
        ? $gettext('Applies to every plugin. A plugin can only lower its own limits.')
        : $gettext('Only applies on Linux. This system does not support it.')"
    >
      <SettingRow
        :title="$gettext('Memory Limit')"
        :description="$gettext('Maximum memory each plugin can use.')"
        path="plugin.memory_limit_mb"
      >
        <AInputNumber
          v-model:value="memoryLimit"
          :min="0"
          :disabled="!resourceLimitsSupported"
          :placeholder="$gettext('Unlimited')"
          suffix="MiB"
          class="w-40"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('CPU Limit')"
        :description="$gettext('Share of one CPU core each plugin can use.')"
        path="plugin.cpu_percent"
      >
        <AInputNumber
          v-model:value="cpuLimit"
          :min="0"
          :disabled="!resourceLimitsSupported"
          :placeholder="$gettext('Unlimited')"
          suffix="%"
          class="w-40"
        />
      </SettingRow>
    </SettingPanel>

    <SettingPanel
      :title="$gettext('Advanced')"
      :description="$gettext('These options make plugin installs less safe. Turn them on only when needed.')"
    >
      <SettingRow
        :title="$gettext('Allow Insecure Download URLs')"
        :description="$gettext('Accepts plain http catalog and download addresses. Only for a private catalog on a trusted network.')"
        path="plugin.allow_insecure_download_url"
      >
        <template #tags>
          <ATag v-if="data.plugin.allow_insecure_download_url" color="warning" class="me-0 font-normal">
            {{ $gettext('Less safe') }}
          </ATag>
        </template>
        <ASwitch v-model:checked="data.plugin.allow_insecure_download_url" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Developer Mode')"
        :description="$gettext('Allows installing unsigned plugins. Only turn this on while developing a plugin.')"
        path="plugin.developer_mode"
      >
        <template #tags>
          <ATag v-if="data.plugin.developer_mode" color="warning" class="me-0 font-normal">
            {{ $gettext('Less safe') }}
          </ATag>
        </template>
        <ASwitch v-model:checked="data.plugin.developer_mode" />
      </SettingRow>
    </SettingPanel>
  </div>
</template>

<style lang="less" scoped>
.plugin-source-list {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  overflow-wrap: anywhere;
  color: var(--ant-color-text-secondary);
}

@media (max-width: 512px) {
  .plugin-source-list {
    align-items: flex-start;
  }
}
</style>
