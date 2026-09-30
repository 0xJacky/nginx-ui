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
import { localizedPluginDescription, localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import { capabilityLabel } from './capabilities'
import { pluginChannel } from './channel'
import ChannelTag from './ChannelTag.vue'
import { conflictNote } from './conflicts'
import { isBelowRecommended, useSystemMemory } from './memory'
import PluginIcon from './PluginIcon.vue'
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
    :class="{ 'is-attention': attention, 'is-muted': isMuted }"
    role="button"
    tabindex="0"
    @click="emit('open', 'overview')"
    @keydown.enter.self="emit('open', 'overview')"
  >
    <div class="plugin-card-head">
      <PluginIcon :src="plugin.icon_url" :name="name" :size="40" :muted="isMuted" />
      <div class="plugin-card-body">
        <div class="plugin-card-title">
          <span class="plugin-card-name">{{ name }}</span>
          <span class="plugin-card-version">v{{ plugin.version }}</span>
        </div>
        <div class="plugin-card-id">
          {{ plugin.id }}
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

    <div class="plugin-card-meta">
      <ATooltip :title="status.hint?.()">
        <ABadge :status="status.badge" :text="status.label()" />
      </ATooltip>
      <!-- Outlined, so the trust level stands apart from the capability tags. -->
      <TrustTag :plugin="plugin" />
      <ChannelTag :channel="channel" />
      <ATooltip v-if="lowMemory" :title="$gettext('This server has less memory than this plugin is recommended for.')">
        <ATag color="warning" class="m-0">
          {{ $gettext('Low memory') }}
        </ATag>
      </ATooltip>
      <ATag
        v-for="capability in plugin.capabilities"
        :key="capability"
        class="m-0"
        variant="filled"
      >
        {{ capabilityLabel(capability) }}
      </ATag>
    </div>

    <div v-if="conflictingNames.length > 0" class="plugin-card-note" :title="conflictNote(conflictingNames)">
      <InfoCircleOutlined class="flex-none" />
      <span class="truncate">{{ conflictNote(conflictingNames) }}</span>
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
      <div v-if="hasNodes" class="plugin-card-sync">
        <CloudSyncOutlined class="text-gray-400" />
        <SyncPolicyEditor :plugin="plugin" @updated="emit('updated')" />
      </div>
      <span v-else />

      <div class="plugin-card-actions">
        <AButton type="text" size="small" @click="emit('open', 'settings')">
          <template #icon>
            <SettingOutlined />
          </template>
          {{ $gettext('Settings') }}
        </AButton>
        <AButton type="text" size="small" @click="emit('open', 'logs')">
          <template #icon>
            <FileTextOutlined />
          </template>
          {{ $gettext('Logs') }}
        </AButton>
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
  align-items: center;
  gap: 6px;
  min-width: 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--ant-color-text-secondary);
}

.plugin-card-sync {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.plugin-card-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-left: auto;
}
</style>
