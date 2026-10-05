<script setup lang="ts">
import type { MarketFact } from '@nginxui/plugin-market-ui'
import type { CatalogEntry } from '@/api/plugin_marketplace'
import { LinkOutlined } from '@antdv-next/icons'
import { MarketDetail, PluginIcon } from '@nginxui/plugin-market-ui'
import { useWindowSize } from '@vueuse/core'
import { marked } from 'marked'
import { catalogEntryName, getMarketplacePlugin } from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import { useSettingsStore } from '@/pinia'
import { channelHint, channelLabel, compareVersions, entryChannel, installableReleases, releaseChannel } from '../channel'
import { useInstalledPlugin } from '../inventory'
import { formatMemory, isBelowRecommended, memoryWarning, recommendedMemory, useSystemMemory } from '../memory'
import { permissionChanges } from '../permissionChanges'
import { useReplacePlugin } from '../replace'
import { useSourceIcon, useSourceName } from './sources'
import { findTrustedOffer, trustedOfferAction } from './trust'

const props = defineProps<{
  entry?: CatalogEntry
}>()

const emit = defineEmits<{
  install: [entry: CatalogEntry, version?: string]
}>()

const open = defineModel<boolean>('open', { default: false })

const { width: windowWidth } = useWindowSize()
// A phone gets the whole screen instead of an unusable sliver.
const drawerSize = computed(() => Math.min(720, windowWidth.value))

const loading = ref(false)
const error = ref('')
const readme = ref('')
const detail = ref<CatalogEntry>()

const current = computed(() => detail.value ?? props.entry)
const sourceName = useSourceName()
const sourceIcon = useSourceIcon()
const name = computed(() => (current.value ? catalogEntryName(current.value, gettext.current) : ''))
const recommendedMb = computed(() => recommendedMemory(current.value?.installable_release?.manifest))
const systemMb = useSystemMemory()
const lowMemory = computed(() => isBelowRecommended(recommendedMb.value, systemMb.value))
const channel = computed(() => entryChannel(current.value))
const installed = useInstalledPlugin(() => current.value?.id)
const offer = computed(() => findTrustedOffer(installed.value?.trust, current.value))
const { replacingId, confirmReplace } = useReplacePlugin()
const settings = useSettingsStore()
const renderedReadme = computed(() => (readme.value ? marked.parse(readme.value) as string : ''))

const canInstall = computed(() => Boolean(current.value?.installable_release)
  && (!current.value?.installed_version || current.value?.update_available))

// What only this node knows, after the facts the catalog gives.
const facts = computed<MarketFact[]>(() => [
  ...(current.value?.installed_version
    ? [{ key: 'installed', label: $gettext('Installed version'), value: `v${current.value.installed_version}` }]
    : []),
  ...(recommendedMb.value > 0
    ? [{ key: 'memory', label: $gettext('Recommended memory'), value: formatMemory(recommendedMb.value) }]
    : []),
])

// An update marks the permissions the installed version did not ask for.
const addedPermissions = computed(() => (current.value?.update_available && installed.value && current.value.installable_release?.manifest
  ? permissionChanges(installed.value, current.value.installable_release.manifest).added
  : []))

// Every version this node can install, each one can be picked.
const releases = computed(() => installableReleases(current.value))

// The notes worth reading before an install: every version newer than the
// installed one up to the offered release, or the offered release alone.
const releaseNotes = computed(() => {
  const entry = current.value
  const offered = entry?.installable_release
  if (!entry || !offered)
    return []
  const installedVersion = entry.installed_version
  const candidates = installedVersion && entry.update_available
    ? releases.value.filter(release => compareVersions(release.version, installedVersion) > 0
      && compareVersions(release.version, offered.version) <= 0)
    : [offered]
  return candidates
    .filter(release => release.notes)
    .map(release => ({
      version: release.version,
      date: release.released_at?.slice(0, 10),
      html: marked.parse(release.notes ?? '') as string,
    }))
})

function releaseAction(version: string) {
  const installedVersion = current.value?.installed_version
  if (!installedVersion)
    return $gettext('Install')
  return compareVersions(version, installedVersion) < 0 ? $gettext('Install this version') : $gettext('Update')
}

async function load() {
  const id = props.entry?.id
  if (!id)
    return

  loading.value = true
  error.value = ''
  readme.value = ''
  detail.value = undefined
  try {
    const response = await getMarketplacePlugin(id, props.entry?.source)
    detail.value = response.plugin
    readme.value = response.readme
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to load the plugin details'))
  }
  finally {
    loading.value = false
  }
}

watch(open, value => {
  if (value)
    load()
})
</script>

