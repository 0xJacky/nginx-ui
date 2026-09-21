<script setup lang="ts">
import type { CatalogEntry } from '@/api/plugin_marketplace'
import { AppstoreOutlined, ArrowUpOutlined, CheckCircleOutlined, DownloadOutlined } from '@antdv-next/icons'
import { catalogEntryDescription, catalogEntryName } from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { trustPreset } from './trust'

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

const actionLabel = computed(() => {
  if (props.entry.update_available)
    return $gettext('Update')
  if (isInstalled.value)
    return $gettext('Installed')
  return $gettext('Install')
})
</script>

<template>
  <ACard
    class="plugin-card"
    size="small"
    hoverable
    @click="emit('detail', entry)"
  >
    <div class="flex items-start gap-3">
      <img
        v-if="entry.icon_url"
        :src="entry.icon_url"
        class="plugin-card-icon"
        alt=""
        loading="lazy"
      >
      <AppstoreOutlined v-else class="plugin-card-icon-fallback" />

      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-2">
          <span class="truncate font-medium">{{ name }}</span>
          <ATooltip :title="trust.hint()">
            <ATag :color="trust.color" class="m-0">
              {{ trust.label() }}
            </ATag>
          </ATooltip>
          <ATag v-if="entry.stage && entry.stage !== 'production'" color="purple" class="m-0">
            {{ entry.stage }}
          </ATag>
        </div>
        <div class="truncate font-mono text-xs text-gray-400">
          {{ entry.id }}
        </div>
      </div>
    </div>

    <p class="plugin-card-description">
      {{ description || $gettext('No description provided.') }}
    </p>

    <div class="mb-3 flex flex-wrap gap-1">
      <ATag v-for="capability in entry.capabilities ?? []" :key="capability" class="m-0">
        {{ capability }}
      </ATag>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="min-w-0 text-xs text-gray-500">
        <span v-if="entry.author">{{ entry.author }}</span>
        <span v-if="entry.author && entry.installable_release"> · </span>
        <span v-if="entry.installable_release">v{{ entry.installable_release.version }}</span>
        <div v-if="isInstalled" class="flex items-center gap-1">
          <CheckCircleOutlined v-if="!entry.update_available" class="text-green-500" />
          <ArrowUpOutlined v-else class="text-orange-500" />
          <span>{{ $gettext('Installed: %{version}', { version: entry.installed_version! }) }}</span>
        </div>
      </div>

      <AButton
        :type="entry.update_available || !isInstalled ? 'primary' : 'default'"
        size="small"
        :disabled="!canInstall || (isInstalled && !entry.update_available)"
        :loading="installing"
        @click.stop="emit('install', entry)"
      >
        <template #icon>
          <DownloadOutlined />
        </template>
        {{ actionLabel }}
      </AButton>
    </div>
  </ACard>
</template>

<style lang="less" scoped>
.plugin-card {
  height: 100%;
  cursor: pointer;
}

.plugin-card-icon,
.plugin-card-icon-fallback {
  width: 36px;
  height: 36px;
  flex: none;
  object-fit: contain;
  border-radius: 8px;
}

.plugin-card-icon-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  color: var(--ant-color-text-quaternary);
  background-color: var(--ant-color-fill-tertiary);
}

.plugin-card-description {
  margin: 12px 0;
  color: var(--ant-color-text-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 44px;
}
</style>
