<script setup lang="ts">
import type { PluginInfo } from '@/api/plugin'
import { CloudSyncOutlined, DeleteOutlined, LinkOutlined } from '@antdv-next/icons'
import { localizedPluginDescription } from '@/api/plugin'
import gettext from '@/gettext'
import { formatDateTime } from '@/lib/helper'
import PermissionList from './PermissionList.vue'
import SyncPolicyEditor from './SyncPolicyEditor.vue'

const props = defineProps<{
  plugin: PluginInfo
  hasNodes: boolean
}>()

const emit = defineEmits<{
  sync: []
  uninstall: []
  updated: []
}>()

const description = computed(() => localizedPluginDescription(props.plugin, gettext.current))

const processLabel = computed(() => {
  if (!props.plugin.has_server)
    return $gettext('No background process')
  return props.plugin.lifecycle === 'on_demand' ? $gettext('On demand') : $gettext('Resident')
})

interface Fact {
  key: string
  label: string
  value: string
  hint?: string
}

// The four numbers a person looks for first, laid out as tiles instead of a
// key value table.
const facts = computed<Fact[]>(() => {
  const plugin = props.plugin
  const items: Fact[] = [
    { key: 'version', label: $gettext('Version'), value: `v${plugin.version}` },
    {
      key: 'api',
      label: $gettext('API version'),
      value: String(plugin.api_version),
      hint: plugin.min_nginx_ui_version
        ? $gettext('Needs Nginx UI %{version} or newer', { version: plugin.min_nginx_ui_version })
        : undefined,
    },
    {
      key: 'process',
      label: $gettext('Process'),
      value: processLabel.value,
      hint: plugin.transport ? (plugin.transport === 'grpc' ? 'gRPC' : 'stdio') : undefined,
    },
  ]
  if (plugin.updated_at)
    items.push({ key: 'updated', label: $gettext('Updated at'), value: formatDateTime(plugin.updated_at) })
  return items
})

const resources = computed(() => props.plugin.resources)
const hasResourceLimits = computed(() => {
  const limits = resources.value
  return !!limits && (limits.memory_limit_mb > 0 || limits.cpu_percent > 0)
})

const resourceCells = computed(() => {
  const limits = resources.value
  if (!limits)
    return []
  return [
    {
      key: 'memory',
      label: $gettext('Memory'),
      value: limits.memory_limit_mb > 0 ? `${limits.memory_limit_mb} MB` : $gettext('Unlimited'),
    },
    {
      key: 'cpu',
      label: $gettext('CPU'),
      value: limits.cpu_percent > 0 ? `${limits.cpu_percent}%` : $gettext('Unlimited'),
    },
  ]
})

const hasLogSink = computed(() => props.plugin.capabilities?.includes('log.sink') ?? false)

const logSinkCells = computed(() => [
  { key: 'streamed', label: $gettext('Streamed'), value: props.plugin.streamed_log_entries ?? 0, warn: false },
  { key: 'rejected', label: $gettext('Rejected'), value: props.plugin.rejected_log_entries ?? 0, warn: false },
  { key: 'dropped', label: $gettext('Dropped'), value: props.plugin.dropped_log_entries ?? 0, warn: (props.plugin.dropped_log_entries ?? 0) > 0 },
])

function formatCount(value: number) {
  return value.toLocaleString()
}
</script>