<template>
  <ADrawer
    v-model:open="open"
    :title="name"
    :size="drawerSize"
    placement="right"
  >
    <template #extra>
      <AButton
        v-if="offer && !current?.update_available"
        type="primary"
        :loading="replacingId === offer.entry.id"
        @click="confirmReplace(offer)"
      >
        {{ trustedOfferAction(offer) }}
      </AButton>
      <AButton
        v-else
        type="primary"
        :disabled="!canInstall"
        @click="current && emit('install', current)"
      >
        {{ current?.update_available ? $gettext('Update') : $gettext('Install') }}
      </AButton>
    </template>

    <ASpin :spinning="loading">
      <AAlert
        v-if="error"
        type="error"
        show-icon
        class="mb-4"
        :title="error"
      />

      <MarketDetail
        v-if="current"
        :entry="current"
        :locale="gettext.current"
        :dark="settings.theme === 'dark'"
        :facts="facts"
        :added-permissions="addedPermissions"
      >
        <template #pills>
          <ATooltip v-if="channel !== 'stable'" :title="channelHint(channel)">
            <span class="pmu-pill" :class="channel === 'beta' ? 'is-warning' : 'is-purple'">{{ channelLabel(channel) }}</span>
          </ATooltip>
        </template>

        <template v-if="lowMemory" #alerts>
          <AAlert
            type="warning"
            show-icon
            :title="memoryWarning(recommendedMb, systemMb)"
          />
        </template>

        <template #rows>
          <div class="pmu-row">
            <dt>{{ $gettext('Source') }}</dt>
            <dd class="detail-source">
              <PluginIcon
                v-if="sourceIcon(current.source)"
                :src="sourceIcon(current.source)"
                :name="sourceName(current.source)"
                :size="16"
                class="detail-source-icon"
              />
              <span :title="current.source">{{ sourceName(current.source) }}</span>
            </dd>
          </div>
        </template>

        <template #sections>
          <section v-if="releaseNotes.length" class="pmu-section">
            <div class="pmu-section-head">
              <h4 class="pmu-section-title">
                {{ current.update_available ? $gettext('Changes in this update') : $gettext('Release notes') }}
              </h4>
            </div>
            <div class="notes-list">
              <article v-for="notes in releaseNotes" :key="notes.version">
                <div class="release-main">
                  <span class="release-version">v{{ notes.version }}</span>
                  <span v-if="notes.date" class="release-date">{{ notes.date }}</span>
                </div>
                <div v-dompurify-html="notes.html" class="pmu-readme release-notes-body" />
              </article>
            </div>
          </section>

          <section class="pmu-section">
            <div class="pmu-section-head">
              <h4 class="pmu-section-title">
                {{ $gettext('Versions') }}
              </h4>
            </div>
            <ul class="release-list">
              <li
                v-for="release in releases"
                :key="release.version"
                class="release"
                :class="{ 'is-latest': release.version === current.installable_release?.version }"
              >
                <div class="release-main">
                  <span class="release-version">v{{ release.version }}</span>
                  <span v-if="release.version === current.installed_version" class="pmu-pill is-success">{{ $gettext('Installed') }}</span>
                  <span v-else-if="release.version === current.installable_release?.version" class="pmu-pill is-accent">{{ $gettext('Latest') }}</span>
                  <ATooltip v-if="releaseChannel(release) !== 'stable'" :title="channelHint(releaseChannel(release))">
                    <span class="pmu-pill" :class="releaseChannel(release) === 'beta' ? 'is-warning' : 'is-purple'">
                      {{ channelLabel(releaseChannel(release)) }}
                    </span>
                  </ATooltip>
                  <span v-if="release.released_at" class="release-date">{{ release.released_at.slice(0, 10) }}</span>
                  <a
                    v-if="release.release_notes_url"
                    :href="release.release_notes_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="pmu-pill is-link"
                  >
                    <LinkOutlined />
                    {{ $gettext('Release notes') }}
                  </a>
                </div>
                <AButton
                  v-if="release.version !== current.installed_version"
                  size="small"
                  @click="emit('install', current, release.version)"
                >
                  {{ releaseAction(release.version) }}
                </AButton>
              </li>
            </ul>
          </section>
        </template>

        <template v-if="renderedReadme" #readme>
          <div v-dompurify-html="renderedReadme" class="pmu-readme" />
        </template>
      </MarketDetail>
    </ASpin>
  </ADrawer>
</template>

<style lang="less" scoped>
@import '../plugin-detail.less';

.detail-source-icon {
  margin-right: 6px;
  vertical-align: -3px;
}

.detail-source {
  font-size: 12px;
  color: var(--ant-color-text-secondary);
  overflow-wrap: anywhere;
}

.notes-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.release-notes-body {
  margin-top: 6px;

  :deep(h1),
  :deep(h2),
  :deep(h3),
  :deep(h4) {
    margin: 10px 0 2px;
    font-size: 13px;
    font-weight: 600;
    color: var(--ant-color-text-secondary);
  }

  :deep(p) {
    margin: 4px 0;
  }

  :deep(ul),
  :deep(ol) {
    margin: 0;
    padding-left: 20px;
  }
}
</style>
