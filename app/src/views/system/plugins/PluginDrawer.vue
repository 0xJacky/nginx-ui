<script setup lang="ts">
import type { PluginDrawerTab } from './presets'
import type { PluginInfo } from '@/api/plugin'
import { CloudSyncOutlined, DeleteOutlined, LinkOutlined } from '@antdv-next/icons'
import { useWindowSize } from '@vueuse/core'
import { formatDateTime } from '@/lib/helper'
import LogsPanel from './LogsPanel.vue'
import PermissionList from './PermissionList.vue'
import PluginIcon from './PluginIcon.vue'
import { isToggleDisabled, statusOf } from './presets'
import SettingsPanel from './SettingsPanel.vue'
import SyncPolicyEditor from './SyncPolicyEditor.vue'

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

const status = computed(() => (props.plugin ? statusOf(props.plugin) : undefined))
const toggleDisabled = computed(() => (props.plugin ? isToggleDisabled(props.plugin) : true))

const tabs = computed(() => [
  { key: 'overview', label: $gettext('Overview') },
  { key: 'settings', label: $gettext('Settings') },
  { key: 'logs', label: $gettext('Logs') },
])

const lifecycleLabel = computed(() => {
  if (!props.plugin?.has_server)
    return $gettext('No background process')
  return props.plugin.lifecycle === 'on_demand' ? $gettext('On demand') : $gettext('Resident')
})

const transportLabel = computed(() => props.plugin?.transport === 'grpc' ? 'gRPC' : 'stdio')

const hasLogSink = computed(() => props.plugin?.capabilities?.includes('log.sink') ?? false)

const logSinkCounters = computed(() => [
  { key: 'streamed', label: $gettext('Streamed'), value: props.plugin?.streamed_log_entries ?? 0 },
  { key: 'rejected', label: $gettext('Rejected'), value: props.plugin?.rejected_log_entries ?? 0 },
  { key: 'dropped', label: $gettext('Dropped'), value: props.plugin?.dropped_log_entries ?? 0 },
])

const resourceLimits = computed(() => {
  const resources = props.plugin?.resources
  if (!resources)
    return undefined
  return [
    {
      key: 'memory',
      label: $gettext('Memory'),
      value: resources.memory_limit_mb > 0 ? `${resources.memory_limit_mb} MB` : $gettext('Unlimited'),
    },
    {
      key: 'cpu',
      label: $gettext('CPU'),
      value: resources.cpu_percent > 0 ? `${resources.cpu_percent}%` : $gettext('Unlimited'),
    },
  ]
})

const hasResourceLimits = computed(() => {
  const resources = props.plugin?.resources
  return !!resources && (resources.memory_limit_mb > 0 || resources.cpu_percent > 0)
})

