<script setup lang="ts">
import type { CatalogEntry } from '@/api/plugin_marketplace'
import { ArrowUpOutlined, CheckCircleOutlined, DownloadOutlined } from '@antdv-next/icons'
import { catalogEntryDescription, catalogEntryName } from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { capabilityLabel } from '../capabilities'
import { useInstalledPlugin } from '../inventory'
import PluginIcon from '../PluginIcon.vue'
import { useReplacePlugin } from '../replace'
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

const isInstalled = computed(() => Boolean(props.entry.installed_version))
const canInstall = computed(() => Boolean(props.entry.installable_release))
const isActionable = computed(() => canInstall.value && (!isInstalled.value || props.entry.update_available))

// The installed copy may be a less trusted build of the same plugin.
const installed = useInstalledPlugin(() => props.entry.id)
const offer = computed(() => (props.entry.update_available ? undefined : findTrustedOffer(installed.value?.trust, props.entry)))
const { replacingId, confirmReplace } = useReplacePlugin()

const actionLabel = computed(() => {
  if (props.entry.update_available)
    return $gettext('Update')
  if (isInstalled.value)
    return $gettext('Installed')
  return $gettext('Install')
})
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
        <div class="plugin-card-title">
          <span class="plugin-card-name">{{ name }}</span>
          <span v-if="entry.installable_release" class="plugin-card-version">
            v{{ entry.installable_release.version }}
          </span>
        </div>
        <div class="plugin-card-id">
          {{ entry.id }}
        </div>
      </div>
      <ATooltip :title="trust.hint()">
        <ATag :color="trust.color" class="m-0 flex-none" variant="filled">
          {{ trust.label() }}
        </ATag>
      </ATooltip>
    </div>

    <p class="plugin-card-description">
      {{ description || $gettext('No description provided.') }}
    </p>

    <div class="plugin-card-meta">
      <ATag
        v-for="capability in entry.capabilities ?? []"
        :key="capability"
        class="m-0"
        variant="filled"
      >
        {{ capabilityLabel(capability) }}
      </ATag>
      <ATag
        v-if="entry.stage && entry.stage !== 'production'"
        color="purple"
        class="m-0"
      >
        {{ entry.stage }}
      </ATag>
    </div>

    <div class="plugin-card-foot" @click.stop>
      <div class="plugin-card-status">
        <span v-if="entry.author" class="truncate">{{ entry.author }}</span>
        <span v-if="offer" class="plugin-card-installed is-outdated">
          {{ $gettext('Installed %{version} is not this version', { version: entry.installed_version! }) }}
        </span>
        <span v-else-if="isInstalled" class="plugin-card-installed" :class="{ 'is-outdated': entry.update_available }">
          <ArrowUpOutlined v-if="entry.update_available" />
          <CheckCircleOutlined v-else />
          {{ $gettext('Installed: %{version}', { version: entry.installed_version! }) }}
        </span>
      </div>

      <AButton
        v-if="offer"
        type="primary"
        size="small"
        :loading="replacingId === entry.id"
        @click="confirmReplace(offer)"
      >
        {{ trustedOfferAction(offer) }}
      </AButton>
      <AButton
        v-else
        :type="isActionable ? 'primary' : 'default'"
        size="small"
        :disabled="!isActionable"
        :loading="installing"
        @click="emit('install', entry)"
      >
        <template #icon>
          <DownloadOutlined />
        </template>
        {{ actionLabel }}
      </AButton>
    </div>
  </article>
</template>

<style lang="less" scoped>
@import '../plugin-card.less';

.plugin-card-status {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  font-size: 12px;
  color: var(--ant-color-text-secondary);
}

.plugin-card-installed {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--ant-color-success);

  &.is-outdated {
    color: var(--ant-color-warning);
  }
}
</style>
