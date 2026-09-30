<script setup lang="ts">
import type { PluginChannel, PluginInfo, PluginUsage } from '@/api/plugin'
import { CloudSyncOutlined, DeleteOutlined, DownOutlined, LinkOutlined, RightOutlined } from '@antdv-next/icons'
import pluginApi, { localizedPluginDescription } from '@/api/plugin'
import gettext from '@/gettext'
import { formatDateTime } from '@/lib/helper'
import { getErrorMessage } from '@/lib/http'
import { capabilityLabel, capabilityPreset } from './capabilities'
import { channelDescription, channelLabel, effectiveChannel, followedChannel, heldByReleaseText, isHeldByRelease, PLUGIN_CHANNELS, pluginChannel } from './channel'
import ChannelTag from './ChannelTag.vue'
import { conflictNote } from './conflicts'
import { packageTrustPreset, trustedOfferAction, trustedOfferSummary, unsignedExplanation } from './marketplace/trust'
import { formatMemory, isBelowRecommended, memoryWarning, useSystemMemory } from './memory'
import { describePermission, permissionLabel } from './permissions'
import { useReplacePlugin, useTrustedOffer } from './replace'
import SyncPolicyEditor from './SyncPolicyEditor.vue'
import { formatUsagePreview, previewUsage } from './usage'
import { useConflictNames } from './useConflicts'

const props = defineProps<{
  plugin: PluginInfo
  hasNodes: boolean
}>()

const emit = defineEmits<{
  sync: []
  uninstall: []
  updated: []
}>()

const { message } = App.useApp()

/** Addresses shown before the list is expanded. */
const collapsedHostCount = 3
/** Names of dependent items listed in the summary line. */
const usageNameCount = 3

const description = computed(() => localizedPluginDescription(props.plugin, gettext.current))

// Trust notice
const trust = computed(() => packageTrustPreset(props.plugin.trust))
const isUnsigned = computed(() => props.plugin.trust === 'unsigned')
const offer = useTrustedOffer(() => props.plugin)
const { replacingId, confirmReplace } = useReplacePlugin()
const showTrustNotice = computed(() => Boolean(trust.value) && (isUnsigned.value || Boolean(offer.value)))
const trustNotice = computed(() => {
  const text = isUnsigned.value ? unsignedExplanation() : trust.value?.hint() ?? ''
  return offer.value ? `${text} ${trustedOfferSummary(offer.value)}` : text
})

// Provides
const provides = computed(() => (props.plugin.capabilities ?? []).map(capability => ({
  key: capability,
  preset: capabilityPreset(capability),
})))

// Can access
const hostsExpanded = ref(false)
const networkHosts = computed(() => props.plugin.network_hosts ?? [])
const visibleHosts = computed(() => (hostsExpanded.value
  ? networkHosts.value
  : networkHosts.value.slice(0, collapsedHostCount)))
const hiddenHostCount = computed(() => networkHosts.value.length - visibleHosts.value.length)

watch(() => props.plugin.id, () => {
  hostsExpanded.value = false
})

// In use
const usage = ref<PluginUsage>()

async function loadUsage() {
  const id = props.plugin.id
  try {
    const result = await pluginApi.getUsage(id)
    // Drop a late answer for a plugin that is no longer shown.
    if (props.plugin.id === id)
      usage.value = result
  }
  catch {
    usage.value = undefined
  }
}

watch(() => [props.plugin.id, props.plugin.enabled, props.plugin.updated_at], () => {
  void loadUsage()
}, { immediate: true })

const certificateUsage = computed(() => {
  if (!usage.value || usage.value.total === 0)
    return undefined
  return {
    total: usage.value.total,
    names: formatUsagePreview(previewUsage(usage.value, usageNameCount)),
  }
})

// Technical details
const startMode = computed(() => {
  if (!props.plugin.has_server)
    return $gettext('Nothing to start')
  return props.plugin.lifecycle === 'on_demand' ? $gettext('Starts when needed') : $gettext('Always running')
})

interface Fact {
  key: string
  label: string
  value: string
  hint?: string
  channel?: PluginChannel
}

