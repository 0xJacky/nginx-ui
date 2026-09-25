<script setup lang="ts">
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const syncPolicyOptions = computed(() => [
  { value: 'manual', label: $gettext('Manual') },
  { value: 'auto', label: $gettext('Automatic') },
])

// The textarea holds one key per line. Blank lines and the comment line a
// minisign key file starts with are dropped, so a pasted key file works too.
const trustedKeysText = computed({
  get: () => (data.value.plugin?.trusted_public_keys ?? []).join('\n'),
  set: (value: string) => {
    if (!data.value.plugin)
      return
    data.value.plugin.trusted_public_keys = value
      .split('\n')
      .map(line => line.trim())
      .filter(line => line && !line.startsWith('untrusted comment:'))
  },
})
</script>

<template>
  <div v-if="data.plugin">
    <SettingPanel :title="$gettext('General')">
      <SettingRow
        :title="$gettext('Plugin System')"
        :description="$gettext('Turning it off stops every plugin and hides the plugin pages. Takes effect after a restart.')"
        path="plugin.enabled"
      >
        <ASwitch v-model:checked="data.plugin.enabled" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Plugin Directory')"
        path="plugin.dir"
        config-file="plugin"
        :value="data.plugin.dir || $gettext('The plugins directory under the configuration directory')"
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
      <SettingRow
        :title="$gettext('Allow Insecure Download URLs')"
        :description="$gettext('Accepts plain http catalog and download addresses. Only for a private catalog on a trusted network.')"
        path="plugin.allow_insecure_download_url"
      >
        <ASwitch v-model:checked="data.plugin.allow_insecure_download_url" />
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
        :title="$gettext('Developer Mode')"
        :description="$gettext('Allows installing unsigned plugins. Only turn this on while developing a plugin.')"
        path="plugin.developer_mode"
      >
        <ASwitch v-model:checked="data.plugin.developer_mode" />
      </SettingRow>
      <SettingRow
        :title="$gettext('Trusted Keys')"
        :description="$gettext('Packages signed with one of these keys install as community plugins. One key per line.')"
        path="plugin.trusted_public_keys"
        stacked
      >
        <ATextarea
          v-model:value="trustedKeysText"
          :rows="3"
          :placeholder="$gettext('Paste public keys here')"
          class="font-mono"
        />
      </SettingRow>
    </SettingPanel>

    <SettingPanel :title="$gettext('Resources')">
      <SettingRow
        :title="$gettext('Memory Limit')"
        :description="$gettext('Applies to every plugin process. 0 means unlimited, a plugin can only lower its own limit.')"
        path="plugin.memory_limit_mb"
      >
        <AInputNumber
          v-model:value="data.plugin.memory_limit_mb"
          :min="0"
          suffix="MiB"
          class="w-40"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('CPU Limit')"
        :description="$gettext('Percent of one core for every plugin process. 0 means unlimited.')"
        path="plugin.cpu_percent"
      >
        <AInputNumber
          v-model:value="data.plugin.cpu_percent"
          :min="0"
          suffix="%"
          class="w-40"
        />
      </SettingRow>
      <SettingRow
        :title="$gettext('cgroup Root')"
        :description="$gettext('Limits are enforced on Linux with cgroup v2 only.')"
        path="plugin.cgroup_root"
        config-file="plugin"
        :value="data.plugin.cgroup_root"
      />
    </SettingPanel>
  </div>
</template>

<style lang="less" scoped>
.plugin-source-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  text-align: right;
  overflow-wrap: anywhere;
  color: var(--ant-color-text-secondary);
}

@media (max-width: 512px) {
  .plugin-source-list {
    text-align: left;
  }
}
</style>
