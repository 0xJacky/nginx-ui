<script setup lang="ts">
import type { MenuProps } from 'antdv-next'
import type { PluginDrawerTab } from './presets'
import type { PluginInfo } from '@/api/plugin'
import {
  CloudSyncOutlined,
  DeleteOutlined,
  FileTextOutlined,
  GlobalOutlined,
  InfoCircleOutlined,
  MoreOutlined,
  SettingOutlined,
  WarningOutlined,
} from '@antdv-next/icons'
import { PluginIcon } from '@nginxui/plugin-market-ui'
import { localizedPluginDescription, localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import { capabilityIcon, capabilityLabel } from './capabilities'
import { channelHint, channelLabel, pluginChannel } from './channel'
import { conflictNote } from './conflicts'
import { formatMemory, isBelowRecommended, memoryWarning, useSystemMemory } from './memory'
import { isToggleDisabled, needsAttention, statusOf } from './presets'
import SyncPolicyEditor from './SyncPolicyEditor.vue'
import TrustTag from './TrustTag.vue'
import { useConflictNames } from './useConflicts'

const props = defineProps<{
  plugin: PluginInfo
  hasNodes: boolean
  toggling?: boolean
}>()

const emit = defineEmits<{
  open: [tab: PluginDrawerTab]
  toggle: [checked: boolean]
  sync: []
  uninstall: []
  updated: []
}>()

const name = computed(() => localizedPluginName(props.plugin, gettext.current))
const description = computed(() => localizedPluginDescription(props.plugin, gettext.current))
const status = computed(() => statusOf(props.plugin))
const attention = computed(() => needsAttention(props.plugin))
const systemMb = useSystemMemory()
const lowMemory = computed(() => isBelowRecommended(props.plugin.recommended_memory_mb, systemMb.value))
const memoryHint = computed(() => lowMemory.value
  ? memoryWarning(props.plugin.recommended_memory_mb!, systemMb.value)
  : $gettext('Recommended memory: %{size}', { size: formatMemory(props.plugin.recommended_memory_mb ?? 0) }))
const conflictingNames = useConflictNames(() => props.plugin)
const channel = computed(() => pluginChannel(props.plugin))
const toggleDisabled = computed(() => isToggleDisabled(props.plugin))
const isMuted = computed(() => !props.plugin.enabled && !attention.value)

const toggleHint = computed(() => {
  if (toggleDisabled.value)
    return $gettext('This plugin cannot run on this node')
  return props.plugin.enabled ? $gettext('Disable') : $gettext('Enable')
})

const menuItems = computed<MenuProps['items']>(() => {
  const items: NonNullable<MenuProps['items']> = []

  if (props.hasNodes)
    items.push({ key: 'sync', label: $gettext('Sync to nodes'), icon: h(CloudSyncOutlined) })
  if (props.plugin.homepage_url)
    items.push({ key: 'homepage', label: $gettext('Homepage'), icon: h(GlobalOutlined) })

  // The destructive entry sits alone below a divider, unless it is the only one.
  if (items.length > 0)
    items.push({ type: 'divider' })
  items.push({ key: 'uninstall', label: $gettext('Uninstall'), icon: h(DeleteOutlined), danger: true })

  return items
})

function onMenuClick({ key }: { key: string | number }) {
  switch (key) {
    case 'sync':
      emit('sync')
      break
    case 'homepage':
      window.open(props.plugin.homepage_url, '_blank', 'noopener,noreferrer')
      break
    case 'uninstall':
      emit('uninstall')
      break
  }
}
</script>

<template>
  <article
    class="plugin-card is-clickable"
    :class="{ 'is-attention': attention, 'is-error': attention && status.badge === 'error', 'is-muted': isMuted }"
    role="button"
    tabindex="0"
    @click="emit('open', 'overview')"
    @keydown.enter.self="emit('open', 'overview')"
  >
    <div class="plugin-card-head">
      <PluginIcon :src="plugin.icon_url" :name="name" :size="40" :muted="isMuted" />
      <div class="plugin-card-body">
        <ATooltip :title="plugin.id" placement="topLeft">
          <span class="plugin-card-name">{{ name }}</span>
        </ATooltip>
        <div class="plugin-card-sub">
          <span class="plugin-card-version">v{{ plugin.version }}</span>
          <ATooltip v-if="channel !== 'stable'" :title="channelHint(channel)">
            <span class="plugin-card-channel" :class="`is-${channel}`">{{ channelLabel(channel) }}</span>
          </ATooltip>
        </div>
      </div>
      <span class="flex-none" @click.stop>
        <ATooltip :title="toggleHint">
          <ASwitch
            :checked="plugin.enabled"
            :disabled="toggleDisabled"
            :loading="toggling"
            @change="checked => emit('toggle', Boolean(checked))"
          />
        </ATooltip>
      </span>
    </div>

    <p class="plugin-card-description">
      {{ description || $gettext('No description provided.') }}
    </p>

    <div class="plugin-card-facts">
      <ATooltip :title="status.hint?.()">
        <span class="plugin-card-status" :class="`is-${status.badge}`">
          <ABadge :status="status.badge" />
          {{ status.label() }}
        </span>
      </ATooltip>
      <TrustTag :plugin="plugin" plain />
      <span
        v-for="capability in plugin.capabilities"
        :key="capability"
        class="plugin-card-fact"
      >
        <span :class="capabilityIcon(capability)" />
        {{ capabilityLabel(capability) }}
      </span>
      <ATooltip v-if="plugin.recommended_memory_mb" :title="memoryHint">
        <span class="plugin-card-fact" :class="{ 'is-warning': lowMemory }" :aria-label="memoryHint">
          <span class="i-tabler-cpu" />
          {{ formatMemory(plugin.recommended_memory_mb) }}
        </span>
      </ATooltip>
    </div>

    <div v-if="conflictingNames.length > 0" class="plugin-card-note">
      <InfoCircleOutlined class="plugin-card-note-icon flex-none" />
      <span class="min-w-0 break-words">{{ conflictNote(conflictingNames) }}</span>
    </div>

    <div
      v-if="plugin.last_error"
      class="plugin-card-error"
      :title="plugin.last_error"
    >
      <WarningOutlined class="flex-none" />
      <span class="truncate">{{ plugin.last_error }}</span>
    </div>

    <div class="plugin-card-foot" @click.stop>
      <SyncPolicyEditor v-if="hasNodes" :plugin="plugin" compact @updated="emit('updated')" />
      <span v-else />

      <div class="plugin-card-actions">
        <ATooltip :title="$gettext('Settings')">
          <AButton type="text" size="small" :aria-label="$gettext('Settings')" @click="emit('open', 'settings')">
            <template #icon>
              <SettingOutlined />
            </template>
          </AButton>
        </ATooltip>
        <ATooltip :title="$gettext('Logs')">
          <AButton type="text" size="small" :aria-label="$gettext('Logs')" @click="emit('open', 'logs')">
            <template #icon>
              <FileTextOutlined />
            </template>
          </AButton>
        </ATooltip>
        <ADropdown
          :trigger="['click']"
          placement="bottomRight"
          :menu="{ items: menuItems, onClick: onMenuClick }"
        >
          <AButton type="text" size="small" :aria-label="$gettext('More actions')">
            <template #icon>
              <MoreOutlined />
            </template>
          </AButton>
        </ADropdown>
      </div>
    </div>
  </article>
</template>

<style lang="less" scoped>
@import './plugin-card.less';

.plugin-card-error {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 6px 10px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--ant-color-error);
  background: var(--ant-color-error-bg);
  border-radius: var(--ant-border-radius);
}

.plugin-card-note {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  min-width: 0;
  padding: 6px 10px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--ant-color-text-secondary);
  background: var(--ant-color-fill-quaternary);
  border-radius: var(--ant-border-radius);
}

// Keeps the icon on the first line when the names wrap.
.plugin-card-note-icon {
  margin-top: 3px;
}

.plugin-card-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-left: auto;
}
</style>