<template>
  <div class="overview">
    <AAlert
      v-if="plugin.last_error"
      type="error"
      show-icon
      :title="$gettext('Last error')"
      :description="plugin.last_error"
    />
    <AAlert
      v-else-if="plugin.status === 'needs_approval'"
      type="warning"
      show-icon
      :title="$gettext('This plugin needs your approval before it can run.')"
      :description="$gettext('Turn it on to review the requested permissions.')"
    />

    <p v-if="description" class="overview-description">
      {{ description }}
    </p>

    <div class="fact-grid">
      <div
        v-for="fact in facts"
        :key="fact.key"
        class="fact"
      >
        <span class="fact-label">{{ fact.label }}</span>
        <span class="fact-value">{{ fact.value }}</span>
        <span v-if="fact.hint" class="fact-hint">{{ fact.hint }}</span>
      </div>
    </div>

    <section v-if="hasResourceLimits" class="overview-section">
      <div class="section-head">
        <h4 class="section-title">
          {{ $gettext('Resource limits') }}
        </h4>
        <ATooltip
          v-if="resources"
          :title="resources.enforced
            ? $gettext('The running process is confined to these limits.')
            : $gettext('The plugin is not running, or this node cannot confine processes: that needs Linux with a writable cgroup v2 hierarchy.')"
        >
          <span class="pill" :class="resources.enforced ? 'is-success' : 'is-muted'">
            <span class="pill-dot" />
            {{ resources.enforced ? $gettext('Enforced') : $gettext('Not enforced') }}
          </span>
        </ATooltip>
      </div>
      <div class="metric-row">
        <div
          v-for="cell in resourceCells"
          :key="cell.key"
          class="metric"
        >
          <span class="metric-value">{{ cell.value }}</span>
          <span class="metric-label">{{ cell.label }}</span>
        </div>
      </div>
    </section>

    <section v-if="hasLogSink" class="overview-section">
      <div class="section-head">
        <h4 class="section-title">
          {{ $gettext('Access log stream') }}
        </h4>
      </div>
      <div class="metric-row">
        <div
          v-for="cell in logSinkCells"
          :key="cell.key"
          class="metric"
        >
          <span class="metric-value" :class="{ 'is-warning': cell.warn }">{{ formatCount(cell.value) }}</span>
          <span class="metric-label">{{ cell.label }}</span>
        </div>
      </div>
    </section>

    <section class="overview-section">
      <div class="section-head">
        <h4 class="section-title">
          {{ $gettext('Capabilities') }}
        </h4>
      </div>
      <div v-if="plugin.capabilities?.length" class="pill-row">
        <span
          v-for="capability in plugin.capabilities"
          :key="capability"
          class="pill is-accent"
        >
          {{ capability }}
        </span>
      </div>
      <p v-else class="section-empty">
        {{ $gettext('None') }}
      </p>

      <dl class="detail-list">
        <div v-if="plugin.requires?.length" class="detail-row">
          <dt>{{ $gettext('Depends on') }}</dt>
          <dd class="pill-row">
            <span
              v-for="requirement in plugin.requires"
              :key="requirement.id"
              class="pill is-mono"
            >
              {{ requirement.id }}<template v-if="requirement.version"> {{ requirement.version }}</template>
            </span>
          </dd>
        </div>
        <div v-if="plugin.requires_capabilities?.length" class="detail-row">
          <dt>{{ $gettext('Needs capabilities') }}</dt>
          <dd class="pill-row">
            <span
              v-for="capability in plugin.requires_capabilities"
              :key="capability"
              class="pill"
            >
              {{ capability }}
            </span>
          </dd>
        </div>
        <div v-if="plugin.homepage_url" class="detail-row">
          <dt>{{ $gettext('Homepage') }}</dt>
          <dd>
            <a
              :href="plugin.homepage_url"
              target="_blank"
              rel="noopener noreferrer"
              class="detail-link"
            >
              <LinkOutlined />
              <span class="truncate">{{ plugin.homepage_url }}</span>
            </a>
          </dd>
        </div>
      </dl>
    </section>

    <section class="overview-section">
      <div class="section-head">
        <h4 class="section-title">
          {{ $gettext('Permissions') }}
        </h4>
        <span v-if="plugin.permissions?.length" class="section-count">{{ plugin.permissions.length }}</span>
      </div>
      <PermissionList :permissions="plugin.permissions ?? []" />
    </section>

    <section v-if="hasNodes" class="overview-section">
      <div class="section-head">
        <h4 class="section-title">
          {{ $gettext('Cluster') }}
        </h4>
      </div>
      <div class="panel">
        <div class="min-w-0">
          <div class="panel-title">
            {{ $gettext('Auto install to nodes') }}
          </div>
          <SyncPolicyEditor :plugin="plugin" @updated="emit('updated')" />
        </div>
        <AButton @click="emit('sync')">
          <template #icon>
            <CloudSyncOutlined />
          </template>
          {{ $gettext('Sync now') }}
        </AButton>
      </div>
    </section>

    <section class="overview-section">
      <div class="panel is-danger">
        <div class="min-w-0">
          <div class="panel-title">
            {{ $gettext('Uninstall this plugin') }}
          </div>
          <div class="panel-text">
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
</template>

<style lang="less" scoped>
@import './plugin-detail.less';
</style>