const facts = computed<Fact[]>(() => {
  const plugin = props.plugin
  const items: Fact[] = [
    { key: 'version', label: $gettext('Version'), value: `v${plugin.version}`, channel: pluginChannel(plugin) },
    {
      key: 'api',
      label: $gettext('API version'),
      value: String(plugin.api_version),
      hint: plugin.min_nginx_ui_version
        ? $gettext('Needs Nginx UI %{version} or newer', { version: plugin.min_nginx_ui_version })
        : undefined,
    },
    { key: 'start', label: $gettext('Start mode'), value: startMode.value },
  ]
  if (plugin.recommended_memory_mb)
    items.push({ key: 'memory', label: $gettext('Recommended memory'), value: formatMemory(plugin.recommended_memory_mb) })
  if (plugin.updated_at)
    items.push({ key: 'updated', label: $gettext('Updated at'), value: formatDateTime(plugin.updated_at) })
  return items
})

// Versions to receive
const followed = computed(() => followedChannel(props.plugin))
const savingChannel = ref(false)
const channelOptions = computed(() => PLUGIN_CHANNELS.map(value => ({ value, label: channelLabel(value) })))
// A test version that was installed keeps test versions coming until a stable one is out.
const heldByRelease = computed(() => isHeldByRelease(props.plugin))

async function changeChannel(value: PluginChannel) {
  if (value === followed.value)
    return
  savingChannel.value = true
  try {
    await pluginApi.setChannel(props.plugin.id, value)
    message.success($gettext('Saved'))
    emit('updated')
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to save')))
  }
  finally {
    savingChannel.value = false
  }
}

const systemMb = useSystemMemory()
const conflictingNames = useConflictNames(() => props.plugin)
const lowMemory = computed(() => isBelowRecommended(props.plugin.recommended_memory_mb, systemMb.value))

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

