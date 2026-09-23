<script setup lang="ts">
import type { PluginDrawerTab } from './presets'
import type { PluginInfo } from '@/api/plugin'
import { useWindowSize } from '@vueuse/core'
import { localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import LogsPanel from './LogsPanel.vue'
import PluginIcon from './PluginIcon.vue'
import PluginOverview from './PluginOverview.vue'
import { isToggleDisabled, statusOf } from './presets'
import SettingsPanel from './SettingsPanel.vue'

const props = defineProps<{
  plugin?: PluginInfo
  hasNodes: boolean
  toggling?: boolean
}>()

const emit = defineEmits<{
  toggle: [checked: boolean]
  sync: []
  uninstall: []
  updated: []
}>()

const open = defineModel<boolean>('open', { default: false })
const tab = defineModel<PluginDrawerTab>('tab', { default: 'overview' })

const { width: windowWidth } = useWindowSize()
// A phone gets the whole screen instead of an unusable sliver.
const drawerSize = computed(() => Math.min(760, windowWidth.value))

const name = computed(() => (props.plugin ? localizedPluginName(props.plugin, gettext.current) : ''))
const status = computed(() => (props.plugin ? statusOf(props.plugin) : undefined))
const toggleDisabled = computed(() => (props.plugin ? isToggleDisabled(props.plugin) : true))

const tabs = computed(() => [
  { key: 'overview', label: $gettext('Overview') },
  { key: 'settings', label: $gettext('Settings') },
  { key: 'logs', label: $gettext('Logs') },
])

const isActive = (key: PluginDrawerTab) => open.value && tab.value === key
</script>

<template>
  <ADrawer
    v-model:open="open"
    :size="drawerSize"
    placement="right"
    class="plugin-drawer"
  >
    <template #title>
      <div v-if="props.plugin" class="drawer-title">
        <PluginIcon :src="props.plugin.icon_url" :size="36" />
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <span class="truncate font-semibold">{{ name }}</span>
            <span class="drawer-version">v{{ props.plugin.version }}</span>
          </div>
          <div class="drawer-id truncate">
            {{ props.plugin.id }}
          </div>
        </div>
      </div>
    </template>

    <template #extra>
      <div v-if="props.plugin && status" class="flex items-center gap-3">
        <ATooltip :title="props.plugin.last_error || undefined">
          <ABadge :status="status.badge" :text="status.label()" />
        </ATooltip>
        <ASwitch
          :checked="props.plugin.enabled"
          :disabled="toggleDisabled"
          :loading="props.toggling"
          @change="checked => emit('toggle', Boolean(checked))"
        />
      </div>
    </template>

    <ATabs v-model:active-key="tab" :items="tabs">
      <template #contentRender="{ item }">
        <PluginOverview
          v-if="item.key === 'overview' && props.plugin"
          :plugin="props.plugin"
          :has-nodes="props.hasNodes"
          @sync="emit('sync')"
          @uninstall="emit('uninstall')"
          @updated="emit('updated')"
        />

        <SettingsPanel
          v-else-if="item.key === 'settings'"
          :plugin="props.plugin"
          :active="isActive('settings')"
        />
        <LogsPanel
          v-else-if="item.key === 'logs'"
          :plugin="props.plugin"
          :active="isActive('logs')"
        />
      </template>
    </ATabs>
  </ADrawer>
</template>

<style lang="less" scoped>
.drawer-title {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.drawer-version {
  flex: none;
  padding: 0 7px;
  font-size: 12px;
  font-weight: 400;
  line-height: 18px;
  color: var(--ant-color-text-secondary);
  background: var(--ant-color-fill-tertiary);
  border-radius: 999px;
}

.drawer-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  font-weight: 400;
  color: var(--ant-color-text-quaternary);
}
</style>
