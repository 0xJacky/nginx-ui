<script setup lang="ts">
import type { CatalogEntry } from '@/api/plugin_marketplace'
import { ArrowUpOutlined, CheckCircleOutlined, DownloadOutlined, UserOutlined, WarningOutlined } from '@antdv-next/icons'
import { catalogEntryDescription, catalogEntryName } from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { capabilityIcon, capabilityLabel } from '../capabilities'
import { channelHint, channelLabel, entryChannel } from '../channel'
import { useInstalledPlugin } from '../inventory'
import { formatMemory, isBelowRecommended, memoryWarning, recommendedMemory, useSystemMemory } from '../memory'
import PluginIcon from '../PluginIcon.vue'
import { useReplacePlugin } from '../replace'
import { useOfficialElsewhere, useSourceName } from './sources'
import { findTrustedOffer, trustedOfferAction, trustPreset } from './trust'

const props = defineProps<{
  entry: CatalogEntry
  installing?: boolean
}>()

const emit = defineEmits<{
  detail: [entry: CatalogEntry]
  install: [entry: CatalogEntry]
}>()

const name = computed(() => catalogEntryName(props.entry, gettext.current))
const description = computed(() => catalogEntryDescription(props.entry, gettext.current))
const trust = computed(() => trustPreset(props.entry.trust))

const recommendedMb = computed(() => recommendedMemory(props.entry.installable_release?.manifest))
const systemMb = useSystemMemory()
const lowMemory = computed(() => isBelowRecommended(recommendedMb.value, systemMb.value))
const memoryHint = computed(() => lowMemory.value
  ? memoryWarning(recommendedMb.value, systemMb.value)
  : $gettext('Recommended memory: %{size}', { size: formatMemory(recommendedMb.value) }))

const channel = computed(() => entryChannel(props.entry))

const sourceName = useSourceName()
const isOfficialElsewhere = useOfficialElsewhere()

const isInstalled = computed(() => Boolean(props.entry.installed_version))
const canInstall = computed(() => Boolean(props.entry.installable_release))
const isActionable = computed(() => canInstall.value && (!isInstalled.value || props.entry.update_available))

// The installed copy may be a less trusted build of the same plugin.
const installed = useInstalledPlugin(() => props.entry.id)
const offer = computed(() => (props.entry.update_available ? undefined : findTrustedOffer(installed.value?.trust, props.entry)))
const { replacingId, confirmReplace } = useReplacePlugin()

const actionLabel = computed(() => props.entry.update_available ? $gettext('Update') : $gettext('Install'))
</script>

<template>
  <article
    class="plugin-card is-clickable"
    role="button"
    tabindex="0"
    @click="emit('detail', entry)"
    @keydown.enter.self="emit('detail', entry)"
  >
    <div class="plugin-card-head">
      <PluginIcon :src="entry.icon_url" :name="name" :size="40" />
      <div class="plugin-card-body">
        <ATooltip :title="entry.id" placement="topLeft">
          <span class="plugin-card-name">{{ name }}</span>
        </ATooltip>
        <div class="plugin-card-sub">
          <span v-if="entry.installable_release" class="plugin-card-version">
            v{{ entry.installable_release.version }}
          </span>
          <span v-if="entry.author" class="plugin-card-author">
            <UserOutlined />
            {{ entry.author }}
          </span>
          <ATooltip
            v-if="isOfficialElsewhere(entry)"
            :title="$gettext('Offered by %{source} instead of the official catalog.', { source: sourceName(entry.source) })"
          >
            <span class="plugin-card-source">
              <span class="i-tabler-arrows-exchange" />
              {{ sourceName(entry.source) }}
            </span>
          </ATooltip>
          <ATooltip v-if="channel !== 'stable'" :title="channelHint(channel)">
            <span class="plugin-card-channel" :class="`is-${channel}`">{{ channelLabel(channel) }}</span>
          </ATooltip>
        </div>
      </div>
      <ATooltip :title="trust.hint()">
        <span class="plugin-card-trust" :class="`is-${entry.trust || 'unsigned'}`">
          <span :class="entry.trust === 'unsigned' || !entry.trust ? 'i-tabler-shield' : 'i-tabler-shield-check'" />
          {{ trust.label() }}
        </span>
      </ATooltip>
    </div>

    <p class="plugin-card-description">
      {{ description || $gettext('No description provided.') }}
    </p>

    <div class="plugin-card-facts">
      <span
        v-for="capability in entry.capabilities ?? []"
        :key="capability"
        class="plugin-card-fact"
      >
        <span :class="capabilityIcon(capability)" />
        {{ capabilityLabel(capability) }}
      </span>
      <ATooltip v-if="recommendedMb > 0" :title="memoryHint">
        <span class="plugin-card-fact" :class="{ 'is-warning': lowMemory }" :aria-label="memoryHint">
          <span class="i-tabler-cpu" />
          {{ formatMemory(recommendedMb) }}
        </span>
      </ATooltip>
    </div>

    <div class="plugin-card-foot" @click.stop>
      <span v-if="offer" class="plugin-card-installed is-outdated">
        <WarningOutlined />
        {{ $gettext('Installed %{version} is not this version', { version: entry.installed_version! }) }}
      </span>
      <span v-else-if="isInstalled" class="plugin-card-installed" :class="{ 'is-outdated': entry.update_available }">
        <ArrowUpOutlined v-if="entry.update_available" />
        <CheckCircleOutlined v-else />
        {{ $gettext('Installed: %{version}', { version: entry.installed_version! }) }}
      </span>
      <span v-else />

      <ATooltip v-if="offer" :title="trustedOfferAction(offer)">
        <AButton
          type="primary"
          size="small"
          :loading="replacingId === entry.id"
          @click="confirmReplace(offer)"
        >
          {{ $gettext('Replace') }}
        </AButton>
      </ATooltip>
      <AButton
        v-else-if="isActionable"
        type="primary"
        size="small"
        :loading="installing"
        @click="emit('install', entry)"
      >
        <template #icon>
          <DownloadOutlined />
        </template>
        {{ actionLabel }}
      </AButton>
      <AButton v-else size="small" @click="emit('detail', entry)">
        {{ $gettext('Details') }}
      </AButton>
    </div>
  </article>
</template>

<style lang="less" scoped>
@import '../plugin-card.less';

.plugin-card-author,
.plugin-card-source {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.plugin-card-trust {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;

  &.is-official {
    color: var(--ant-color-primary);
  }

  &.is-verified {
    color: var(--ant-color-success);
  }

  &.is-community {
    color: var(--ant-orange-7, #d46b08);
  }

  &.is-unsigned {
    color: var(--ant-color-warning);
  }
}

.plugin-card-installed {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  font-size: 12px;
  color: var(--ant-color-success);

  &.is-outdated {
    color: var(--ant-color-warning);
  }
}
</style>