const technicalItems = [{ key: 'technical' }]

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
      :description="$gettext('Turn it on to review what it can access.')"
    />

    <AAlert
      v-if="lowMemory"
      type="warning"
      show-icon
      :title="memoryWarning(plugin.recommended_memory_mb!, systemMb)"
    />

    <AAlert
      v-if="conflictingNames.length > 0"
      type="info"
      show-icon
      :title="conflictNote(conflictingNames)"
    />

    <div v-if="description || plugin.homepage_url" class="flex flex-col gap-2">
      <p v-if="description" class="overview-description">
        {{ description }}
      </p>
      <a
        v-if="plugin.homepage_url"
        :href="plugin.homepage_url"
        target="_blank"
        rel="noopener noreferrer"
        class="detail-link"
      >
        <LinkOutlined />
        <span class="truncate">{{ $gettext('Homepage') }}</span>
      </a>
    </div>

    <AAlert
      v-if="showTrustNotice && trust"
      type="warning"
      show-icon
    >
      <template #title>
        <span><strong>{{ trust.label() }}:</strong> {{ trustNotice }}</span>
      </template>
      <template v-if="offer" #description>
        <AButton
          type="primary"
          size="small"
          class="mt-1"
          :loading="replacingId === plugin.id"
          @click="confirmReplace(offer)"
        >
          {{ trustedOfferAction(offer) }}
        </AButton>
      </template>
    </AAlert>

    <section v-if="provides.length" class="overview-section">
      <h4 class="section-title">
        {{ $gettext('Provides') }}
      </h4>
      <div class="flex flex-col gap-2">
        <div v-for="item in provides" :key="item.key" class="panel">
          <div class="min-w-0 flex-1">
            <div class="panel-title">
              {{ item.preset.label() }}
            </div>
            <div class="panel-text">
              {{ item.preset.description() }}
            </div>
          </div>
          <RouterLink v-if="item.preset.link" :to="item.preset.link.to" class="panel-link">
            {{ item.preset.link.label() }}
            <RightOutlined />
          </RouterLink>
        </div>
      </div>
    </section>

    <section class="overview-section">
      <h4 class="section-title">
        {{ $gettext('Can access') }}
      </h4>
      <div v-if="plugin.permissions?.length" class="flex flex-col gap-2">
        <div v-for="permission in plugin.permissions" :key="permission" class="panel is-stacked">
          <div class="panel-title">
            {{ permissionLabel(permission) }}
          </div>
          <div class="panel-text">
            {{ describePermission(permission) }}
          </div>
          <template v-if="permission === 'network'">
            <template v-if="networkHosts.length">
              <div class="panel-text mt-1">
                {{ $gettext('Only these addresses') }}
              </div>
              <div class="pill-row mt-1">
                <ATag
                  v-for="host in visibleHosts"
                  :key="host"
                  class="host-tag m-0"
                >
                  {{ host }}
                </ATag>
                <AButton
                  v-if="hiddenHostCount > 0"
                  size="small"
                  @click="hostsExpanded = true"
                >
                  {{ $gettext('%{n} more', { n: String(hiddenHostCount) }) }}
                  <DownOutlined />
                </AButton>
              </div>
            </template>
            <div v-else class="panel-text mt-1">
              {{ $gettext('No specific addresses listed') }}
            </div>
          </template>
        </div>
      </div>
      <p v-else class="section-empty">
        {{ $gettext('Nothing beyond its own features.') }}
      </p>
    </section>

    <section v-if="certificateUsage" class="overview-section">
      <h4 class="section-title">
        {{ $gettext('In use') }}
      </h4>
      <p class="usage-line">
        {{ $ngettext(
          '%{count} certificate renews through it:',
          '%{count} certificates renew through it:',
          certificateUsage.total,
          { count: String(certificateUsage.total) },
        ) }}
        {{ certificateUsage.names }}
      </p>
    </section>

    <section class="overview-section">
      <h4 class="section-title">
        {{ $gettext('Updates') }}
      </h4>
      <div class="panel">
        <div class="min-w-0">
          <div class="panel-title">
            {{ $gettext('Update channel') }}
          </div>
          <div class="panel-text">
            {{ channelDescription(followed) }}
          </div>
          <div v-if="heldByRelease" class="panel-text mt-1">
            {{ heldByReleaseText(effectiveChannel(plugin)) }}
          </div>
        </div>
        <ASelect
          class="channel-select"
          :value="followed"
          :options="channelOptions"
          :loading="savingChannel"
          :disabled="savingChannel"
          @change="changeChannel"
        />
      </div>
    </section>

    <section v-if="hasNodes" class="overview-section">
      <h4 class="section-title">
        {{ $gettext('Cluster') }}
      </h4>
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

    <ACollapse ghost :items="technicalItems">
      <template #labelRender>
        <span class="technical-label">
          {{ $gettext('Technical details') }}
          <span class="section-hint">{{ $gettext('Version, API version, start mode, updated time') }}</span>
        </span>
      </template>
      <template #contentRender>
        <div class="flex flex-col gap-4">
          <div class="fact-grid">
            <div
              v-for="fact in facts"
              :key="fact.key"
              class="fact"
            >
              <span class="fact-label">{{ fact.label }}</span>
              <span class="fact-value">
                {{ fact.value }}
                <span v-if="fact.channel && fact.channel !== 'stable'" class="ml-2 inline-flex align-middle"><ChannelTag :channel="fact.channel" /></span>
              </span>
              <span v-if="fact.hint" class="fact-hint">{{ fact.hint }}</span>
            </div>
          </div>

          <dl class="detail-list">
            <div class="detail-row">
              <dt>{{ $gettext('ID') }}</dt>
              <dd class="technical-id">
                {{ plugin.id }}
              </dd>
            </div>
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
              <dt>{{ $gettext('Needs features') }}</dt>
              <dd class="pill-row">
                <span
                  v-for="capability in plugin.requires_capabilities"
                  :key="capability"
                  class="pill"
                >
                  {{ capabilityLabel(capability) }}
                </span>
              </dd>
            </div>
          </dl>

          <div v-if="hasResourceLimits" class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ $gettext('Resource limits') }}
              </h4>
              <ATooltip
                v-if="resources"
                :title="resources.enforced
                  ? $gettext('The limits are applied while the plugin runs.')
                  : $gettext('The plugin is not running, or this node cannot apply limits.')"
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
          </div>

          <div v-if="hasLogSink" class="overview-section">
            <h4 class="section-title">
              {{ $gettext('Access log stream') }}
            </h4>
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
          </div>
        </div>
      </template>
    </ACollapse>

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

.panel.is-stacked {
  flex-direction: column;
  align-items: stretch;
  gap: 2px;
}

.channel-select {
  flex: none;
  width: 150px;
}

.panel-link {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 4px;
  font-size: 13px;
}

.host-tag {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}

.usage-line {
  margin: 0;
  line-height: 1.7;
  color: var(--ant-color-text-secondary);
}

.technical-label {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 8px;
  font-weight: 600;
}

.technical-label .section-hint {
  font-weight: 400;
}

.technical-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  overflow-wrap: anywhere;
}
</style>