function formatCount(value: number) {
  return value.toLocaleString()
}

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
            <span class="truncate font-semibold">{{ props.plugin.name }}</span>
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
        <div v-if="item.key === 'overview' && props.plugin" class="drawer-overview">
          <AAlert
            v-if="props.plugin.last_error"
            type="error"
            show-icon
            :title="$gettext('Last error')"
            :description="props.plugin.last_error"
          />

          <AAlert
            v-else-if="props.plugin.status === 'needs_approval'"
            type="warning"
            show-icon
            :title="$gettext('This plugin needs your approval before it can run.')"
            :description="$gettext('Turn it on to review the requested permissions.')"
          />

          <p v-if="props.plugin.description" class="drawer-description">
            {{ props.plugin.description }}
          </p>

          <ADescriptions
            :column="1"
            size="small"
            bordered
          >
            <ADescriptionsItem :label="$gettext('Version')">
              {{ props.plugin.version }}
            </ADescriptionsItem>
            <ADescriptionsItem :label="$gettext('API version')">
              {{ props.plugin.api_version }}
              <span v-if="props.plugin.min_nginx_ui_version" class="ml-2 text-gray-500">
                {{ $gettext('Needs Nginx UI %{version} or newer', { version: props.plugin.min_nginx_ui_version }) }}
              </span>
            </ADescriptionsItem>
            <ADescriptionsItem :label="$gettext('Process')">
              {{ lifecycleLabel }}
            </ADescriptionsItem>
            <ADescriptionsItem v-if="props.plugin.transport" :label="$gettext('Transport')">
              {{ transportLabel }}
            </ADescriptionsItem>
            <ADescriptionsItem v-if="resourceLimits" :label="$gettext('Resource limits')">
              <div class="drawer-stats">
                <span v-for="limit in resourceLimits" :key="limit.key">
                  <span class="drawer-stat-label">{{ limit.label }}</span>
                  {{ limit.value }}
                </span>
                <ATooltip
                  v-if="hasResourceLimits"
                  :title="props.plugin.resources?.enforced
                    ? $gettext('The running process is confined to these limits.')
                    : $gettext('The plugin is not running, or this node cannot confine processes: that needs Linux with a writable cgroup v2 hierarchy.')"
                >
                  <ATag class="m-0" :color="props.plugin.resources?.enforced ? 'success' : 'default'">
                    {{ props.plugin.resources?.enforced ? $gettext('Enforced') : $gettext('Not enforced') }}
                  </ATag>
                </ATooltip>
              </div>
            </ADescriptionsItem>
            <ADescriptionsItem v-if="hasLogSink" :label="$gettext('Access log stream')">
              <div class="drawer-stats">
                <span v-for="counter in logSinkCounters" :key="counter.key">
                  <span class="drawer-stat-label">{{ counter.label }}</span>
                  <span :class="{ 'drawer-stat-warning': counter.key === 'dropped' && counter.value > 0 }">
                    {{ formatCount(counter.value) }}
                  </span>
                </span>
              </div>
            </ADescriptionsItem>
            <ADescriptionsItem :label="$gettext('Capabilities')">
              <div v-if="props.plugin.capabilities?.length" class="flex flex-wrap gap-1">
                <ATag
                  v-for="capability in props.plugin.capabilities"
                  :key="capability"
                  class="m-0"
                  variant="filled"
                >
                  {{ capability }}
                </ATag>
              </div>
              <span v-else class="text-gray-400">{{ $gettext('None') }}</span>
            </ADescriptionsItem>
            <ADescriptionsItem v-if="props.plugin.requires?.length" :label="$gettext('Depends on')">
              <div class="flex flex-wrap gap-1">
                <ATag
                  v-for="requirement in props.plugin.requires"
                  :key="requirement.id"
                  class="m-0 font-mono text-xs"
                >
                  {{ requirement.id }}<template v-if="requirement.version">
                    {{ requirement.version }}
                  </template>
                </ATag>
              </div>
            </ADescriptionsItem>
            <ADescriptionsItem v-if="props.plugin.requires_capabilities?.length" :label="$gettext('Needs capabilities')">
              <div class="flex flex-wrap gap-1">
                <ATag
                  v-for="capability in props.plugin.requires_capabilities"
                  :key="capability"
                  class="m-0"
                >
                  {{ capability }}
                </ATag>
              </div>
            </ADescriptionsItem>
            <ADescriptionsItem v-if="props.plugin.homepage_url" :label="$gettext('Homepage')">
              <a
                :href="props.plugin.homepage_url"
                target="_blank"
                rel="noopener noreferrer"
                class="break-all"
              >
                <LinkOutlined class="mr-1" />{{ props.plugin.homepage_url }}
              </a>
            </ADescriptionsItem>
            <ADescriptionsItem v-if="props.plugin.updated_at" :label="$gettext('Updated at')">
              {{ formatDateTime(props.plugin.updated_at) }}
            </ADescriptionsItem>
          </ADescriptions>

          <section>
            <h4 class="drawer-section-title">
              {{ $gettext('Permissions') }}
            </h4>
            <PermissionList :permissions="props.plugin.permissions ?? []" />
          </section>

          <section v-if="props.hasNodes">
            <h4 class="drawer-section-title">
              {{ $gettext('Cluster') }}
            </h4>
            <div class="drawer-cluster">
              <div class="min-w-0">
                <div class="mb-1 font-medium">
                  {{ $gettext('Auto install to nodes') }}
                </div>
                <SyncPolicyEditor :plugin="props.plugin" @updated="emit('updated')" />
              </div>
              <AButton @click="emit('sync')">
                <template #icon>
                  <CloudSyncOutlined />
                </template>
                {{ $gettext('Sync now') }}
              </AButton>
            </div>
          </section>

          <section>
            <h4 class="drawer-section-title">
              {{ $gettext('Danger zone') }}
            </h4>
            <div class="drawer-danger">
              <div class="min-w-0">
                <div class="mb-1 font-medium">
                  {{ $gettext('Uninstall this plugin') }}
                </div>
                <div class="text-gray-500">
                  {{ $gettext('Its files, data and settings are removed from this node.') }}
                </div>
              </div>
              <AButton danger @click="emit('uninstall')">
                <template #icon>
                  <DeleteOutlined />
                </template>
                {{ $gettext('Uninstall') }}
              </AButton>
            </div>
          </section>
        </div>

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

.drawer-overview {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.drawer-description {
  margin: 0;
  line-height: 1.7;
  color: var(--ant-color-text-secondary);
}

.drawer-section-title {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  color: var(--ant-color-text-tertiary);
}

.drawer-cluster,
.drawer-danger {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius-lg);
}

.drawer-danger {
  border-color: var(--ant-color-error-border);
}

.drawer-stats {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 16px;
}

.drawer-stat-label {
  margin-right: 6px;
  color: var(--ant-color-text-tertiary);
}

.drawer-stat-warning {
  color: var(--ant-color-warning-text);
}
</style>
